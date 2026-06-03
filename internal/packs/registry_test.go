package packs

import (
	"testing"

	"github.com/aykutssert/inspector/internal/core"
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
