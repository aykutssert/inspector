// Package importcycle reports circular import dependencies among the scanned
// JavaScript/TypeScript files. Unlike Go, JS/TS compilers permit import cycles;
// they cause runtime "undefined"/"is not a function" errors when one module is
// used before the cycle finishes initializing, and they signal tight coupling
// that makes the involved modules hard to test or reuse in isolation.
//
// Detection runs entirely on inspector's own import graph (jscontext.Build) —
// pure Go, no external tool — using Tarjan's strongly-connected-components
// algorithm: any component with more than one file, or a single file that
// imports itself, is a cycle.
package importcycle

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/aykutssert/inspector/internal/architecture/cycle"
	"github.com/aykutssert/inspector/internal/core"
	jscontext "github.com/aykutssert/inspector/internal/lang/javascript/context"
)

type Analyzer struct{}

func New() *Analyzer { return &Analyzer{} }

var _ core.Analyzer = (*Analyzer)(nil)

func (a *Analyzer) Name() string { return "import-cycle" }

func (a *Analyzer) Available() bool { return true }

func (a *Analyzer) Scan(ctx core.ProjectContext) ([]core.Finding, error) {
	var files []string
	for _, f := range ctx.Files {
		if isJSTS(f) {
			files = append(files, f)
		}
	}
	if len(files) == 0 {
		return nil, nil
	}
	// In diff mode only changed files are scanned, so a cycle that runs through
	// an unchanged file can be missed; that is acceptable — the full scan catches
	// it. Build resolves only relative specifiers, so npm packages never appear.
	g := jscontext.Build(ctx.Root, files)
	cycles := findCycles(g)
	var findings []core.Finding
	for _, c := range cycles {
		findings = append(findings, cycleFinding(a.Name(), c))
	}
	findings = append(findings, internalBarrelImportFindings(a.Name(), g)...)
	return findings, nil
}

func isJSTS(f string) bool {
	switch strings.ToLower(filepath.Ext(f)) {
	case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs", ".mts", ".cts":
		return true
	}
	return false
}

// findCycles maps the JS import graph to the abstract graph and finds cycles.
func findCycles(g *jscontext.Graph) [][]string {
	nodes := make([]string, 0, len(g.Files))
	edges := make(map[string][]string)
	for f := range g.Files {
		nodes = append(nodes, f)
		edges[f] = g.Imports(f)
	}
	return cycle.FindCycles(cycle.Graph{
		Nodes: nodes,
		Edges: edges,
	})
}

func cycleFinding(analyzer string, cycle []string) core.Finding {
	chain := strings.Join(cycle, " -> ") + " -> " + cycle[0]
	return core.Finding{
		Analyzer:   analyzer,
		RuleID:     "import-cycle",
		Severity:   core.SeverityWarning,
		Level:      core.SeverityWarning.String(),
		Category:   "quality",
		Confidence: core.ConfidenceRule,
		File:       cycle[0],
		Message:    "Circular import dependency: " + chain + ". Cyclic modules can initialize out of order and throw runtime 'undefined'/'is not a function' errors, and the coupling makes them hard to test or reuse in isolation.",
		Fix:        "Break the cycle: extract the shared code both files need into a third module, or invert one of the dependencies.",
	}
}

func internalBarrelImportFindings(analyzer string, g *jscontext.Graph) []core.Finding {
	files := make([]string, 0, len(g.Files))
	for f := range g.Files {
		files = append(files, f)
	}
	sort.Strings(files)

	var out []core.Finding
	for _, file := range files {
		if isIndexFile(file) {
			continue
		}
		fp := g.Files[file]
		for _, im := range fp.Imports {
			target := g.ResolveImport(file, im.Source)
			if !isSameDirectoryIndex(file, target) {
				continue
			}
			out = append(out, core.Finding{
				Analyzer:   analyzer,
				RuleID:     "internal-barrel-import",
				Severity:   core.SeverityWarning,
				Level:      core.SeverityWarning.String(),
				Category:   "quality",
				Confidence: core.ConfidenceRule,
				File:       file,
				Line:       im.Line,
				Message:    file + " imports its own directory barrel (" + target + "). Internal files importing their local index barrel commonly create circular dependencies and hide the real module edge.",
				Fix:        "Import the sibling module directly instead of going through this directory's index barrel.",
			})
		}
	}
	return out
}

func isSameDirectoryIndex(file, target string) bool {
	if target == "" || !isIndexFile(target) {
		return false
	}
	return filepath.Dir(file) == filepath.Dir(target)
}

func isIndexFile(file string) bool {
	base := strings.ToLower(filepath.Base(file))
	switch base {
	case "index.js", "index.jsx", "index.ts", "index.tsx", "index.mjs", "index.cjs", "index.mts", "index.cts":
		return true
	default:
		return false
	}
}
