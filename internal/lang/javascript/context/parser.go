package jscontext

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aykutssert/scout/internal/context"
)

// JSParser implements context.FileParser for JavaScript/TypeScript.
var _ context.FileParser = (*JSParser)(nil)

type JSParser struct{}

func (JSParser) Parse(root, path string) (*context.FileParse, error) {
	if !isJavaScriptPath(path) {
		return nil, nil
	}
	fp, err := ParseJS(filepath.Join(root, path))
	if err != nil {
		return nil, err
	}
	aliases := tsconfigAliases(root)
	imports := make([]context.Import, len(fp.Imports))
	for i, im := range fp.Imports {
		imports[i] = context.Import{
			Source:  im.Source,
			Target:  resolveJSImport(root, path, im.Source),
			Package: jsPackage(aliases, im.Source),
			Line:    im.Line,
		}
	}
	defs := make([]context.Def, len(fp.Defs))
	for i, d := range fp.Defs {
		defs[i] = context.Def{Name: d.Name, Kind: d.Kind, Line: d.Line, EndLine: d.EndLine, Exported: d.Exported}
	}
	calls := make([]context.Call, len(fp.Calls))
	for i, c := range fp.Calls {
		calls[i] = context.Call{Name: c.Name, Recv: c.Recv, Line: c.Line}
	}
	return &context.FileParse{
		Path:     path,
		Language: jsLanguage(path),
		Imports:  imports,
		Defs:     defs,
		Calls:    calls,
		HasError: fp.HasError,
	}, nil
}

func (JSParser) EnrichMap(root string, graph *context.Graph, repo *context.RepoMap) {
	repo.Framework, repo.FrameworkVer = detectJSFramework(root)
	entries, roles := jsFrameworkEntries(root, repo.Framework, graph)
	if len(entries) > 0 {
		repo.EntryPoints = entries
	}
	applyRoles(repo, roles)
}

func isJavaScriptPath(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx", ".mts", ".cts":
		return true
	default:
		return false
	}
}

func jsLanguage(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ts", ".tsx", ".mts", ".cts":
		return "typescript"
	default:
		return "javascript"
	}
}

func resolveJSImport(root, from, source string) string {
	if !strings.HasPrefix(source, ".") {
		return ""
	}
	base := filepath.Join(filepath.Dir(from), source)
	for _, ext := range resolveExts {
		candidate := filepath.Clean(base + ext)
		if info, err := os.Stat(filepath.Join(root, candidate)); err == nil && !info.IsDir() {
			return candidate
		}
	}
	for _, ext := range resolveExts[1:] {
		candidate := filepath.Clean(filepath.Join(base, "index"+ext))
		if info, err := os.Stat(filepath.Join(root, candidate)); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func jsPackage(aliases map[string]bool, source string) string {
	if source == "" || strings.HasPrefix(source, ".") ||
		strings.HasPrefix(source, "@/") || strings.HasPrefix(source, "~/") ||
		strings.HasPrefix(source, "#") || strings.HasPrefix(source, "$/") {
		return ""
	}
	for prefix := range aliases {
		if source == prefix || strings.HasPrefix(source, prefix+"/") {
			return ""
		}
	}
	return externalPkgName(source)
}

func detectJSFramework(root string) (string, string) {
	pkg := readPkgJSON(root)
	if pkg == nil {
		return "", ""
	}
	for _, framework := range frameworkPriority {
		if version, ok := pkg.hasDep(framework.dep); ok {
			return framework.name, cleanVersion(version)
		}
	}
	return "", ""
}

func jsFrameworkEntries(root, framework string, graph *context.Graph) ([]string, map[string]string) {
	roles := map[string]string{}
	var entries []string
	switch framework {
	case "nextjs":
		for path := range graph.Files {
			base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			switch base {
			case "page":
				entries = append(entries, path)
				roles[path] = "page"
			case "layout":
				entries = append(entries, path)
				roles[path] = "layout"
			case "route":
				entries = append(entries, path)
				roles[path] = "api-route"
			case "middleware":
				entries = append(entries, path)
				roles[path] = "middleware"
			case "loading", "error", "not-found", "template":
				entries = append(entries, path)
				roles[path] = base
			}
		}
	case "express":
		for path, file := range graph.Files {
			importsExpress := false
			for _, imp := range file.Imports {
				if imp.Package == "express" {
					importsExpress = true
					break
				}
			}
			for _, call := range file.Calls {
				if (call.Name == "express" && call.Recv == "") ||
					(importsExpress && call.Name == "listen" && call.Recv != "") {
					entries = append(entries, path)
					roles[path] = "server"
					break
				}
			}
		}
	case "nestjs":
		for path, file := range graph.Files {
			for _, call := range file.Calls {
				if call.Recv == "NestFactory" && call.Name == "create" {
					entries = append(entries, path)
					roles[path] = "bootstrap"
					break
				}
			}
		}
	case "vite":
		if info, err := os.Stat(filepath.Join(root, "index.html")); err == nil && !info.IsDir() {
			entries = append(entries, "index.html")
		}
		for path := range graph.Files {
			base := strings.ToLower(filepath.Base(path))
			dir := filepath.ToSlash(filepath.Dir(path))
			if dir == "src" && strings.HasPrefix(base, "main.") {
				entries = append(entries, path)
				roles[path] = "app-entry"
			}
		}
	}
	sort.Strings(entries)
	return entries, roles
}

func applyRoles(repo *context.RepoMap, roles map[string]string) {
	if len(roles) == 0 {
		return
	}
	for i := range repo.Dirs {
		for j := range repo.Dirs[i].Files {
			repo.Dirs[i].Files[j].Role = roles[repo.Dirs[i].Files[j].Path]
		}
	}
	for i := range repo.HotFiles {
		repo.HotFiles[i].Role = roles[repo.HotFiles[i].Path]
	}
}
