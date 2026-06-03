package tseslint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aykutssert/inspector/internal/core"
)

// A sub-package tsconfig (no root tsconfig) must still register the project, so
// monorepos are no longer skipped wholesale.
func TestTSConfigDirsMonorepo(t *testing.T) {
	root := t.TempDir()
	web := filepath.Join(root, "apps", "web")
	if err := os.MkdirAll(web, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(web, "tsconfig.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := core.ProjectContext{
		Root: root,
		Files: []string{
			filepath.Join("apps", "web", "page.tsx"),
			filepath.Join("apps", "web", "x.js"), // non-TS, ignored
		},
	}
	got := tsConfigDirs(ctx)
	if len(got) != 1 || got[0] != web {
		t.Fatalf("want [%s], got %v", web, got)
	}
}

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

func TestParserNoticeBecomesFinding(t *testing.T) {
	got := eslintFindings("ts-eslint", []eslintFile{{
		FilePath: "src/a.ts",
		Messages: []struct {
			RuleID   string `json:"ruleId"`
			Severity int    `json:"severity"`
			Message  string `json:"message"`
			Line     int    `json:"line"`
		}{{
			RuleID:   "",
			Severity: 2,
			Message:  "Parsing error: ESLint was configured to run on this file, but that TSConfig does not include it.",
			Line:     1,
		}},
	}})
	if len(got) != 1 {
		t.Fatalf("expected one finding, got %#v", got)
	}
	if got[0].RuleID != "parser-error" || got[0].File != "src/a.ts" || got[0].Severity != core.SeverityError {
		t.Fatalf("bad parser finding: %#v", got[0])
	}
}

func TestESLintRunFailure(t *testing.T) {
	got := eslintRunFailure("ts-eslint", "Oops! Something went wrong\nsecond line")
	if got.RuleID != "type-aware-lint-failed" || !strings.Contains(got.Message, "Oops! Something went wrong") {
		t.Fatalf("bad run failure: %#v", got)
	}
	if strings.Contains(got.Message, "second line") {
		t.Fatalf("message should keep first line only: %q", got.Message)
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
