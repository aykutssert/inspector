package jscontext

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	ctxpkg "github.com/aykutssert/scout/internal/context"
)

// GetMap implements ctxpkg.MapProvider.
func (Provider) GetMap(root string, files []string) (ctxpkg.RepoMap, error) {
	return Build(root, files).BuildMap(), nil
}

// BuildMap constructs a RepoMap from the parsed graph.
func (g *Graph) BuildMap() ctxpkg.RepoMap {
	m := ctxpkg.RepoMap{
		Language: g.detectLanguage(),
	}
	m.Framework, m.FrameworkVer = g.detectFramework()
	m.EntryPoints = g.detectEntryPoints(m.Framework)
	// Read tsconfig path aliases once; shared across all externalDeps calls.
	g.tsAliases = tsconfigAliases(g.Root)
	nodes := g.buildFileNodes()
	m.HotFiles = topByImportedBy(nodes, 10)
	m.Dirs = groupIntoDirs(nodes)
	return m
}

// ─── language / framework ─────────────────────────────────────────────────────

func (g *Graph) detectLanguage() string {
	for rel := range g.Files {
		ext := strings.ToLower(filepath.Ext(rel))
		if ext == ".ts" || ext == ".tsx" || ext == ".mts" || ext == ".cts" {
			return "typescript"
		}
	}
	return "javascript"
}

// pkgJSON is the subset of package.json fields we care about.
type pkgJSON struct {
	Main    string            `json:"main"`
	Exports map[string]string `json:"exports"`
	Deps    map[string]string `json:"dependencies"`
	DevDeps map[string]string `json:"devDependencies"`
	PeerDeps map[string]string `json:"peerDependencies"`
}

func (p *pkgJSON) hasDep(name string) (string, bool) {
	for _, m := range []map[string]string{p.Deps, p.DevDeps, p.PeerDeps} {
		if v, ok := m[name]; ok {
			return v, true
		}
	}
	return "", false
}

func readPkgJSON(root string) *pkgJSON {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err != nil {
		return nil
	}
	var p pkgJSON
	if json.Unmarshal(data, &p) != nil {
		return nil
	}
	return &p
}

// frameworkPriority lists known frameworks in detection order. The first match
// wins, so more specific frameworks (react-native, nestjs) come before generic
// ones (react, express).
var frameworkPriority = []struct{ dep, name string }{
	{"react-native", "react-native"},
	{"next", "nextjs"},
	{"@nestjs/core", "nestjs"},
	{"nuxt", "nuxt"},
	{"@sveltejs/kit", "sveltekit"},
	{"svelte", "svelte"},
	{"fastify", "fastify"},
	{"express", "express"},
	{"vite", "vite"},
	{"react", "react"},
}

func (g *Graph) detectFramework() (name, ver string) {
	pkg := readPkgJSON(g.Root)
	if pkg == nil {
		return "", ""
	}
	for _, f := range frameworkPriority {
		if v, ok := pkg.hasDep(f.dep); ok {
			return f.name, cleanVersion(v)
		}
	}
	return "", ""
}

// cleanVersion strips semver range prefixes (^, ~, >=, etc.).
func cleanVersion(v string) string {
	return strings.TrimLeft(v, "^~>=<v ")
}

// ─── entry points ─────────────────────────────────────────────────────────────

func (g *Graph) detectEntryPoints(framework string) []string {
	var eps []string
	switch framework {
	case "nextjs":
		eps = g.nextjsEntryPoints()
	case "react-native":
		eps = g.genericEntryPoints()
	default:
		eps = g.genericEntryPoints()
	}
	// Deduplicate and sort for stable output.
	seen := map[string]bool{}
	var out []string
	for _, e := range eps {
		if !seen[e] {
			seen[e] = true
			out = append(out, e)
		}
	}
	sort.Strings(out)
	return out
}

// nextjsEntryPoints uses the Next.js file-system router conventions.
// These are framework contracts, not folder naming guesses.
func (g *Graph) nextjsEntryPoints() []string {
	var eps []string
	for rel := range g.Files {
		base := filepath.Base(rel)
		baseNoExt := strings.TrimSuffix(base, filepath.Ext(base))
		switch baseNoExt {
		case "middleware":
			eps = append(eps, rel)
		case "page", "layout", "loading", "error", "not-found", "template":
			eps = append(eps, rel)
		case "route":
			// app router API route
			eps = append(eps, rel)
		}
	}
	// Fall back to generic if no Next.js-specific entries found.
	if len(eps) == 0 {
		return g.genericEntryPoints()
	}
	return eps
}

// genericEntryPoints finds structurally obvious entry files:
//   - well-known root-level names (index, main, server, app, cli)
//   - files with 0 importers that have at least one export
var entryBaseNames = map[string]bool{
	"index": true, "main": true, "server": true, "app": true,
	"cli": true, "start": true, "run": true, "init": true,
}

func (g *Graph) genericEntryPoints() []string {
	var eps []string
	for rel, fp := range g.Files {
		base := filepath.Base(rel)
		baseNoExt := strings.ToLower(strings.TrimSuffix(base, filepath.Ext(base)))
		if entryBaseNames[baseNoExt] {
			eps = append(eps, rel)
			continue
		}
		// No importers + has exports = likely an entry point.
		if len(g.importers[rel]) == 0 && hasExport(fp) {
			eps = append(eps, rel)
		}
	}
	return eps
}

func hasExport(fp *FileParse) bool {
	for _, d := range fp.Defs {
		if d.Exported {
			return true
		}
	}
	return false
}

// ─── file nodes ───────────────────────────────────────────────────────────────

func (g *Graph) buildFileNodes() []ctxpkg.FileNode {
	// Cache file source lines keyed by relative path.
	srcCache := map[string][]string{}
	readLines := func(rel string) []string {
		if lines, ok := srcCache[rel]; ok {
			return lines
		}
		data, err := os.ReadFile(filepath.Join(g.Root, rel))
		if err != nil {
			srcCache[rel] = nil
			return nil
		}
		lines := strings.Split(string(data), "\n")
		srcCache[rel] = lines
		return lines
	}

	nodes := make([]ctxpkg.FileNode, 0, len(g.Files))
	for rel, fp := range g.Files {
		node := ctxpkg.FileNode{
			Path:       rel,
			ImportedBy: len(g.importers[rel]),
			Deps:       g.externalDeps(rel),
		}
		lines := readLines(rel)
		for _, d := range fp.Defs {
			if !d.Exported {
				continue
			}
			exp := ctxpkg.Export{
				Name: d.Name,
				Kind: d.Kind,
				Sig:  extractSig(lines, d.Line),
			}
			node.Exports = append(node.Exports, exp)
		}
		// Sort exports: functions/classes first, then by name.
		sort.Slice(node.Exports, func(i, j int) bool {
			ki, kj := exportKindRank(node.Exports[i].Kind), exportKindRank(node.Exports[j].Kind)
			if ki != kj {
				return ki < kj
			}
			return node.Exports[i].Name < node.Exports[j].Name
		})
		nodes = append(nodes, node)
	}
	return nodes
}

func exportKindRank(kind string) int {
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

// extractSig builds a clean function/type signature starting at 1-based `line`.
// It reads ahead until parentheses are balanced and a body opener is found,
// handling both same-line `{` (K&R) and next-line `{` (Allman) styles.
// Result is a single joined string with the body opener stripped.
func extractSig(lines []string, line int) string {
	if line < 1 || line > len(lines) {
		return ""
	}
	var parts []string
	depth := 0 // unmatched open parens
	for i := line - 1; i < len(lines) && i < line+7; i++ {
		l := strings.TrimRight(lines[i], " \t\r")
		if i > line-1 {
			// Join continuation lines with a single space, trimming leading indent.
			parts = append(parts, strings.TrimLeft(l, " \t"))
		} else {
			parts = append(parts, strings.TrimSpace(l))
		}
		for _, ch := range l {
			switch ch {
			case '(':
				depth++
			case ')':
				depth--
			}
		}
		trimmed := strings.TrimRight(l, " \t")
		// Stop once parens are balanced — signature params are complete.
		if depth <= 0 {
			// Body opener on same line.
			if strings.HasSuffix(trimmed, "{") || strings.HasSuffix(trimmed, "=>") {
				break
			}
			// Type annotation without body (e.g. interface member, type alias).
			// Stop here; the next line would be the body or closing brace.
			break
		}
	}
	sig := strings.Join(parts, " ")
	// Strip trailing body opener.
	if strings.HasSuffix(sig, "{") {
		sig = strings.TrimSpace(sig[:len(sig)-1])
	}
	sig = strings.TrimSpace(sig)
	if len(sig) > 160 {
		sig = sig[:160] + "…"
	}
	return sig
}

// tsconfigAliases reads compilerOptions.paths from tsconfig.json at root and
// returns the set of alias prefixes (e.g. "@/", "~/" "components/").
// Returns nil if no tsconfig exists or it has no paths.
func tsconfigAliases(root string) map[string]bool {
	data, err := os.ReadFile(filepath.Join(root, "tsconfig.json"))
	if err != nil {
		return nil
	}
	var ts struct {
		CompilerOptions struct {
			Paths map[string]json.RawMessage `json:"paths"`
		} `json:"compilerOptions"`
	}
	if json.Unmarshal(data, &ts) != nil {
		return nil
	}
	if len(ts.CompilerOptions.Paths) == 0 {
		return nil
	}
	aliases := make(map[string]bool, len(ts.CompilerOptions.Paths))
	for pattern := range ts.CompilerOptions.Paths {
		// "@/components/*" → prefix "@/"
		// "components/*"   → prefix "components/"
		// "@app"           → exact match "@app"
		prefix := strings.TrimSuffix(pattern, "*")
		prefix = strings.TrimSuffix(prefix, "/")
		if prefix != "" {
			aliases[prefix] = true
		}
	}
	return aliases
}

// externalDeps returns the distinct external package names imported by file.
// Relative imports, common path-alias prefixes (@/, ~/, #), and tsconfig.json
// path aliases are excluded.
func (g *Graph) externalDeps(file string) []string {
	fp := g.Files[file]
	if fp == nil {
		return nil
	}
	var deps []string
	for _, imp := range fp.Imports {
		src := imp.Source
		if strings.HasPrefix(src, ".") {
			continue // relative — internal
		}
		// Common bundler/tsconfig alias prefixes that are never npm packages.
		if strings.HasPrefix(src, "@/") || strings.HasPrefix(src, "~/") ||
			strings.HasPrefix(src, "#") || strings.HasPrefix(src, "$/") {
			continue
		}
		// tsconfig.json path aliases (e.g. "@app/*", "components/*").
		if g.isTSAlias(src) {
			continue
		}
		if g.resolveImport(file, src) != "" {
			continue // path alias resolved to a known file — internal
		}
		pkg := externalPkgName(src)
		deps = appendUnique(deps, pkg)
	}
	sort.Strings(deps)
	return deps
}

// isTSAlias reports whether spec starts with a known tsconfig path alias prefix.
func (g *Graph) isTSAlias(spec string) bool {
	for prefix := range g.tsAliases {
		if strings.HasPrefix(spec, prefix+"/") || spec == prefix {
			return true
		}
	}
	return false
}

// externalPkgName extracts the canonical package name from an import specifier:
//
//	node:fs            → fs
//	@scope/name/deep   → @scope/name
//	package/subpath    → package
func externalPkgName(spec string) string {
	spec = strings.TrimPrefix(spec, "node:")
	if strings.HasPrefix(spec, "@") {
		parts := strings.SplitN(spec, "/", 3)
		if len(parts) >= 2 {
			return parts[0] + "/" + parts[1]
		}
		return spec
	}
	return strings.SplitN(spec, "/", 2)[0]
}

// ─── grouping / ranking ───────────────────────────────────────────────────────

// topByImportedBy returns the n files with the highest ImportedBy counts,
// sorted descending. Files with 0 importers are excluded (entry points, not hot).
func topByImportedBy(nodes []ctxpkg.FileNode, n int) []ctxpkg.FileNode {
	var candidates []ctxpkg.FileNode
	for _, nd := range nodes {
		if nd.ImportedBy > 0 {
			candidates = append(candidates, nd)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].ImportedBy != candidates[j].ImportedBy {
			return candidates[i].ImportedBy > candidates[j].ImportedBy
		}
		return candidates[i].Path < candidates[j].Path
	})
	if len(candidates) > n {
		candidates = candidates[:n]
	}
	return candidates
}

// groupIntoDirs groups FileNodes by their parent directory and sorts dirs by
// average ImportedBy descending (most important dirs first).
func groupIntoDirs(nodes []ctxpkg.FileNode) []ctxpkg.DirNode {
	dirMap := map[string][]ctxpkg.FileNode{}
	for _, nd := range nodes {
		dir := filepath.Dir(nd.Path)
		if dir == "." {
			dir = ""
		}
		dirMap[dir] = append(dirMap[dir], nd)
	}

	dirs := make([]ctxpkg.DirNode, 0, len(dirMap))
	for dir, files := range dirMap {
		// Sort files within dir by imported_by desc, then path.
		sort.Slice(files, func(i, j int) bool {
			if files[i].ImportedBy != files[j].ImportedBy {
				return files[i].ImportedBy > files[j].ImportedBy
			}
			return files[i].Path < files[j].Path
		})
		name := dir
		if name == "" {
			name = "."
		}
		dirs = append(dirs, ctxpkg.DirNode{
			Path:      name,
			FileCount: len(files),
			Files:     files,
		})
	}

	// Sort dirs by max ImportedBy in the dir (most important dir first).
	sort.Slice(dirs, func(i, j int) bool {
		mi := maxImportedBy(dirs[i].Files)
		mj := maxImportedBy(dirs[j].Files)
		if mi != mj {
			return mi > mj
		}
		return dirs[i].Path < dirs[j].Path
	})
	return dirs
}

func maxImportedBy(files []ctxpkg.FileNode) int {
	max := 0
	for _, f := range files {
		if f.ImportedBy > max {
			max = f.ImportedBy
		}
	}
	return max
}
