package oxlint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aykutssert/scout/internal/core"
)

func TestBuildConfigGatesNextjs(t *testing.T) {
	if strings.Contains(buildConfig(true, false), "nextjs") {
		t.Fatal("non-Next config must not enable the nextjs plugin")
	}
	if !strings.Contains(buildConfig(true, true), "nextjs") {
		t.Fatal("next config must enable the nextjs plugin")
	}
}

func TestBuildConfigGatesReactPlugins(t *testing.T) {
	nonReact := buildConfig(false, false)
	for _, p := range []string{`"react"`, `"react-perf"`, `"jsx-a11y"`} {
		if strings.Contains(nonReact, p) {
			t.Fatalf("non-React config must not enable %s plugin", p)
		}
	}
	// React-only rule overrides must not appear when the plugins are absent;
	// oxlint would reject an override for an unloaded plugin's rule.
	if strings.Contains(nonReact, "react/") || strings.Contains(nonReact, "react-perf/") {
		t.Fatal("non-React config must not reference react-family rules")
	}
	react := buildConfig(true, false)
	for _, p := range []string{`"react"`, `"react-perf"`, `"jsx-a11y"`, `"react-hooks"`} {
		if !strings.Contains(react, p) {
			t.Fatalf("React config must enable %s plugin", p)
		}
	}
	if !strings.Contains(react, "react/button-has-type") {
		t.Fatal("React config must carry the react rule overrides")
	}
	// rules-of-hooks is not enabled by the default categories, so it must be
	// pinned explicitly or the most important hook bug rule silently goes dark.
	if !strings.Contains(react, "react-hooks/rules-of-hooks") {
		t.Fatal("React config must explicitly enable react-hooks/rules-of-hooks")
	}
	if !strings.Contains(react, "react-hooks/exhaustive-deps") {
		t.Fatal("React config must enable react-hooks/exhaustive-deps")
	}
}

func TestBuildConfigDisablesCoreNoiseRules(t *testing.T) {
	cfg := buildConfig(false, false)
	for _, rule := range []string{"no-unused-vars", "no-shadow", "no-underscore-dangle"} {
		if !strings.Contains(cfg, `"`+rule+`": "off"`) {
			t.Fatalf("%s should be disabled in oxlint config: %s", rule, cfg)
		}
	}
}

// TestOxlintRuleCoverage runs the real oxlint binary over fixtures and asserts
// every rule the scout explicitly enables for React still fires, and every
// rule it suppresses stays silent. The config-string tests above prove intent;
// this proves behavior, catching an oxlint upgrade or rule rename that silently
// drops a rule we depend on. Skips when oxlint is absent unless
// SCOUT_REQUIRE_OXLINT=1 forces it.
func TestOxlintRuleCoverage(t *testing.T) {
	a := New()
	if !a.Available() {
		if os.Getenv("SCOUT_REQUIRE_OXLINT") == "1" {
			t.Fatal("oxlint is required but not installed")
		}
		t.Skip("oxlint not installed")
	}

	root, err := filepath.Abs(filepath.Join("testdata", "coverage"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := a.Scan(core.ProjectContext{
		Root:  root,
		Files: []string{"react_rules.tsx", "noise.tsx"},
	})
	if err != nil {
		t.Fatal(err)
	}
	fired := make(map[string]bool, len(got))
	for _, f := range got {
		fired[f.RuleID] = true
	}

	// Enabled rules whose firing the scout relies on (see reactRules).
	for _, rule := range []string{
		"button-has-type",
		"rules-of-hooks",
		"exhaustive-deps",
		"no-array-index-key",
		"prefer-number-properties", // unicorn quality/modernization set
	} {
		if !fired[rule] {
			t.Errorf("enabled oxlint rule %q did not fire — config drift or oxlint rename?", rule)
		}
	}
	// Suppressed noise rules must never reach a finding.
	for _, rule := range []string{
		"no-underscore-dangle",
		"no-unused-vars",
		"no-shadow",
	} {
		if fired[rule] {
			t.Errorf("suppressed noise rule %q fired — suppression regressed", rule)
		}
	}
}

func TestCoreNoiseRuleFilter(t *testing.T) {
	cases := []struct {
		plugin string
		rule   string
		want   bool
	}{
		{"", "no-unused-vars", true},
		{"eslint", "no-unused-vars", true},
		{"oxc", "no-shadow", true},
		{"eslint", "no-underscore-dangle", true},
		{"react", "no-unused-vars", false},
		{"react-hooks", "exhaustive-deps", false},
	}
	for _, tc := range cases {
		if got := isCoreNoiseRule(tc.plugin, tc.rule); got != tc.want {
			t.Fatalf("isCoreNoiseRule(%q, %q)=%v, want %v", tc.plugin, tc.rule, got, tc.want)
		}
	}
}

func TestIsReactProjectByDependency(t *testing.T) {
	dir := t.TempDir()
	pkg := `{"dependencies":{"react":"18.0.0","react-dom":"18.0.0"}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0o644); err != nil {
		t.Fatal(err)
	}
	if !isReactProject(core.ProjectContext{Root: dir}) {
		t.Fatal("react dependency should mark a React project")
	}
}

func TestIsReactProjectByJSXFile(t *testing.T) {
	dir := t.TempDir()
	ctx := core.ProjectContext{Root: dir, Files: []string{filepath.Join(dir, "App.tsx")}}
	if !isReactProject(ctx) {
		t.Fatal(".tsx file should mark a React project")
	}
}

func TestBackendNotReact(t *testing.T) {
	dir := t.TempDir()
	pkg := `{"dependencies":{"express":"4.18.0"}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := core.ProjectContext{Root: dir, Files: []string{filepath.Join(dir, "server.js")}}
	if isReactProject(ctx) {
		t.Fatal("a plain Express backend must not be detected as React")
	}
}

func TestIsNextProjectByConfigFile(t *testing.T) {
	dir := t.TempDir()
	ctx := core.ProjectContext{Root: dir, Files: []string{filepath.Join(dir, "next.config.mjs")}}
	if !isNextProject(ctx) {
		t.Fatal("next.config.mjs should mark a Next.js project")
	}
}

func TestIsNextProjectByDependency(t *testing.T) {
	dir := t.TempDir()
	pkg := `{"dependencies":{"next":"14.2.0","react":"18.0.0"}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0o644); err != nil {
		t.Fatal(err)
	}
	if !isNextProject(core.ProjectContext{Root: dir}) {
		t.Fatal("next dependency should mark a Next.js project")
	}
}

func TestIsNextProjectByDevDependency(t *testing.T) {
	dir := t.TempDir()
	pkg := `{"devDependencies":{"next":"14.2.0"}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0o644); err != nil {
		t.Fatal(err)
	}
	if !isNextProject(core.ProjectContext{Root: dir}) {
		t.Fatal("next devDependency should mark a Next.js project")
	}
}

func TestPlainReactNotNext(t *testing.T) {
	dir := t.TempDir()
	pkg := `{"dependencies":{"react":"18.0.0","react-dom":"18.0.0"}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0o644); err != nil {
		t.Fatal(err)
	}
	if isNextProject(core.ProjectContext{Root: dir}) {
		t.Fatal("a plain React app must not be detected as Next.js")
	}
}

func TestNoPackageJSONNotNext(t *testing.T) {
	if isNextProject(core.ProjectContext{Root: t.TempDir()}) {
		t.Fatal("missing package.json must not be detected as Next.js")
	}
}
