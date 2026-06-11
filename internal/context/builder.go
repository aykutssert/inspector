package context

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Graph is the language-neutral structural graph consumed by RepoMap.
type Graph struct {
	Root        string
	Files       map[string]*FileParse
	Diagnostics []string
	importers   map[string][]string
}

func BuildGraph(root string, files []string, parser FileParser) *Graph {
	g := &Graph{
		Root:      root,
		Files:     map[string]*FileParse{},
		importers: map[string][]string{},
	}
	for _, path := range files {
		fp, err := parser.Parse(root, path)
		if err != nil {
			g.Diagnostics = append(g.Diagnostics, "parse failed: "+path+": "+err.Error())
			continue
		}
		if fp == nil {
			continue
		}
		fp.Path = path
		g.Files[path] = fp
		if fp.HasError {
			g.Diagnostics = append(g.Diagnostics, "syntax errors (partial AST): "+path)
		}
	}
	for from, fp := range g.Files {
		for _, imp := range fp.Imports {
			if imp.Target == "" || g.Files[imp.Target] == nil {
				continue
			}
			g.importers[imp.Target] = appendUniqueString(g.importers[imp.Target], from)
		}
	}
	sort.Strings(g.Diagnostics)
	return g
}

func BuildRepoMap(root string, files []string, parser FileParser) (RepoMap, error) {
	if parser == nil {
		return RepoMap{}, errors.New("no file parsers registered")
	}
	graph := BuildGraph(root, files, parser)
	if len(graph.Files) == 0 {
		return RepoMap{}, errors.New("no supported source files found")
	}

	nodes := graph.fileNodes()
	languages := graph.languages()
	repo := RepoMap{
		Language:    primaryLanguage(languages),
		Languages:   languages,
		EntryPoints: graph.GenericEntryPoints(),
		HotFiles:    topFiles(nodes, 10),
		Dirs:        groupDirs(nodes),
	}
	if enricher, ok := parser.(MapEnricher); ok {
		enricher.EnrichMap(root, graph, &repo)
	}
	sort.Strings(repo.EntryPoints)
	repo.EntryPoints = uniqueStrings(repo.EntryPoints)
	return repo, nil
}

func (g *Graph) Importers(path string) []string {
	return append([]string(nil), g.importers[path]...)
}

func (g *Graph) GenericEntryPoints() []string {
	var out []string
	for path, fp := range g.Files {
		base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		switch strings.ToLower(base) {
		case "index", "main", "server", "app", "cli", "start", "run", "init":
			out = append(out, path)
			continue
		}
		if len(g.importers[path]) == 0 && hasExportedDef(fp) {
			out = append(out, path)
		}
	}
	sort.Strings(out)
	return uniqueStrings(out)
}

func (g *Graph) languages() []string {
	seen := map[string]bool{}
	for _, fp := range g.Files {
		if fp.Language != "" {
			seen[fp.Language] = true
		}
	}
	out := make([]string, 0, len(seen))
	for language := range seen {
		out = append(out, language)
	}
	sort.Strings(out)
	return out
}

func primaryLanguage(languages []string) string {
	if len(languages) == 0 {
		return ""
	}
	if len(languages) == 1 {
		return languages[0]
	}
	jsFamily := true
	hasTypeScript := false
	for _, language := range languages {
		if language != "javascript" && language != "typescript" {
			jsFamily = false
		}
		if language == "typescript" {
			hasTypeScript = true
		}
	}
	if jsFamily && hasTypeScript {
		return "typescript"
	}
	return "mixed"
}

func (g *Graph) fileNodes() []FileNode {
	nodes := make([]FileNode, 0, len(g.Files))
	for path, fp := range g.Files {
		node := FileNode{
			Path:       path,
			ImportedBy: len(g.importers[path]),
		}
		for _, imp := range fp.Imports {
			if imp.Target == "" && imp.Package != "" {
				node.Deps = appendUniqueString(node.Deps, imp.Package)
			}
		}
		sort.Strings(node.Deps)
		lines := readLines(filepath.Join(g.Root, path))
		for _, def := range fp.Defs {
			if !def.Exported {
				continue
			}
			node.Exports = append(node.Exports, Export{
				Name: def.Name,
				Kind: def.Kind,
				Sig:  extractSignature(lines, def.Line),
			})
		}
		sort.Slice(node.Exports, func(i, j int) bool {
			if exportRank(node.Exports[i].Kind) != exportRank(node.Exports[j].Kind) {
				return exportRank(node.Exports[i].Kind) < exportRank(node.Exports[j].Kind)
			}
			return node.Exports[i].Name < node.Exports[j].Name
		})
		nodes = append(nodes, node)
	}
	return nodes
}

func hasExportedDef(fp *FileParse) bool {
	for _, def := range fp.Defs {
		if def.Exported {
			return true
		}
	}
	return false
}

func readLines(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return strings.Split(string(data), "\n")
}

func extractSignature(lines []string, line int) string {
	if line < 1 || line > len(lines) {
		return ""
	}
	var parts []string
	depth := 0
	for i := line - 1; i < len(lines) && i < line+7; i++ {
		current := strings.TrimRight(lines[i], " \t\r")
		parts = append(parts, strings.TrimSpace(current))
		depth += strings.Count(current, "(") - strings.Count(current, ")")
		if depth <= 0 {
			break
		}
	}
	signature := strings.TrimSpace(strings.Join(parts, " "))
	signature = strings.TrimSpace(strings.TrimSuffix(signature, "{"))
	if len(signature) > 160 {
		signature = signature[:160] + "..."
	}
	return signature
}

func exportRank(kind string) int {
	switch kind {
	case "function":
		return 0
	case "class":
		return 1
	case "method":
		return 2
	default:
		return 3
	}
}

func topFiles(nodes []FileNode, limit int) []FileNode {
	var out []FileNode
	for _, node := range nodes {
		if node.ImportedBy > 0 {
			out = append(out, node)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ImportedBy != out[j].ImportedBy {
			return out[i].ImportedBy > out[j].ImportedBy
		}
		return out[i].Path < out[j].Path
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func groupDirs(nodes []FileNode) []DirNode {
	byDir := map[string][]FileNode{}
	for _, node := range nodes {
		dir := filepath.Dir(node.Path)
		if dir == "." {
			dir = "."
		}
		byDir[dir] = append(byDir[dir], node)
	}
	out := make([]DirNode, 0, len(byDir))
	for path, files := range byDir {
		sort.Slice(files, func(i, j int) bool {
			if files[i].ImportedBy != files[j].ImportedBy {
				return files[i].ImportedBy > files[j].ImportedBy
			}
			return files[i].Path < files[j].Path
		})
		out = append(out, DirNode{Path: path, FileCount: len(files), Files: files})
	}
	sort.Slice(out, func(i, j int) bool {
		left, right := maxImports(out[i].Files), maxImports(out[j].Files)
		if left != right {
			return left > right
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func maxImports(files []FileNode) int {
	max := 0
	for _, file := range files {
		if file.ImportedBy > max {
			max = file.ImportedBy
		}
	}
	return max
}

func appendUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func uniqueStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	out := values[:0]
	for _, value := range values {
		if len(out) == 0 || out[len(out)-1] != value {
			out = append(out, value)
		}
	}
	return out
}
