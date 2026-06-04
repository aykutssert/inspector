package cycle

import "sort"

// Graph represents a generic directed graph of entities and their dependency edges.
type Graph struct {
	Nodes []string
	Edges map[string][]string // adjacency list representation
}

// FindCycles returns the cyclic dependency paths in the graph, each as an ordered
// list of nodes (the closing edge back to the first element is implied).
// Result is deterministic: nodes and edges are sorted, and cycles are ordered by their
// entry node.
func FindCycles(g Graph) [][]string {
	nodes := make([]string, 0, len(g.Nodes))
	nodes = append(nodes, g.Nodes...)
	sort.Strings(nodes)

	inSet := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		inSet[n] = true
	}

	adj := make(map[string][]string, len(nodes))
	for _, n := range nodes {
		var es []string
		for _, t := range g.Edges[n] {
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
		if n := scc[0]; hasEdge(adj[n], n) { // self-reference
			cycles = append(cycles, []string{n})
		}
	}
	sort.Slice(cycles, func(i, j int) bool { return cycles[i][0] < cycles[j][0] })
	return cycles
}

// tarjanSCC returns the strongly connected components of the graph.
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

// orderCycle recovers one concrete cycle path within a strongly connected component,
// starting from its lexicographically smallest node.
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
	return scc
}

func hasEdge(targets []string, v string) bool {
	for _, t := range targets {
		if t == v {
			return true
		}
	}
	return false
}
