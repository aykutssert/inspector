package tailwindlint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aykutssert/scout/internal/core"
)

func TestClassifyAndConfidence(t *testing.T) {
	// Conflicting classnames are a deterministic visual bug.
	if got := classify("tailwindcss/no-contradicting-classname"); got != "bug" {
		t.Fatalf("contradicting classname should be a bug, got %q", got)
	}
	if got := confidence("tailwindcss/no-contradicting-classname"); got != core.ConfidenceRule {
		t.Fatalf("contradicting classname should be a rule, got %q", got)
	}
	// Shorthand collapsing is a stylistic suggestion the agent verifies.
	if got := classify("tailwindcss/enforces-shorthand"); got != "quality" {
		t.Fatalf("shorthand should be quality, got %q", got)
	}
	if got := confidence("tailwindcss/enforces-shorthand"); got != core.ConfidenceHint {
		t.Fatalf("shorthand should be a hint, got %q", got)
	}
}

func TestHasTailwindInstalled(t *testing.T) {
	root := t.TempDir()
	if hasTailwindInstalled(root) {
		t.Fatal("empty project must not report tailwind installed")
	}
	if err := os.MkdirAll(filepath.Join(root, "node_modules", "tailwindcss"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !hasTailwindInstalled(root) {
		t.Fatal("node_modules/tailwindcss should mark tailwind installed")
	}
}

func TestScanSkipsWhenTailwindAbsent(t *testing.T) {
	root := t.TempDir()
	// JS target present but no tailwindcss installed: must skip silently (the
	// plugin would otherwise crash resolving tailwindcss) — no findings, no error.
	jsx := filepath.Join(root, "App.jsx")
	if err := os.WriteFile(jsx, []byte(`export const C = () => <div className="block inline" />;`), 0o644); err != nil {
		t.Fatal(err)
	}
	findings, err := New().Scan(core.ProjectContext{Root: root, Files: []string{jsx}})
	if err != nil {
		t.Fatalf("scan should not error when tailwind is absent: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected no findings without tailwind, got %d", len(findings))
	}
}

func TestScanNoTargetsNoFindings(t *testing.T) {
	root := t.TempDir()
	findings, err := New().Scan(core.ProjectContext{Root: root, Files: []string{filepath.Join(root, "main.go")}})
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Fatalf("non-JS files must yield no tailwind findings, got %d", len(findings))
	}
}
