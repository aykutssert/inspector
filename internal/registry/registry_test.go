package registry

import (
	"testing"

	"github.com/aykutssert/inspector/internal/core"
	"github.com/aykutssert/inspector/internal/lang/javascript"
)

func TestDefaultRegistryProvidesScanSurface(t *testing.T) {
	r := Default()
	if got := len(r.ScanAdapters("rules")); got < 2 {
		t.Fatalf("default registry should include JS and Svelte scan adapters, got %d", got)
	}
	if got := len(r.ContextAdapters()); got == 0 {
		t.Fatal("default registry should include at least one context adapter")
	}
	jsCtx := core.ProjectContext{Languages: []string{"javascript"}}
	if got := len(r.Analyzers(jsCtx, nil)); got < 3 {
		t.Fatalf("JS scan should include global and JS pack analyzers, got %d", got)
	}
	providers := r.ContextProviders()
	if len(providers) != 1 || providers[0].Name() != "javascript" {
		t.Fatalf("default registry should expose the JavaScript context provider, got %#v", providers)
	}
}

// A repo whose languages none of the packs match must still get the global
// scanners, and must NOT pull in any pack's (language-specific) analyzers — so
// the fail-closed orchestrator can't demand a JS toolchain on a non-JS repo.
func TestAnalyzersGateOnDetection(t *testing.T) {
	r := Default()
	globals := len(r.Analyzers(core.ProjectContext{Languages: []string{"go"}}, nil))
	withJS := len(r.Analyzers(core.ProjectContext{Languages: []string{"javascript"}}, nil))
	if globals >= withJS {
		t.Fatalf("non-JS repo should run fewer analyzers than a JS repo: global=%d js=%d", globals, withJS)
	}
	if globals != 3 { // semgrep + osv + gitlog
		t.Fatalf("non-JS repo should run only the 3 global scanners, got %d", globals)
	}
}

func TestTailwindIsSeparatePack(t *testing.T) {
	if hasAnalyzer(javascript.JavaScript().Analyzers(), "tailwind-lint") {
		t.Fatal("JavaScript pack should not own Tailwind analysis")
	}
	if !hasAnalyzer(javascript.Tailwind().Analyzers(), "tailwind-lint") {
		t.Fatal("Tailwind pack should own tailwind-lint")
	}
	if got := javascript.Tailwind().Toolchains(); len(got) != 1 || got[0].Name != "tailwind" {
		t.Fatalf("Tailwind pack should declare the tailwind toolchain, got %#v", got)
	}
}

func hasAnalyzer(analyzers []core.Analyzer, name string) bool {
	for _, a := range analyzers {
		if a.Name() == name {
			return true
		}
	}
	return false
}
