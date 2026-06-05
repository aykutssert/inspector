package sveltelint

import (
	"testing"

	"github.com/aykutssert/scout/internal/core"
)

func TestMapSeverity(t *testing.T) {
	cases := map[int]core.Severity{
		2: core.SeverityError,
		1: core.SeverityWarning,
		0: core.SeverityInfo,
	}
	for in, want := range cases {
		if got := mapSeverity(in); got != want {
			t.Errorf("mapSeverity(%d)=%v want %v", in, got, want)
		}
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		rule string
		sev  core.Severity
		want string
	}{
		{"svelte/require-each-key", core.SeverityError, "bug"},
		{"svelte/no-at-html-tags", core.SeverityError, "security"},
		{"svelte/a11y-missing-attribute", core.SeverityWarning, "quality"},
		{"svelte/a11y-no-static-element-interactions", core.SeverityError, "quality"},
		{"svelte/no-unused-svelte-ignore", core.SeverityWarning, "quality"},
	}
	for _, c := range cases {
		if got := classify(c.rule, c.sev); got != c.want {
			t.Errorf("classify(%q,%v)=%q want %q", c.rule, c.sev, got, c.want)
		}
	}
}

// TestScanNoSvelteFiles verifies the analyzer is a no-op (and never shells out)
// when the scan contains no .svelte files.
func TestScanNoSvelteFiles(t *testing.T) {
	a := New()
	got, err := a.Scan(core.ProjectContext{Root: t.TempDir(), Files: []string{"a.ts", "b.jsx"}})
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("expected no findings for a non-Svelte scan, got %v", got)
	}
}
