package tseslint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aykutssert/inspector/internal/core"
)

func TestMapSeverity(t *testing.T) {
	if mapSeverity(2) != core.SeverityError || mapSeverity(1) != core.SeverityWarning || mapSeverity(0) != core.SeverityInfo {
		t.Fatal("severity mapping wrong")
	}
}

func TestClassify(t *testing.T) {
	if classify(core.SeverityError) != "bug" {
		t.Fatal("error should be bug")
	}
	if classify(core.SeverityWarning) != "quality" {
		t.Fatal("warning should be quality")
	}
}

// No TS files → silent no-op.
func TestScanNoTSFiles(t *testing.T) {
	got, err := New().Scan(core.ProjectContext{Root: t.TempDir(), Files: []string{"a.js", "b.svelte"}})
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

// TS files but no tsconfig → no-op (type-aware needs a project), not an error.
func TestScanNoTSConfig(t *testing.T) {
	got, err := New().Scan(core.ProjectContext{Root: t.TempDir(), Files: []string{"a.ts"}})
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("expected nil without tsconfig, got %v", got)
	}
}

// TS files + tsconfig but no node_modules → a single info notice, not an error.
func TestScanMissingDepsNotice(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := New().Scan(core.ProjectContext{Root: dir, Files: []string{"a.ts"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].RuleID != "type-aware-skipped" || got[0].Severity != core.SeverityInfo {
		t.Fatalf("expected one info skip notice, got %#v", got)
	}
}
