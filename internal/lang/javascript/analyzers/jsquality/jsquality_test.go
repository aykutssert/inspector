package jsquality

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

func TestGodClassComplexity(t *testing.T) {
	methods := ""
	for i := 0; i < 12; i++ {
		methods += "  method" + strconv.Itoa(i) + "() {}\n"
	}
	src := `class ComplexController {
  constructor(
    private a: any,
    private b: any,
    private c: any,
    private d: any,
    private e: any,
    private f: any,
    private g: any,
    private h: any,
    private i: any
  ) {}
` + methods + strings.Repeat("  let x = 1;\n", 200) + `}`

	findings := scanSrc(t, "controller.ts", src)
	if !hasRule(findings, "god-class") {
		t.Fatalf("expected god-class violation, got %#v", findings)
	}
}

func TestLargeFunctionComplexity(t *testing.T) {
	calls := ""
	for i := 0; i < 7; i++ {
		calls += "  call" + strconv.Itoa(i) + "();\n"
	}
	src := `function complexFunc(a, b, c, d, e, f) {
` + calls + strings.Repeat("  let x = 1;\n", 120) + `}`
	findings := scanSrc(t, "func.js", src)
	if !hasRule(findings, "large-function") {
		t.Fatalf("expected large-function violation, got %#v", findings)
	}
}

func TestNonNullAssertionSpam(t *testing.T) {
	// 11 non-null assertions -> exceeds threshold of 10
	src := `
		const a = obj!.prop;
		const b = obj!.prop;
		const c = obj!.prop;
		const d = obj!.prop;
		const e = obj!.prop;
		const f = obj!.prop;
		const g = obj!.prop;
		const h = obj!.prop;
		const i = obj!.prop;
		const j = obj!.prop;
		const k = obj!.prop;
	`
	findings := scanSrc(t, "spam.ts", src)
	if !hasRule(findings, "non-null-assertion-spam") {
		t.Fatalf("expected non-null-assertion-spam violation, got %#v", findings)
	}
	// Check the violation info
	for _, f := range findings {
		if f.RuleID == "non-null-assertion-spam" {
			if f.Line != 2 {
				t.Fatalf("expected violation line to be first occurrence (2), got %d", f.Line)
			}
			if f.Severity != core.SeverityWarning {
				t.Fatalf("expected warning severity, got %v", f.Severity)
			}
		}
	}
}

func TestNonNullAssertionSpamSafe(t *testing.T) {
	// 10 non-null assertions -> exactly at/under threshold of 10, should be safe
	src := `
		const a = obj!.prop;
		const b = obj!.prop;
		const c = obj!.prop;
		const d = obj!.prop;
		const e = obj!.prop;
		const f = obj!.prop;
		const g = obj!.prop;
		const h = obj!.prop;
		const i = obj!.prop;
		const j = obj!.prop;
	`
	findings := scanSrc(t, "safe.ts", src)
	if hasRule(findings, "non-null-assertion-spam") {
		t.Fatalf("did not expect non-null-assertion-spam violation on 10 assertions, got %#v", findings)
	}
}

func TestSequentialAwaits(t *testing.T) {
	src := `
		async function test() {
			const a = await foo();
			const b = await bar(); // Violation: independent
		}
	`
	findings := scanSrc(t, "awaits.ts", src)
	if !hasRule(findings, "sequential-awaits-independent") {
		t.Fatalf("expected sequential-awaits-independent, got %#v", findings)
	}
}

func TestSequentialAwaitsDependent(t *testing.T) {
	src := `
		async function test() {
			const a = await foo();
			const b = await bar(a); // Safe: depends on a
		}
	`
	findings := scanSrc(t, "awaits_dep.ts", src)
	if hasRule(findings, "sequential-awaits-independent") {
		t.Fatalf("did not expect sequential-awaits-independent, got %#v", findings)
	}
}

func TestSequentialAwaitsNonConsecutive(t *testing.T) {
	src := `
		async function test() {
			const a = await foo();
			console.log("something in between");
			const b = await bar(); // Safe: non-consecutive
		}
	`
	findings := scanSrc(t, "awaits_non_consec.ts", src)
	if hasRule(findings, "sequential-awaits-independent") {
		t.Fatalf("did not expect sequential-awaits-independent, got %#v", findings)
	}
}
