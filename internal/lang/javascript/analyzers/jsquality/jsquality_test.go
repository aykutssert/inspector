package jsquality

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aykutssert/inspector/internal/core"
)

func scanSrc(t *testing.T, name, src string) []core.Finding {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, name), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	findings, err := New().Scan(core.ProjectContext{Root: root, Files: []string{name}})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	return findings
}

func hasRule(findings []core.Finding, rule string) bool {
	for _, f := range findings {
		if f.RuleID == rule {
			return true
		}
	}
	return false
}

// The whole point of this analyzer: repeated magic literals are flagged on a
// plain (non-React) TypeScript file. reacthint would never run here because it
// is gated on a React/Next signal, so this signal must live at the language
// level.
func TestRepeatedMagicLiteralOnPlainTypeScript(t *testing.T) {
	src := `const statusA = "pending-review";
const statusB = "pending-review";
const statusC = "pending-review";
const statusD = "pending-review";
const retryA = 30;
const retryB = 30;
const retryC = 30;`
	findings := scanSrc(t, "service.ts", src)
	if !hasRule(findings, "repeated-magic-literal") {
		t.Fatalf("expected repeated-magic-literal on plain .ts, got %#v", findings)
	}
	for _, f := range findings {
		if f.Analyzer != "js-quality" {
			t.Fatalf("analyzer = %q, want js-quality", f.Analyzer)
		}
	}
}

func TestCommonLiteralsNotFlagged(t *testing.T) {
	src := `const a = "";
const b = "";
const c = "";
const d = "";
const x = 1;
const y = 1;
const z = 1;`
	if findings := scanSrc(t, "common.ts", src); hasRule(findings, "repeated-magic-literal") {
		t.Fatalf("common literals must not be flagged, got %#v", findings)
	}
}

func TestImportLiteralsIgnored(t *testing.T) {
	src := `import { a } from "@scope/shared";
import { b } from "@scope/shared";
import { c } from "@scope/shared";
import { d } from "@scope/shared";`
	if findings := scanSrc(t, "imports.ts", src); hasRule(findings, "repeated-magic-literal") {
		t.Fatalf("import specifiers must not count as magic literals, got %#v", findings)
	}
}

func TestNonJSFilesSkipped(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "readme.md"), []byte("# hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	findings, err := New().Scan(core.ProjectContext{Root: root, Files: []string{"readme.md"}})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("non-JS file must yield no findings, got %#v", findings)
	}
}
