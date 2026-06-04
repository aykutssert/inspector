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

// findCycles returns the cyclic import paths in the graph, each as an ordered
// list of files (the closing edge back to the first element is implied). Result
// is deterministic: nodes and edges are sorted, and cycles are ordered by their
// entry file.
func findCycles(g *jscontext.Graph) [][]string {
	nodes := make([]string, 0, len(g.Files))
	for f := range g.Files {
		nodes = append(nodes, f)
	}
	sort.Strings(nodes)

	inSet := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		inSet[n] = true
	}
	// Restrict edges to parsed files: a cycle's members must all be in the graph,
	// and an edge to an on-disk-but-unscanned file can never close a loop.
	adj := make(map[string][]string, len(nodes))
	for _, n := range nodes {
		var es []string
		for _, t := range g.Imports(n) {
			if inSet[t] {
				es = append(es, t)
			}
		}
		sort.Strings(es)
		adj[n] = es
	}

	var cycles [][]string
	for _, scc := range tarjanSCC(nodes, adj) {
		if len(scc) > 1 {
			cycles = append(cycles, orderCycle(scc, adj))
			continue
		}
		if n := scc[0]; hasEdge(adj[n], n) { // self-import
			cycles = append(cycles, []string{n})
		}
	}
	sort.Slice(cycles, func(i, j int) bool { return cycles[i][0] < cycles[j][0] })
	return cycles
}

// tarjanSCC returns the strongly connected components of the graph. Iterating
// nodes and edges in sorted order, and sorting each component, makes the output
// deterministic.
func tarjanSCC(nodes []string, adj map[string][]string) [][]string {
	index := map[string]int{}
	low := map[string]int{}
	onStack := map[string]bool{}
	var stack []string
	idx := 0
	var sccs [][]string

	var strongconnect func(v string)
	strongconnect = func(v string) {
		index[v] = idx
		low[v] = idx
		idx++
		stack = append(stack, v)
		onStack[v] = true
		for _, w := range adj[v] {
			if _, seen := index[w]; !seen {
				strongconnect(w)
				if low[w] < low[v] {
					low[v] = low[w]
				}
			} else if onStack[w] && index[w] < low[v] {
				low[v] = index[w]
			}
		}
		if low[v] == index[v] {
			var comp []string
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[w] = false
				comp = append(comp, w)
				if w == v {
					break
				}
			}
			sort.Strings(comp)
			sccs = append(sccs, comp)
		}
	}
	for _, v := range nodes {
		if _, seen := index[v]; !seen {
			strongconnect(v)
		}
	}
	return sccs
}

// orderCycle recovers one concrete cycle path within a strongly connected
// component, starting from its lexicographically smallest file, so the message
// reads as a real chain (a -> b -> c) rather than an unordered set.
func orderCycle(scc []string, adj map[string][]string) []string {
	set := make(map[string]bool, len(scc))
	for _, n := range scc {
		set[n] = true
	}
	start := scc[0] // scc is sorted
	visited := map[string]bool{}
	var path []string
	var dfs func(v string) bool
	dfs = func(v string) bool {
		path = append(path, v)
		visited[v] = true
		for _, w := range adj[v] {
			if !set[w] {
				continue
			}
			if w == start && len(path) > 1 {
				return true
			}
			if !visited[w] && dfs(w) {
				return true
			}
		}
		path = path[:len(path)-1]
		return false
	}
	if dfs(start) {
		return path
	}
	return scc // unreachable for a real SCC>1; sorted fallback
}

func hasEdge(targets []string, v string) bool {
	for _, t := range targets {
		if t == v {
			return true
		}
	}
	return false
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
