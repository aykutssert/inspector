// Package jsproject derives framework signals (React, Next.js) from a scan
// context — the scanned files' extensions plus the dependencies declared in the
// relevant package.json files. It is the single source of truth so every
// analyzer gates on the same signal: oxlint enables its React/Next plugins, and
// the React hint pack only runs, on exactly the same definition of "this is a
// React/Next app".
package jsproject

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/aykutssert/scout/internal/core"
)

// IsReact reports whether the scan target is a React app: any scanned .jsx/.tsx
// file, or a "react" dependency in any relevant package.json.
func IsReact(ctx core.ProjectContext) bool {
	for _, f := range ctx.Files {
		switch strings.ToLower(filepath.Ext(f)) {
		case ".jsx", ".tsx":
			return true
		}
	}
	for dir := range RelevantPkgDirs(ctx) {
		if PkgHasDep(filepath.Join(dir, "package.json"), "react") {
			return true
		}
	}
	return false
}

// IsNext reports whether the scan target is a Next.js app: a next.config.* among
// the scanned files, or a "next" dependency in any relevant package.json.
func IsNext(ctx core.ProjectContext) bool {
	for _, f := range ctx.Files {
		if strings.HasPrefix(filepath.Base(f), "next.config.") {
			return true
		}
	}
	for dir := range RelevantPkgDirs(ctx) {
		if PkgHasDep(filepath.Join(dir, "package.json"), "next") {
			return true
		}
	}
	return false
}

// IsVite reports whether the scan target is a Vite project: a vite.config.* among
// the scanned files, or a "vite" dependency in any relevant package.json.
func IsVite(ctx core.ProjectContext) bool {
	for _, f := range ctx.Files {
		if strings.HasPrefix(filepath.Base(f), "vite.config.") {
			return true
		}
	}
	for dir := range RelevantPkgDirs(ctx) {
		if PkgHasDep(filepath.Join(dir, "package.json"), "vite") {
			return true
		}
	}
	return false
}

// IsBun reports whether the scan target is a Bun project: a bun.lockb among the
// scanned files, or a "bun" / "bun-types" dependency, or an "@types/bun" dep in
// any relevant package.json. Used to gate Bun-specific rules (Bun.password,
// Bun.file, Bun.serve, Bun.write) so they do not fire on plain Node projects.
func IsBun(ctx core.ProjectContext) bool {
	for _, f := range ctx.Files {
		base := strings.ToLower(filepath.Base(f))
		if base == "bun.lockb" || base == "bunfig.toml" {
			return true
		}
	}
	for dir := range RelevantPkgDirs(ctx) {
		pkg := filepath.Join(dir, "package.json")
		if PkgHasDep(pkg, "bun") || PkgHasDep(pkg, "bun-types") || PkgHasDep(pkg, "@types/bun") {
			return true
		}
	}
	return false
}

// IsTestOrExampleFile reports whether a path is test, example, fixture, or story
// code. Quality/design smell analyzers (repeated literals, god-class, large
// function, component splitting) skip these: repeated literals and long bodies
// are idiomatic in test fixtures and example snippets, so flagging them is noise.
func IsTestOrExampleFile(path string) bool {
	p := strings.ToLower(filepath.ToSlash(path))
	base := filepath.Base(p)
	for _, marker := range []string{".test.", ".spec.", ".bench.", ".stories.", ".story.", ".e2e.", ".cy."} {
		if strings.Contains(base, marker) {
			return true
		}
	}
	segs := strings.Split(p, "/")
	if len(segs) > 0 {
		segs = segs[:len(segs)-1] // directory segments only
	}
	for _, s := range segs {
		switch s {
		case "__tests__", "__mocks__", "__fixtures__", "test", "tests", "spec",
			"e2e", "examples", "example", "fixtures", "stories", ".storybook", "cypress":
			return true
		}
	}
	return false
}

// PkgHasDep reports whether the package.json at path lists dep among its
// dependencies or devDependencies.
func PkgHasDep(path, dep string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if json.Unmarshal(data, &pkg) != nil {
		return false
	}
	if _, ok := pkg.Dependencies[dep]; ok {
		return true
	}
	_, ok := pkg.DevDependencies[dep]
	return ok
}

// RelevantPkgDirs returns the repo root plus every directory on the path from
// each scanned file up to the root — the package.json locations a workspace
// dependency could be declared in.
func RelevantPkgDirs(ctx core.ProjectContext) map[string]bool {
	dirs := map[string]bool{ctx.Root: true}
	for _, f := range ctx.Files {
		dir := filepath.Dir(f)
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(ctx.Root, dir)
		}
		for {
			dirs[dir] = true
			if dir == ctx.Root || !strings.HasPrefix(dir, ctx.Root) {
				break
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return dirs
}
