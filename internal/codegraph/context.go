package codegraph

import (
	"os"
	"path/filepath"
	"strings"
)

type DefDetail struct {
	DefLoc
	Callees []string  `json:"callees,omitempty"`
	Callers []CallLoc `json:"callers,omitempty"`
	Source  string    `json:"source,omitempty"`
}

type Context struct {
	Target      string      `json:"target"`
	Kind        string      `json:"kind"`
	Definitions []DefDetail `json:"definitions,omitempty"`
	Defs        []DefLoc    `json:"defs,omitempty"`
	Imports     []string    `json:"imports,omitempty"`
	ImportedBy  []string    `json:"imported_by,omitempty"`
	Diagnostics []string    `json:"diagnostics,omitempty"`
}

const maxDiagnostics = 20

const maxSourceLines = 60

// GetContext resolves a target into a cross-file slice. Target forms:
//
//	"path/to/file.js"          -> file context (imports, importers, defs)
//	"path/to/file.js:symbol"   -> symbol context scoped to that file
//	"symbol"                   -> symbol context across the whole project
func (g *Graph) GetContext(target string) Context {
	var c Context
	if file, sym, ok := g.splitFileSymbol(target); ok {
		c = g.symbolContext(target, sym, file)
	} else if g.Files[target] != nil {
		c = g.fileContext(target)
	} else {
		c = g.symbolContext(target, target, "")
	}
	diags := g.Diagnostics
	if len(diags) > maxDiagnostics {
		diags = diags[:maxDiagnostics]
	}
	c.Diagnostics = diags
	return c
}

func (g *Graph) splitFileSymbol(target string) (file, sym string, ok bool) {
	i := strings.LastIndex(target, ":")
	if i < 0 {
		return "", "", false
	}
	file, sym = target[:i], target[i+1:]
	if _, exists := g.Files[file]; exists && sym != "" {
		return file, sym, true
	}
	return "", "", false
}

func (g *Graph) symbolContext(target, sym, scopeFile string) Context {
	c := Context{Target: target, Kind: "symbol"}
	for _, d := range g.Defs(sym) {
		if scopeFile != "" && d.File != scopeFile {
			continue
		}
		c.Definitions = append(c.Definitions, DefDetail{
			DefLoc:  d,
			Callees: g.calleesIn(d),
			Callers: g.callersOf(sym, d),
			Source:  g.readSource(d.File, d.Line, d.EndLine),
		})
		c.Imports = appendAll(c.Imports, g.Imports(d.File))
		c.ImportedBy = appendAll(c.ImportedBy, g.Importers(d.File))
	}
	return c
}

// callersOf attributes call sites of sym to a specific definition d.
//
// Resolution is hybrid:
//   - same file: a call outside d's own body is a caller (Resolved).
//   - cross-file with an import binding: attribute only if the binding's target
//     file is, or can reach, d's file — pinning the call to the right module and
//     skipping same-named defs elsewhere (Resolved).
//   - cross-file without a binding (dynamic receiver like res.x(), or a global):
//     fall back to the looser reachability heuristic (not Resolved).
func (g *Graph) callersOf(sym string, d DefLoc) []CallLoc {
	var callers []CallLoc
	for _, call := range g.Calls(sym) {
		if call.File == d.File {
			if call.Line >= d.Line && call.Line <= d.EndLine {
				continue // inside the definition itself
			}
			call.Resolved = true
			callers = append(callers, call)
			continue
		}
		if target, ok := g.bindingTarget(call.File, call.Recv, sym); ok {
			if target == d.File || g.reachableFiles(target)[d.File] {
				call.Resolved = true
				callers = append(callers, call)
			}
			continue // binding points to a definite module; don't heuristically attach
		}
		if g.reachableFiles(call.File)[d.File] {
			call.Resolved = false
			callers = append(callers, call)
		}
	}
	return callers
}

func (g *Graph) fileContext(file string) Context {
	c := Context{Target: file, Kind: "file"}
	c.Imports = g.Imports(file)
	c.ImportedBy = g.Importers(file)
	if fp := g.Files[file]; fp != nil {
		for _, d := range fp.Defs {
			c.Defs = append(c.Defs, DefLoc{
				Name: d.Name, File: file, Line: d.Line, EndLine: d.EndLine, Kind: d.Kind, Exported: d.Exported,
			})
		}
	}
	return c
}

// calleesIn returns the distinct callees within a definition's line range,
// qualified by receiver when present (e.g. "db.query", not just "query").
func (g *Graph) calleesIn(d DefLoc) []string {
	fp := g.Files[d.File]
	if fp == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, call := range fp.Calls {
		if call.Line < d.Line || call.Line > d.EndLine {
			continue
		}
		name := call.Name
		if call.Recv != "" {
			name = call.Recv + "." + call.Name
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}

func (g *Graph) readSource(file string, start, end int) string {
	if end-start+1 > maxSourceLines {
		end = start + maxSourceLines - 1
	}
	data, err := os.ReadFile(filepath.Join(g.Root, file))
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	if start < 1 {
		start = 1
	}
	if end > len(lines) {
		end = len(lines)
	}
	return strings.Join(lines[start-1:end], "\n")
}

func appendAll(dst, src []string) []string {
	for _, v := range src {
		dst = appendUnique(dst, v)
	}
	return dst
}
