package registry

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aykutssert/scout/internal/core"
	"github.com/aykutssert/scout/internal/lang/javascript"
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

// bun.* rules must be suppressed on a plain Node repo (they reference Bun
// globals absent in Node) but kept on a Bun repo. dropInapplicableRules is the
// single gate; assert both directions.
func TestDropInapplicableRulesGatesBun(t *testing.T) {
	bunFinding := core.Finding{RuleID: "bun.bun-prefer-bun-password"}
	viteFinding := core.Finding{RuleID: "vite.vite-process-env-usage"}
	other := core.Finding{RuleID: "general.process-env-dispersed-access"}

	node := core.ProjectContext{Root: t.TempDir(), Files: []string{"index.ts"}}
	dropNode := dropInapplicableRules(node)
	if dropNode == nil || !dropNode(bunFinding) || !dropNode(viteFinding) {
		t.Fatal("plain Node repo should drop bun.* and vite.* findings")
	}
	if dropNode(other) {
		t.Fatal("non-framework rules must not be dropped")
	}

	bunDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(bunDir, "package.json"),
		[]byte(`{"dependencies":{"bun-types":"^1.0.0"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	bunCtx := core.ProjectContext{Root: bunDir, Files: []string{"index.ts"}}
	dropBun := dropInapplicableRules(bunCtx)
	if dropBun == nil || dropBun(bunFinding) || !dropBun(viteFinding) {
		t.Fatal("Bun repo should keep bun.* but drop vite.* findings")
	}

	viteDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(viteDir, "package.json"),
		[]byte(`{"dependencies":{"vite":"^1.0.0"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	viteCtx := core.ProjectContext{Root: viteDir, Files: []string{"index.ts"}}
	dropVite := dropInapplicableRules(viteCtx)
	if dropVite == nil || !dropVite(bunFinding) || dropVite(viteFinding) {
		t.Fatal("Vite repo should keep vite.* but drop bun.* findings")
	}

	bothDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(bothDir, "package.json"),
		[]byte(`{"dependencies":{"bun-types":"^1.0.0", "vite":"^1.0.0"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	bothCtx := core.ProjectContext{Root: bothDir, Files: []string{"index.ts"}}
	dropBoth := dropInapplicableRules(bothCtx)
	if dropBoth != nil {
		if dropBoth(bunFinding) || dropBoth(viteFinding) {
			t.Fatal("Bun+Vite repo should keep both findings")
		}
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
