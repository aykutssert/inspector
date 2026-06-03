package codegraph

import (
	"os"
	"path/filepath"
	"strings"
)

type DefLoc struct {
	Name     string `json:"name"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	EndLine  int    `json:"end_line"`
	Kind     string `json:"kind"`
	Exported bool   `json:"exported"`
}

type CallLoc struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Recv string `json:"recv,omitempty"`
	// Resolved is true when the call was tied to this definition through an
	// import binding (high confidence); false when attributed by the looser
	// name + reachability heuristic (e.g. a dynamic receiver like res.x()).
	Resolved bool `json:"resolved"`
}

type Graph struct {
	Root        string
	Files       map[string]*FileParse // keyed by relative path
	Diagnostics []string              // parse failures / syntax errors

	defsBySymbol  map[string][]DefLoc
	callsBySymbol map[string][]CallLoc
	imports       map[string][]string                   // file -> resolved relative files it imports
	importers     map[string][]string                   // file -> files that import it
	bindings      map[string]map[string]resolvedBinding // file -> local name -> binding
	reachCache    map[string]map[string]bool
}

// resolvedBinding ties a local name to the file it resolves to and the original
// exported symbol it refers to ("" = default / namespace / whole module).
type resolvedBinding struct {
	target   string
	imported string
}

// bindingTarget resolves the file a call's binding points to. For a method
// call it keys on the receiver (db in db.query()); for a bare call on the
// callee name. ok is false when no import binding introduced that name.
func (g *Graph) bindingTarget(file, recv, name string) (string, bool) {
	bm := g.bindings[file]
	if bm == nil {
		return "", false
	}
	key := name
	if recv != "" {
		key = recv
	}
	b, ok := bm[key]
	return b.target, ok
}

// binding returns the full resolved binding introduced for a call's name. Keys
// on the receiver for a method call (db in db.query()), else on the callee name.
func (g *Graph) binding(file, recv, name string) (resolvedBinding, bool) {
	bm := g.bindings[file]
	if bm == nil {
		return resolvedBinding{}, false
	}
	key := name
	if recv != "" {
		key = recv
	}
	b, ok := bm[key]
	return b, ok
}

// bindingRefersTo reports whether a binding actually refers to sym. For a member
// access (member=true, e.g. api.foo) the member name is the symbol regardless of
// how the namespace/object was imported. For a bare call the local name equals
// sym, so the binding must import sym by name, or be a default/whole-module
// import whose target's default export is sym — this stops a default import
// (import Foo from './m') from being attributed to a same-named *named* export.
func (g *Graph) bindingRefersTo(b resolvedBinding, sym string, member bool) bool {
	if member {
		return true
	}
	if b.imported == sym {
		return true
	}
	if b.imported == "" {
		if tfp := g.Files[b.target]; tfp != nil && tfp.DefaultExport == sym {
			return true
		}
	}
	return false
}

// reachableFiles returns the set of files transitively imported by `from`
// (not including `from` itself). Used to attribute a call site to the
// definition it can actually reach through the import graph.
func (g *Graph) reachableFiles(from string) map[string]bool {
	if g.reachCache == nil {
		g.reachCache = map[string]map[string]bool{}
	}
	if r, ok := g.reachCache[from]; ok {
		return r
	}
	seen := map[string]bool{}
	queue := []string{from}
	for len(queue) > 0 {
		f := queue[0]
		queue = queue[1:]
		for _, t := range g.imports[f] {
			if !seen[t] {
				seen[t] = true
				queue = append(queue, t)
			}
		}
	}
	g.reachCache[from] = seen
	return seen
}

var resolveExts = []string{"", ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs", ".mts", ".cts"}

func Build(root string, files []string) *Graph {
	g := &Graph{
		Root:          root,
		Files:         map[string]*FileParse{},
		defsBySymbol:  map[string][]DefLoc{},
		callsBySymbol: map[string][]CallLoc{},
		imports:       map[string][]string{},
		importers:     map[string][]string{},
		bindings:      map[string]map[string]resolvedBinding{},
	}
	for _, rel := range files {
		fp, err := ParseJS(filepath.Join(root, rel))
		if err != nil {
			g.Diagnostics = append(g.Diagnostics, "parse failed: "+rel+": "+err.Error())
			continue
		}
		fp.Path = rel
		if fp.HasError {
			g.Diagnostics = append(g.Diagnostics, "syntax errors (partial AST): "+rel)
		}
		g.Files[rel] = fp
	}
	for rel, fp := range g.Files {
		for _, d := range fp.Defs {
			g.defsBySymbol[d.Name] = append(g.defsBySymbol[d.Name], DefLoc{
				Name: d.Name, File: rel, Line: d.Line, EndLine: d.EndLine, Kind: d.Kind, Exported: d.Exported,
			})
		}
		for _, c := range fp.Calls {
			g.callsBySymbol[c.Name] = append(g.callsBySymbol[c.Name], CallLoc{File: rel, Line: c.Line, Recv: c.Recv})
		}
		for _, im := range fp.Imports {
			target := g.resolveImport(rel, im.Source)
			if target == "" {
				continue
			}
			g.imports[rel] = appendUnique(g.imports[rel], target)
			g.importers[target] = appendUnique(g.importers[target], rel)
		}
		for _, b := range fp.Bindings {
			target := g.resolveImport(rel, b.Source)
			if target == "" {
				continue
			}
			if g.bindings[rel] == nil {
				g.bindings[rel] = map[string]resolvedBinding{}
			}
			g.bindings[rel][b.Local] = resolvedBinding{target: target, imported: b.Imported}
		}
	}
	return g
}

// resolveImport turns a relative specifier into a known relative file path.
// Bare specifiers (npm packages, node: builtins) return "".
func (g *Graph) resolveImport(fromFile, spec string) string {
	if !strings.HasPrefix(spec, ".") {
		return ""
	}
	base := filepath.Join(filepath.Dir(fromFile), spec)
	candidates := make([]string, 0, len(resolveExts)*2)
	for _, ext := range resolveExts {
		if ext == "" && (base == "." || strings.HasSuffix(spec, "/")) {
			continue // a directory import resolves via index.<ext>, not the dir itself
		}
		candidates = append(candidates, base+ext)
	}
	for _, ext := range resolveExts[1:] {
		candidates = append(candidates, filepath.Join(base, "index"+ext))
	}
	for _, c := range candidates {
		c = filepath.Clean(c)
		if c == "." || c == "" {
			continue
		}
		if _, ok := g.Files[c]; ok {
			return c
		}
		if info, err := os.Stat(filepath.Join(g.Root, c)); err == nil && !info.IsDir() {
			return c
		}
	}
	return ""
}

func (g *Graph) Defs(symbol string) []DefLoc    { return g.defsBySymbol[symbol] }
func (g *Graph) Calls(symbol string) []CallLoc  { return g.callsBySymbol[symbol] }
func (g *Graph) Imports(file string) []string   { return g.imports[file] }
func (g *Graph) Importers(file string) []string { return g.importers[file] }

func appendUnique(s []string, v string) []string {
	for _, x := range s {
		if x == v {
			return s
		}
	}
	return append(s, v)
}
