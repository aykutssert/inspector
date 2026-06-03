package importcycle

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/aykutssert/inspector/internal/core"
)

func TestTarjanDirectCycle(t *testing.T) {
	adj := map[string][]string{
		"a.ts": {"b.ts"},
		"b.ts": {"a.ts"},
		"c.ts": {"a.ts"}, // points in, not part of the cycle
	}
	got := cyclicComponents(tarjanSCC(sorted(adj), adj))
	want := [][]string{{"a.ts", "b.ts"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestTarjanNoCycle(t *testing.T) {
	adj := map[string][]string{
		"a.ts": {"b.ts"},
		"b.ts": {"c.ts"},
		"c.ts": nil,
	}
	if got := cyclicComponents(tarjanSCC(sorted(adj), adj)); len(got) != 0 {
		t.Fatalf("expected no cyclic component, got %v", got)
	}
}

func TestTarjanIndirectCycle(t *testing.T) {
	adj := map[string][]string{
		"a.ts": {"b.ts"},
		"b.ts": {"c.ts"},
		"c.ts": {"a.ts"},
	}
	got := cyclicComponents(tarjanSCC(sorted(adj), adj))
	want := [][]string{{"a.ts", "b.ts", "c.ts"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestTarjanTwoSeparateCycles(t *testing.T) {
	adj := map[string][]string{
		"a.ts": {"b.ts"},
		"b.ts": {"a.ts"},
		"x.ts": {"y.ts"},
		"y.ts": {"x.ts"},
	}
	got := cyclicComponents(tarjanSCC(sorted(adj), adj))
	if len(got) != 2 {
		t.Fatalf("expected two components, got %v", got)
	}
}

// orderCycle must yield a path that actually traverses real edges and returns
// to the start.
func TestOrderCycle(t *testing.T) {
	adj := map[string][]string{
		"a.ts": {"b.ts"},
		"b.ts": {"c.ts"},
		"c.ts": {"a.ts"},
	}
	path := orderCycle([]string{"a.ts", "b.ts", "c.ts"}, adj)
	if len(path) != 3 || path[0] != "a.ts" {
		t.Fatalf("want a-rooted 3-path, got %v", path)
	}
	// each consecutive pair (and the wrap-around) must be a real edge
	for i := range path {
		from := path[i]
		to := path[(i+1)%len(path)]
		if !hasEdge(adj[from], to) {
			t.Fatalf("path edge %s->%s is not real: %v", from, to, path)
		}
	}
}

// No JS/TS files → silent no-op (never builds a graph).
func TestScanNoJSFiles(t *testing.T) {
	got, err := New().Scan(core.ProjectContext{Root: t.TempDir(), Files: []string{"style.css", "data.json"}})
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestScanNoCycle(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a.ts", `import { b } from './b'; export const a = () => b();`)
	write(t, root, "b.ts", `export const b = () => 1;`)
	got, err := New().Scan(core.ProjectContext{Root: root, Files: []string{"a.ts", "b.ts"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no findings, got %#v", got)
	}
}

func TestScanDirectCycle(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a.ts", `import { b } from './b'; export const a = () => b;`)
	write(t, root, "b.ts", `import { a } from './a'; export const b = () => a;`)
	got, err := New().Scan(core.ProjectContext{Root: root, Files: []string{"a.ts", "b.ts"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected one finding, got %#v", got)
	}
	f := got[0]
	if f.RuleID != "import-cycle" || f.Severity != core.SeverityWarning || f.Confidence != core.ConfidenceRule {
		t.Fatalf("bad finding shape: %#v", f)
	}
	if f.File != "a.ts" || !strings.Contains(f.Message, "a.ts -> b.ts -> a.ts") {
		t.Fatalf("bad cycle message: %q (file %q)", f.Message, f.File)
	}
}

func TestScanIndirectCycle(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a.ts", `import { b } from './b'; export const a = 1;`)
	write(t, root, "b.ts", `import { c } from './c'; export const b = 2;`)
	write(t, root, "c.ts", `import { a } from './a'; export const c = 3;`)
	got, err := New().Scan(core.ProjectContext{Root: root, Files: []string{"a.ts", "b.ts", "c.ts"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected one finding, got %#v", got)
	}
	if !strings.Contains(got[0].Message, "a.ts -> b.ts -> c.ts -> a.ts") {
		t.Fatalf("bad 3-cycle message: %q", got[0].Message)
	}
}

func sorted(adj map[string][]string) []string {
	out := make([]string, 0, len(adj))
	for n := range adj {
		out = append(out, n)
	}
	// importcycle.findCycles sorts; mirror that for determinism in tests.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

// cyclicComponents filters Tarjan output to components that actually represent a
// cycle (more than one node), the same condition findCycles applies.
func cyclicComponents(sccs [][]string) [][]string {
	var out [][]string
	for _, s := range sccs {
		if len(s) > 1 {
			out = append(out, s)
		}
	}
	return out
}

func write(t *testing.T, root, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
