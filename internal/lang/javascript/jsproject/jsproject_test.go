package jsproject

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aykutssert/inspector/internal/core"
)

func writePkg(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestIsTestOrExampleFile(t *testing.T) {
	positives := []string{
		"src/foo.test.ts", "src/foo.spec.tsx", "src/bar.bench.ts",
		"src/Button.stories.tsx", "packages/x/src/__tests__/a.ts",
		"test/util.js", "tests/util.js", "e2e/flow.ts", "cypress/run.ts",
		"examples/demo/app.tsx", "example/app.tsx", "src/__mocks__/db.ts",
		"src/__fixtures__/data.ts", ".storybook/main.ts",
	}
	for _, p := range positives {
		if !IsTestOrExampleFile(p) {
			t.Errorf("expected %q to be test/example", p)
		}
	}
	negatives := []string{
		"src/index.ts", "src/queryClient.ts", "packages/x/src/useQuery.tsx",
		"lib/contest.ts", "src/latest.ts", // substrings 'test'/'est' must not trigger
		"src/protester.ts",
	}
	for _, p := range negatives {
		if IsTestOrExampleFile(p) {
			t.Errorf("expected %q to NOT be test/example", p)
		}
	}
}

func TestIsBun(t *testing.T) {
	dir := t.TempDir()
	writePkg(t, dir, `{"dependencies":{"bun-types":"^1.0.0"}}`)
	if !IsBun(core.ProjectContext{Root: dir, Files: []string{"index.ts"}}) {
		t.Error("expected bun project by bun-types dep")
	}
	dir2 := t.TempDir()
	if IsBun(core.ProjectContext{Root: dir2, Files: []string{"bun.lockb"}}) != true {
		t.Error("expected bun project by bun.lockb")
	}
	dir3 := t.TempDir()
	writePkg(t, dir3, `{"dependencies":{"express":"^4.0.0"}}`)
	if IsBun(core.ProjectContext{Root: dir3, Files: []string{"index.ts"}}) {
		t.Error("expected non-bun project")
	}
}

func TestIsReactByFileExtension(t *testing.T) {
	ctx := core.ProjectContext{Root: t.TempDir(), Files: []string{"src/App.tsx"}}
	if !IsReact(ctx) {
		t.Fatal("a .tsx file should signal a React project")
	}
}

func TestIsReactByDependency(t *testing.T) {
	dir := t.TempDir()
	writePkg(t, dir, `{"dependencies":{"react":"^18.0.0"}}`)
	if !IsReact(core.ProjectContext{Root: dir, Files: []string{"src/index.ts"}}) {
		t.Fatal("a react dependency should signal a React project")
	}
}

func TestIsReactNegative(t *testing.T) {
	dir := t.TempDir()
	writePkg(t, dir, `{"dependencies":{"express":"^4.0.0"}}`)
	if IsReact(core.ProjectContext{Root: dir, Files: []string{"src/server.ts"}}) {
		t.Fatal("a plain Node project must not be flagged as React")
	}
}

func TestIsReactByWorkspaceDependency(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "apps", "web")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	writePkg(t, app, `{"dependencies":{"react":"^18.0.0"}}`)
	ctx := core.ProjectContext{Root: root, Files: []string{"apps/web/src/index.ts"}}
	if !IsReact(ctx) {
		t.Fatal("react declared in a sub-package should be detected via the walk-up")
	}
}

func TestIsNextByConfigAndDependency(t *testing.T) {
	if !IsNext(core.ProjectContext{Root: t.TempDir(), Files: []string{"next.config.mjs"}}) {
		t.Fatal("next.config.* should signal a Next.js project")
	}
	dir := t.TempDir()
	writePkg(t, dir, `{"dependencies":{"next":"^14.0.0"}}`)
	if !IsNext(core.ProjectContext{Root: dir, Files: []string{"app/page.ts"}}) {
		t.Fatal("a next dependency should signal a Next.js project")
	}
}

func TestIsNextNegative(t *testing.T) {
	dir := t.TempDir()
	writePkg(t, dir, `{"dependencies":{"react":"^18.0.0"}}`)
	if IsNext(core.ProjectContext{Root: dir, Files: []string{"src/App.tsx"}}) {
		t.Fatal("a React-only project must not be flagged as Next.js")
	}
}
