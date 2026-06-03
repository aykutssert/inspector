package javascript

import (
	"os"
	"path/filepath"
	"strings"

	inspectctx "github.com/aykutssert/inspector/internal/context"
)

type Provider struct{}

func NewProvider() Provider { return Provider{} }

func (Provider) Name() string { return "javascript" }

func (Provider) GetContext(root string, files []string, target string) (inspectctx.Context, error) {
	return Build(root, files).GetContext(target), nil
}

const maxDiagnostics = 20

const maxSourceLines = 60

// GetContext resolves a target into a cross-file slice. Target forms:
//
//	"path/to/file.js"          -> file context (imports, importers, defs)
//	"path/to/file.js:symbol"   -> symbol context scoped to that file
//	"symbol"                   -> symbol context across the whole project
func (g *Graph) GetContext(target string) inspectctx.Context {
	var c inspectctx.Context
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

func (g *Graph) symbolContext(target, sym, scopeFile string) inspectctx.Context {
	c := inspectctx.Context{Target: target, Kind: "symbol"}
	for _, d := range g.Defs(sym) {
		if scopeFile != "" && d.File != scopeFile {
			continue
		}
		all := g.callersOf(sym, d)
		all = append(all, g.aliasCallers(sym, d)...)
		var resolved, unresolved []inspectctx.CallLoc
		for _, ca := range all {
			if ca.Resolved {
				resolved = append(resolved, ca)
			} else {
				unresolved = append(unresolved, ca)
			}
		}
		c.Definitions = append(c.Definitions, inspectctx.DefDetail{
			DefLoc:            d,
			Callees:           g.calleesIn(d),
			Callers:           resolved,
			UnresolvedCallers: unresolved,
			Source:            g.readSource(d.File, d.Line, d.EndLine),
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
func (g *Graph) callersOf(sym string, d inspectctx.DefLoc) []inspectctx.CallLoc {
	var callers []inspectctx.CallLoc
	for _, call := range g.Calls(sym) {
		if call.File == d.File {
			if call.Line >= d.Line && call.Line <= d.EndLine {
				continue // inside the definition itself
			}
			call.Resolved = true
			callers = append(callers, call)
			continue
		}
		if b, ok := g.binding(call.File, call.Recv, sym); ok {
			if g.bindingRefersTo(b, sym, call.Recv != "") &&
				(b.target == d.File || g.reachableFiles(b.target)[d.File]) {
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

// aliasCallers finds callers that reach d through a *renamed* local binding —
// `import { handler as h } from './a'; h()` or `const h = require('./a'); h()`.
// These calls are recorded under the local name (h), not sym (handler), so the
// sym-keyed scan in callersOf misses them. Here we walk bindings whose imported
// symbol is sym (or whose default import resolves to a file whose default
// export is sym) and attribute calls of the local name. All are binding-based,
// so Resolved is true.
func (g *Graph) aliasCallers(sym string, d inspectctx.DefLoc) []inspectctx.CallLoc {
	var callers []inspectctx.CallLoc
	for file, locals := range g.bindings {
		fp := g.Files[file]
		if fp == nil {
			continue
		}
		for local, b := range locals {
			if local == sym {
				continue // same-named: already handled by callersOf
			}
			if b.target != d.File && !g.reachableFiles(b.target)[d.File] {
				continue
			}
			match := b.imported == sym
			if !match && b.imported == "" {
				// default/namespace import: matches when sym is the target's default export
				if tfp := g.Files[b.target]; tfp != nil && tfp.DefaultExport == sym {
					match = true
				}
			}
			if !match {
				continue
			}
			for _, call := range fp.Calls {
				if call.Name == local && call.Recv == "" {
					callers = append(callers, inspectctx.CallLoc{File: file, Line: call.Line, Resolved: true})
				}
			}
		}
	}
	return callers
}

func (g *Graph) fileContext(file string) inspectctx.Context {
	c := inspectctx.Context{Target: file, Kind: "file"}
	c.Imports = g.Imports(file)
	c.ImportedBy = g.Importers(file)
	if fp := g.Files[file]; fp != nil {
		for _, d := range fp.Defs {
			c.Defs = append(c.Defs, inspectctx.DefLoc{
				Name: d.Name, File: file, Line: d.Line, EndLine: d.EndLine, Kind: d.Kind, Exported: d.Exported,
			})
		}
	}
	return c
}

// calleesIn returns the distinct callees within a definition's line range,
// qualified by receiver when present (e.g. "db.query", not just "query").
func (g *Graph) calleesIn(d inspectctx.DefLoc) []string {
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
