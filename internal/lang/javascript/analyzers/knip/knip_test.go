package knip

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/aykutssert/inspector/internal/core"
)

// Real knip 5.x `--reporter json` output, captured from a fixture with an
// orphan file, an unused export, and an unused dependency.
const sampleReport = `{
  "files": ["orphan.js"],
  "issues": [
    {
      "file": "package.json",
      "dependencies": [{"name": "left-pad"}],
      "devDependencies": [],
      "optionalPeerDependencies": [],
      "unlisted": [{"name": "react"}],
      "exports": [],
      "types": []
    },
    {
      "file": "used.js",
      "dependencies": [],
      "exports": [{"name": "neverUsed", "line": 2, "col": 17, "pos": 53}],
      "types": [{"name": "UnusedType", "line": 5, "col": 13}]
    }
  ]
}`

func TestFindingsFromReport(t *testing.T) {
	var r knipReport
	if err := json.Unmarshal([]byte(sampleReport), &r); err != nil {
		t.Fatal(err)
	}
	got := r.findings("knip")

	byRule := map[string][]core.Finding{}
	for _, f := range got {
		if f.Analyzer != "knip" || f.Confidence != core.ConfidenceHint {
			t.Fatalf("unexpected analyzer/confidence: %#v", f)
		}
		byRule[f.RuleID] = append(byRule[f.RuleID], f)
	}

	if n := len(byRule["unused-file"]); n != 1 || byRule["unused-file"][0].File != "orphan.js" {
		t.Fatalf("unused-file: %#v", byRule["unused-file"])
	}
	// exports + types both fold into unused-export.
	if n := len(byRule["unused-export"]); n != 2 {
		t.Fatalf("want 2 unused-export, got %#v", byRule["unused-export"])
	}
	var sawExportLine bool
	for _, f := range byRule["unused-export"] {
		if f.File != "used.js" {
			t.Fatalf("unused-export file: %q", f.File)
		}
		if f.Line == 2 {
			sawExportLine = true
		}
	}
	if !sawExportLine {
		t.Fatal("unused-export did not carry the export line")
	}
	if n := len(byRule["unused-dependency"]); n != 1 || byRule["unused-dependency"][0].Severity != core.SeverityWarning {
		t.Fatalf("unused-dependency: %#v", byRule["unused-dependency"])
	}
	// unlisted is a correctness problem, categorized as a bug.
	if n := len(byRule["unlisted-dependency"]); n != 1 || byRule["unlisted-dependency"][0].Category != "bug" {
		t.Fatalf("unlisted-dependency: %#v", byRule["unlisted-dependency"])
	}
}

// Diff mode is whole-project-inappropriate, so knip stays silent.
func TestScanDiffModeSkips(t *testing.T) {
	got, err := New().Scan(core.ProjectContext{Root: t.TempDir(), DiffOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("expected nil in diff mode, got %v", got)
	}
}

// No package.json → nothing for knip to anchor on; silent no-op.
func TestScanNoPackageJSON(t *testing.T) {
	got, err := New().Scan(core.ProjectContext{Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("expected nil without package.json, got %v", got)
	}
}

// package.json but no node_modules → a single info skip notice, never an error.
func TestScanMissingNodeModules(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := New().Scan(core.ProjectContext{Root: dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].RuleID != "dead-code-skipped" || got[0].Severity != core.SeverityInfo {
		t.Fatalf("expected one info skip notice, got %#v", got)
	}
}

func TestRunFailureFirstLineOnly(t *testing.T) {
	got := runFailure("knip", "Config error: bad entry\nstack trace line")
	if got.RuleID != "dead-code-failed" {
		t.Fatalf("bad rule: %#v", got)
	}
	if want := "Dead-code analysis could not complete: Config error: bad entry"; got.Message != want {
		t.Fatalf("want %q, got %q", want, got.Message)
	}
}
