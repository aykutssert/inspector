package tseslint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aykutssert/scout/internal/core"
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

func TestScanNoFloatingPromises(t *testing.T) {
	repoRoot, ok := repoRootWithTypeScriptToolchainForTest()
	if !ok {
		t.Skip("typescript toolchain not installed")
	}
	t.Setenv("SCOUT_HOME", repoRoot)
	dir := t.TempDir()
	writeTSProjectFile(t, dir, "tsconfig.json", `{
  "compilerOptions": {
    "strict": true,
    "target": "ES2022",
    "module": "ESNext",
    "moduleResolution": "Node"
  },
  "include": ["src/**/*.ts"]
}`)
	if err := os.Mkdir(filepath.Join(dir, "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTSProjectFile(t, dir, "src/unsafe.ts", `async function saveAsync(): Promise<void> {
  return;
}

saveAsync();
`)
	writeTSProjectFile(t, dir, "src/safe.ts", `async function saveAsync(): Promise<void> {
  return;
}

async function main(): Promise<void> {
  await saveAsync();
  void saveAsync();
  saveAsync().catch(() => undefined);
}

void main();
`)

	got, err := New().Scan(core.ProjectContext{
		Root:  dir,
		Files: []string{"src/unsafe.ts", "src/safe.ts"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var floating []core.Finding
	for _, f := range got {
		if f.RuleID == "no-floating-promises" {
			floating = append(floating, f)
		}
	}
	if len(floating) != 1 {
		t.Fatalf("expected one no-floating-promises finding, got all=%#v floating=%#v", got, floating)
	}
	if !strings.HasSuffix(floating[0].File, "src/unsafe.ts") || floating[0].Line != 5 || floating[0].Severity != core.SeverityError {
		t.Fatalf("bad floating promise finding: %#v", floating[0])
	}
}

func repoRootWithTypeScriptToolchainForTest() (string, bool) {
	dir, err := os.Getwd()
	if err != nil {
		return "", false
	}
	for {
		path := filepath.Join(dir, "_toolchains", "typescript", "node_modules", ".bin", "eslint")
		if _, err := os.Stat(path); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func writeTSProjectFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
