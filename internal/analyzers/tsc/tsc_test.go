package tsc

import (
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
