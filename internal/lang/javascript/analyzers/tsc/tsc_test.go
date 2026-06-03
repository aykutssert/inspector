package tsc

import (
	"os"
	"path/filepath"
	"reflect"
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

// A TS file with no governing tsconfig anywhere up to root → no-op (tsc needs
// a config; we must not shell out).
func TestScanNoTSConfig(t *testing.T) {
	root := t.TempDir()
	got, err := New().Scan(core.ProjectContext{Root: root, Files: []string{"src/a.ts"}})
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestNearestTSConfigDir(t *testing.T) {
	root := t.TempDir()
	writeTSConfig(t, root)                    // root tsconfig
	web := filepath.Join(root, "apps", "web") // workspace with its own
	writeTSConfig(t, web)
	mkdir(t, filepath.Join(root, "lib")) // no tsconfig here

	// File inside a workspace resolves to that workspace, not the root.
	if got := nearestTSConfigDir(filepath.Join(web, "src"), root); got != web {
		t.Fatalf("workspace file: want %q, got %q", web, got)
	}
	// File without a nearer tsconfig falls back to the root one.
	if got := nearestTSConfigDir(filepath.Join(root, "lib"), root); got != root {
		t.Fatalf("root fallback: want %q, got %q", root, got)
	}
}

func TestNearestTSConfigDirNone(t *testing.T) {
	root := t.TempDir() // no tsconfig anywhere
	if got := nearestTSConfigDir(filepath.Join(root, "src"), root); got != "" {
		t.Fatalf("want empty, got %q", got)
	}
}

func TestTSProjectDirs(t *testing.T) {
	root := t.TempDir()
	web := filepath.Join(root, "apps", "web")
	ui := filepath.Join(root, "packages", "ui")
	writeTSConfig(t, web)
	writeTSConfig(t, ui)

	ctx := core.ProjectContext{
		Root: root,
		Files: []string{
			filepath.Join("apps", "web", "page.tsx"),
			filepath.Join("apps", "web", "util.ts"), // same project, de-duped
			filepath.Join("packages", "ui", "button.tsx"),
			filepath.Join("apps", "web", "style.css"), // non-TS, ignored
		},
	}
	want := []string{web, ui}
	sortStrings(want)
	if got := tsProjectDirs(ctx); !reflect.DeepEqual(got, want) {
		t.Fatalf("want %v, got %v", want, got)
	}
}

func TestNormalizeFile(t *testing.T) {
	root := "/repo"
	web := "/repo/apps/web"
	if got := normalizeFile(root, web, "src/a.ts"); got != filepath.Join("apps", "web", "src", "a.ts") {
		t.Fatalf("workspace path: got %q", got)
	}
	if got := normalizeFile(root, root, "src/a.ts"); got != filepath.Join("src", "a.ts") {
		t.Fatalf("root path: got %q", got)
	}
	if got := normalizeFile(root, web, ""); got != "" {
		t.Fatalf("empty path should pass through, got %q", got)
	}
}

func writeTSConfig(t *testing.T, dir string) {
	t.Helper()
	mkdir(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
