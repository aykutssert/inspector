package oxlint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aykutssert/inspector/internal/core"
)

func TestBuildConfigGatesNextjs(t *testing.T) {
	if strings.Contains(buildConfig(false), "nextjs") {
		t.Fatal("plain config must not enable the nextjs plugin")
	}
	if !strings.Contains(buildConfig(true), "nextjs") {
		t.Fatal("next config must enable the nextjs plugin")
	}
}

func TestIsNextProjectByConfigFile(t *testing.T) {
	dir := t.TempDir()
	ctx := core.ProjectContext{Root: dir, Files: []string{filepath.Join(dir, "next.config.mjs")}}
	if !isNextProject(ctx) {
		t.Fatal("next.config.mjs should mark a Next.js project")
	}
}

func TestIsNextProjectByDependency(t *testing.T) {
	dir := t.TempDir()
	pkg := `{"dependencies":{"next":"14.2.0","react":"18.0.0"}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0o644); err != nil {
		t.Fatal(err)
	}
	if !isNextProject(core.ProjectContext{Root: dir}) {
		t.Fatal("next dependency should mark a Next.js project")
	}
}

func TestIsNextProjectByDevDependency(t *testing.T) {
	dir := t.TempDir()
	pkg := `{"devDependencies":{"next":"14.2.0"}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0o644); err != nil {
		t.Fatal(err)
	}
	if !isNextProject(core.ProjectContext{Root: dir}) {
		t.Fatal("next devDependency should mark a Next.js project")
	}
}

func TestPlainReactNotNext(t *testing.T) {
	dir := t.TempDir()
	pkg := `{"dependencies":{"react":"18.0.0","react-dom":"18.0.0"}}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkg), 0o644); err != nil {
		t.Fatal(err)
	}
	if isNextProject(core.ProjectContext{Root: dir}) {
		t.Fatal("a plain React app must not be detected as Next.js")
	}
}

func TestNoPackageJSONNotNext(t *testing.T) {
	if isNextProject(core.ProjectContext{Root: t.TempDir()}) {
		t.Fatal("missing package.json must not be detected as Next.js")
	}
}
