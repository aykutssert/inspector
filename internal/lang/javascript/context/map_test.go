package jscontext

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFixture creates a temp dir with the given files and returns the abs root.
func writeFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		abs := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func buildMapFixture(t *testing.T, files map[string]string) interface{} {
	t.Helper()
	root := writeFixture(t, files)
	var rels []string
	for rel := range files {
		rels = append(rels, rel)
	}
	return Build(root, rels).BuildMap()
}

// ─── language detection ───────────────────────────────────────────────────────

func TestDetectLanguage_TypeScript(t *testing.T) {
	root := writeFixture(t, map[string]string{
		"index.ts": `export function hello() {}`,
	})
	g := Build(root, []string{"index.ts"})
	if lang := g.detectLanguage(); lang != "typescript" {
		t.Fatalf("want typescript, got %q", lang)
	}
}

func TestDetectLanguage_JavaScript(t *testing.T) {
	root := writeFixture(t, map[string]string{
		"index.js": `module.exports = function hello() {}`,
	})
	g := Build(root, []string{"index.js"})
	if lang := g.detectLanguage(); lang != "javascript" {
		t.Fatalf("want javascript, got %q", lang)
	}
}

// ─── framework detection ──────────────────────────────────────────────────────

func TestDetectFramework_NextJS(t *testing.T) {
	root := writeFixture(t, map[string]string{
		"package.json": `{"dependencies":{"next":"14.2.0","react":"18.0.0"}}`,
		"index.ts":     `export {}`,
	})
	g := Build(root, []string{"index.ts"})
	name, ver := g.detectFramework()
	if name != "nextjs" {
		t.Fatalf("want nextjs, got %q", name)
	}
	if ver != "14.2.0" {
		t.Fatalf("want 14.2.0, got %q", ver)
	}
}

func TestDetectFramework_ReactNativeBeforeReact(t *testing.T) {
	// react-native must win over react when both are present.
	root := writeFixture(t, map[string]string{
		"package.json": `{"dependencies":{"react-native":"0.73","react":"18.0"}}`,
		"index.ts":     `export {}`,
	})
	g := Build(root, []string{"index.ts"})
	name, _ := g.detectFramework()
	if name != "react-native" {
		t.Fatalf("want react-native, got %q", name)
	}
}

func TestDetectFramework_NoPkgJSON(t *testing.T) {
	root := writeFixture(t, map[string]string{
		"index.ts": `export {}`,
	})
	g := Build(root, []string{"index.ts"})
	name, ver := g.detectFramework()
	if name != "" || ver != "" {
		t.Fatalf("want empty, got %q %q", name, ver)
	}
}

// ─── external deps ────────────────────────────────────────────────────────────

func TestExternalDeps_FiltersRelative(t *testing.T) {
	root := writeFixture(t, map[string]string{
		"a.ts": `import './b'; import 'react'; import '@/util';`,
		"b.ts": `export const x = 1;`,
	})
	g := Build(root, []string{"a.ts", "b.ts"})
	deps := g.externalDeps("a.ts")
	for _, d := range deps {
		if d == "b.ts" || d == "@/util" {
			t.Fatalf("internal import leaked into deps: %q (all deps: %v)", d, deps)
		}
	}
	found := false
	for _, d := range deps {
		if d == "react" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected react in deps, got %v", deps)
	}
}

func TestExternalDeps_ScopedPackage(t *testing.T) {
	root := writeFixture(t, map[string]string{
		"a.ts": `import '@nestjs/core/decorators'; import '@nestjs/common';`,
	})
	g := Build(root, []string{"a.ts"})
	deps := g.externalDeps("a.ts")
	for _, d := range deps {
		if d == "@nestjs/core/decorators" {
			t.Fatalf("subpath leaked: %q", d)
		}
	}
	found := false
	for _, d := range deps {
		if d == "@nestjs/core" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected @nestjs/core in deps, got %v", deps)
	}
}

// ─── entry points ─────────────────────────────────────────────────────────────

func TestNextjsEntryPoints(t *testing.T) {
	root := writeFixture(t, map[string]string{
		"package.json":                `{"dependencies":{"next":"14"}}`,
		"app/page.tsx":                `export default function Page() { return null }`,
		"app/layout.tsx":              `export default function Layout() { return null }`,
		"app/api/users/route.ts":      `export async function GET() {}`,
		"middleware.ts":               `export function middleware() {}`,
		"lib/utils.ts":                `export function cn() {}`,
	})
	rels := []string{"app/page.tsx", "app/layout.tsx", "app/api/users/route.ts", "middleware.ts", "lib/utils.ts"}
	g := Build(root, rels)
	eps := g.detectEntryPoints("nextjs")
	epSet := make(map[string]bool)
	for _, e := range eps {
		epSet[e] = true
	}
	for _, want := range []string{"app/page.tsx", "app/layout.tsx", "app/api/users/route.ts", "middleware.ts"} {
		if !epSet[want] {
			t.Errorf("missing entry point %q (got %v)", want, eps)
		}
	}
	if epSet["lib/utils.ts"] {
		t.Errorf("lib/utils.ts should not be an entry point (got %v)", eps)
	}
}

// ─── groupIntoDirs ────────────────────────────────────────────────────────────

func TestGroupIntoDirs_SortsByMaxImportedBy(t *testing.T) {
	root := writeFixture(t, map[string]string{
		"lib/db.ts":        `export function query() {}`,
		"lib/auth.ts":      `export function getSession() {}`,
		"components/Btn.ts": `export function Button() {}`,
	})
	g := Build(root, []string{"lib/db.ts", "lib/auth.ts", "components/Btn.ts"})
	// Manually inject importers to simulate graph.
	g.importers["lib/db.ts"] = make([]string, 10)
	g.importers["components/Btn.ts"] = make([]string, 5)

	nodes := g.buildFileNodes()
	dirs := groupIntoDirs(nodes)
	if len(dirs) == 0 {
		t.Fatal("expected dirs, got none")
	}
	// lib/ has a file with 10 importers — should come first.
	if dirs[0].Path != "lib" {
		t.Fatalf("want lib first (max importers), got %q", dirs[0].Path)
	}
}

// ─── topByImportedBy ──────────────────────────────────────────────────────────

func TestTopByImportedBy_ExcludesZero(t *testing.T) {
	root := writeFixture(t, map[string]string{
		"a.ts": `export function a() {}`,
		"b.ts": `export function b() {}`,
	})
	g := Build(root, []string{"a.ts", "b.ts"})
	g.importers["a.ts"] = []string{"x.ts"}

	nodes := g.buildFileNodes()
	top := topByImportedBy(nodes, 5)
	for _, n := range top {
		if n.ImportedBy == 0 {
			t.Fatalf("zero-importer file %q leaked into hot files", n.Path)
		}
	}
}

// ─── BuildMap smoke test ───────────────────────────────────────────────────────

func TestBuildMap_NextJS(t *testing.T) {
	root := writeFixture(t, map[string]string{
		"package.json":   `{"dependencies":{"next":"14.1.0"}}`,
		"app/page.tsx":   `export default function Page() { return null }`,
		"lib/db.ts":      `export function query(sql: string): Promise<Row[]> {}`,
	})
	g := Build(root, []string{"app/page.tsx", "lib/db.ts"})
	g.importers["lib/db.ts"] = []string{"app/page.tsx"}

	m := g.BuildMap()
	if m.Framework != "nextjs" {
		t.Errorf("want nextjs, got %q", m.Framework)
	}
	if m.FrameworkVer != "14.1.0" {
		t.Errorf("want 14.1.0, got %q", m.FrameworkVer)
	}
	if m.Language != "typescript" {
		t.Errorf("want typescript, got %q", m.Language)
	}
	if len(m.Dirs) == 0 {
		t.Error("expected dirs, got none")
	}
}
