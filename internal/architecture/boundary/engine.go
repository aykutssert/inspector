package boundary

import "sort"

type Edge struct {
	From   string
	To     string
	Source string
	Line   int
}

type Graph struct {
	Files []string
	Edges map[string][]Edge
}

type Tag struct {
	Kind       string
	Reason     string
	Confidence string
}

type Classifier func(file string) []Tag

type Rule struct {
	ID          string
	SourceKind  string
	TargetKinds []string
	PassThrough func(file string) bool
}

type Violation struct {
	RuleID           string
	SourceFile       string
	SourceKind       string
	SourceReason     string
	TargetFile       string
	TargetKind       string
	TargetReason     string
	Confidence       string
	ImportSource     string
	Line             int
	Chain            []string
	TransparentEntry bool
}

func Analyze(g Graph, classify Classifier, rules []Rule) []Violation {
	tagsByFile := map[string][]Tag{}
	for _, file := range g.Files {
		tagsByFile[file] = classify(file)
	}

	var out []Violation
	for _, rule := range rules {
		targetKinds := set(rule.TargetKinds)
		for _, file := range sortedFiles(g.Files) {
			for _, sourceTag := range tagsByFile[file] {
				if sourceTag.Kind != rule.SourceKind {
					continue
				}
				for _, edge := range sortedEdges(g.Edges[file]) {
					out = append(out, evaluateEdge(g, tagsByFile, rule, targetKinds, file, sourceTag, edge)...)
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SourceFile != out[j].SourceFile {
			return out[i].SourceFile < out[j].SourceFile
		}
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		if out[i].TargetFile != out[j].TargetFile {
			return out[i].TargetFile < out[j].TargetFile
		}
		return out[i].RuleID < out[j].RuleID
	})
	return dedupe(out)
}

func evaluateEdge(g Graph, tagsByFile map[string][]Tag, rule Rule, targetKinds map[string]bool, sourceFile string, sourceTag Tag, edge Edge) []Violation {
	var out []Violation
	for _, targetTag := range tagsByFile[edge.To] {
		if targetKinds[targetTag.Kind] {
			out = append(out, violation(rule, sourceFile, sourceTag, targetTag, edge, []string{sourceFile, edge.To}, false))
		}
	}
	if rule.PassThrough == nil || !rule.PassThrough(edge.To) {
		return out
	}
	queue := []pathNode{{file: edge.To, chain: []string{sourceFile, edge.To}}}
	seen := map[string]bool{edge.To: true}
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		for _, next := range sortedEdges(g.Edges[node.file]) {
			if seen[next.To] {
				continue
			}
			seen[next.To] = true
			chain := append(append([]string(nil), node.chain...), next.To)
			for _, targetTag := range tagsByFile[next.To] {
				if targetKinds[targetTag.Kind] {
					out = append(out, violation(rule, sourceFile, sourceTag, targetTag, edge, chain, true))
				}
			}
			if rule.PassThrough(next.To) {
				queue = append(queue, pathNode{file: next.To, chain: chain})
			}
		}
	}
	return out
}

type pathNode struct {
	file  string
	chain []string
}

func violation(rule Rule, sourceFile string, sourceTag, targetTag Tag, edge Edge, chain []string, transparent bool) Violation {
	return Violation{
		RuleID:           rule.ID,
		SourceFile:       sourceFile,
		SourceKind:       sourceTag.Kind,
		SourceReason:     sourceTag.Reason,
		TargetFile:       chain[len(chain)-1],
		TargetKind:       targetTag.Kind,
		TargetReason:     targetTag.Reason,
		Confidence:       combinedConfidence(sourceTag.Confidence, targetTag.Confidence),
		ImportSource:     edge.Source,
		Line:             edge.Line,
		Chain:            chain,
		TransparentEntry: transparent,
	}
}

func combinedConfidence(values ...string) string {
	for _, v := range values {
		if v == "hint" {
			return "hint"
		}
	}
	return "rule"
}

func set(values []string) map[string]bool {
	out := map[string]bool{}
	for _, v := range values {
		out[v] = true
	}
	return out
}

func sortedFiles(files []string) []string {
	out := append([]string(nil), files...)
	sort.Strings(out)
	return out
}

func sortedEdges(edges []Edge) []Edge {
	out := append([]Edge(nil), edges...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		if out[i].Source != out[j].Source {
			return out[i].Source < out[j].Source
		}
		return out[i].To < out[j].To
	})
	return out
}

func dedupe(in []Violation) []Violation {
	seen := map[string]bool{}
	out := make([]Violation, 0, len(in))
	for _, v := range in {
		key := v.RuleID + "\x00" + v.SourceFile + "\x00" + v.TargetFile + "\x00" + v.ImportSource
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, v)
	}
	return out
}
