package svelte

import "testing"

func TestSveltePackIncludesHintAnalyzer(t *testing.T) {
	pack := Svelte()
	coverage := pack.Coverage()
	if !coverage.Security || !coverage.Hints {
		t.Fatalf("Svelte pack should advertise security and hint coverage, got %#v", coverage)
	}
	var names []string
	for _, a := range pack.Analyzers() {
		names = append(names, a.Name())
	}
	if !hasAnalyzer(names, "svelte-lint") || !hasAnalyzer(names, "svelte-hint") {
		t.Fatalf("Svelte pack analyzers = %v, want svelte-lint and svelte-hint", names)
	}
}

func hasAnalyzer(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}
