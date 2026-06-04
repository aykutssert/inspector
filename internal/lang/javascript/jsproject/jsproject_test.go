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
