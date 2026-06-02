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
}

type Graph struct {
	Root  string
	Files map[string]*FileParse // keyed by relative path

	defsBySymbol  map[string][]DefLoc
	callsBySymbol map[string][]CallLoc
	imports       map[string][]string // file -> resolved relative files it imports
	importers     map[string][]string // file -> files that import it
}

var resolveExts = []string{"", ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs"}

func Build(root string, files []string) *Graph {
	g := &Graph{
		Root:          root,
		Files:         map[string]*FileParse{},
		defsBySymbol:  map[string][]DefLoc{},
		callsBySymbol: map[string][]CallLoc{},
		imports:       map[string][]string{},
		importers:     map[string][]string{},
	}
	for _, rel := range files {
		fp, err := ParseJS(filepath.Join(root, rel))
		if err != nil {
			continue
		}
		fp.Path = rel
		g.Files[rel] = fp
	}
	for rel, fp := range g.Files {
		for _, d := range fp.Defs {
			g.defsBySymbol[d.Name] = append(g.defsBySymbol[d.Name], DefLoc{
				Name: d.Name, File: rel, Line: d.Line, EndLine: d.EndLine, Kind: d.Kind, Exported: d.Exported,
			})
		}
		for _, c := range fp.Calls {
			g.callsBySymbol[c.Name] = append(g.callsBySymbol[c.Name], CallLoc{File: rel, Line: c.Line})
		}
		for _, im := range fp.Imports {
			target := g.resolveImport(rel, im.Source)
			if target == "" {
				continue
			}
			g.imports[rel] = appendUnique(g.imports[rel], target)
			g.importers[target] = appendUnique(g.importers[target], rel)
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
		candidates = append(candidates, base+ext)
	}
	for _, ext := range resolveExts[1:] {
		candidates = append(candidates, filepath.Join(base, "index"+ext))
	}
	for _, c := range candidates {
		c = filepath.Clean(c)
		if _, ok := g.Files[c]; ok {
			return c
		}
		if _, err := os.Stat(filepath.Join(g.Root, c)); err == nil {
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
