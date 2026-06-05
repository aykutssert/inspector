package importcycle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aykutssert/scout/internal/core"
)

// No JS/TS files → silent no-op (never builds a graph).
func TestScanNoJSFiles(t *testing.T) {
	got, err := New().Scan(core.ProjectContext{Root: t.TempDir(), Files: []string{"style.css", "data.json"}})
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestScanNoCycle(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a.ts", `import { b } from './b'; export const a = () => b();`)
	write(t, root, "b.ts", `export const b = () => 1;`)
	got, err := New().Scan(core.ProjectContext{Root: root, Files: []string{"a.ts", "b.ts"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected no findings, got %#v", got)
	}
}

func TestScanDirectCycle(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a.ts", `import { b } from './b'; export const a = () => b;`)
	write(t, root, "b.ts", `import { a } from './a'; export const b = () => a;`)
	got, err := New().Scan(core.ProjectContext{Root: root, Files: []string{"a.ts", "b.ts"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected one finding, got %#v", got)
	}
	f := got[0]
	if f.RuleID != "import-cycle" || f.Severity != core.SeverityWarning || f.Confidence != core.ConfidenceRule {
		t.Fatalf("bad finding shape: %#v", f)
	}
	if f.File != "a.ts" || !strings.Contains(f.Message, "a.ts -> b.ts -> a.ts") {
		t.Fatalf("bad cycle message: %q (file %q)", f.Message, f.File)
	}
}

func TestScanIndirectCycle(t *testing.T) {
	root := t.TempDir()
	write(t, root, "a.ts", `import { b } from './b'; export const a = 1;`)
	write(t, root, "b.ts", `import { c } from './c'; export const b = 2;`)
	write(t, root, "c.ts", `import { a } from './a'; export const c = 3;`)
	got, err := New().Scan(core.ProjectContext{Root: root, Files: []string{"a.ts", "b.ts", "c.ts"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected one finding, got %#v", got)
	}
	if !strings.Contains(got[0].Message, "a.ts -> b.ts -> c.ts -> a.ts") {
		t.Fatalf("bad 3-cycle message: %q", got[0].Message)
	}
}

func TestScanInternalBarrelImport(t *testing.T) {
	root := t.TempDir()
	write(t, root, "components/Button.ts", `import { Card } from ".";
export const Button = Card;`)
	write(t, root, "components/Card.ts", `export const Card = 1;`)
	write(t, root, "components/index.ts", `export { Card } from "./Card";`)

	got, err := New().Scan(core.ProjectContext{Root: root, Files: []string{
		"components/Button.ts",
		"components/Card.ts",
		"components/index.ts",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected one finding, got %#v", got)
	}
	f := got[0]
	if f.RuleID != "internal-barrel-import" || f.Severity != core.SeverityWarning || f.Confidence != core.ConfidenceRule {
		t.Fatalf("bad finding shape: %#v", f)
	}
	if f.File != "components/Button.ts" || f.Line != 1 {
		t.Fatalf("bad finding location: %#v", f)
	}
	if !strings.Contains(f.Message, "components/index.ts") {
		t.Fatalf("message should name local barrel target: %q", f.Message)
	}
}

func TestScanInternalBarrelImportViaExplicitIndex(t *testing.T) {
	root := t.TempDir()
	write(t, root, "components/Button.ts", `import { Card } from "./index";
export const Button = Card;`)
	write(t, root, "components/Card.ts", `export const Card = 1;`)
	write(t, root, "components/index.ts", `export { Card } from "./Card";`)

	got, err := New().Scan(core.ProjectContext{Root: root, Files: []string{
		"components/Button.ts",
		"components/Card.ts",
		"components/index.ts",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].RuleID != "internal-barrel-import" {
		t.Fatalf("expected explicit index barrel finding, got %#v", got)
	}
}

func TestScanExternalBarrelImportIsAllowed(t *testing.T) {
	root := t.TempDir()
	write(t, root, "pages/Home.ts", `import { Button } from "../components";
export const Home = Button;`)
	write(t, root, "components/Button.ts", `export const Button = 1;`)
	write(t, root, "components/index.ts", `export { Button } from "./Button";`)

	got, err := New().Scan(core.ProjectContext{Root: root, Files: []string{
		"pages/Home.ts",
		"components/Button.ts",
		"components/index.ts",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("external barrel import should be allowed, got %#v", got)
	}
}

func TestScanIndexFileImportingSiblingIsAllowed(t *testing.T) {
	root := t.TempDir()
	write(t, root, "components/Button.ts", `export const Button = 1;`)
	write(t, root, "components/index.ts", `import { Button } from "./Button";
export { Button };`)

	got, err := New().Scan(core.ProjectContext{Root: root, Files: []string{
		"components/Button.ts",
		"components/index.ts",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("index barrel importing siblings should be allowed, got %#v", got)
	}
}

func write(t *testing.T, root, name, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
