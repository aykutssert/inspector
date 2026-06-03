package tsc

import (
	"strings"
	"testing"

	"github.com/aykutssert/inspector/internal/core"
)

func TestDiagRe(t *testing.T) {
	line := "src/a.ts(3,7): error TS2322: Type 'number' is not assignable to type 'string'."
	m := diagRe.FindStringSubmatch(line)
	if m == nil {
		t.Fatal("expected a match")
	}
	if m[1] != "src/a.ts" || m[2] != "3" || m[4] != "error" || m[5] != "TS2322" {
		t.Fatalf("bad capture: %#v", m[1:])
	}
}

func TestDiagReIgnoresNonDiag(t *testing.T) {
	for _, line := range []string{"", "Found 1 error.", "  some context line"} {
		if diagRe.FindStringSubmatch(line) != nil {
			t.Fatalf("should not match: %q", line)
		}
	}
}

func TestParseGlobalDiagnostic(t *testing.T) {
	got := parseDiagnostics("tsc", "error TS18003: No inputs were found in config file.\n")
	if len(got) != 1 {
		t.Fatalf("expected one finding, got %#v", got)
	}
	if got[0].RuleID != "TS18003" || got[0].File != "" || got[0].Severity != core.SeverityError {
		t.Fatalf("bad global diagnostic: %#v", got[0])
	}
}

func TestUnparsedOutputNotice(t *testing.T) {
	got := unparsedOutputNotice("tsc", "Unexpected compiler output\nsecond line")
	if got.RuleID != "type-check-output-unparsed" || !strings.Contains(got.Message, "Unexpected compiler output") {
		t.Fatalf("bad unparsed notice: %#v", got)
	}
	if strings.Contains(got.Message, "second line") {
		t.Fatalf("message should keep first line only: %q", got.Message)
	}
}

func TestHasTSFiles(t *testing.T) {
	if !hasTSFiles([]string{"a.js", "b.tsx"}) {
		t.Fatal(".tsx should count")
	}
	if hasTSFiles([]string{"a.js", "b.svelte"}) {
		t.Fatal("no TS files present")
	}
}

// No TS files → silent no-op (analyzer never shells out).
func TestScanNoTSFiles(t *testing.T) {
	got, err := New().Scan(core.ProjectContext{Root: t.TempDir(), Files: []string{"a.js"}})
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}
