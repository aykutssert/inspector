// Package toolchain locates managed Node lint toolchains that inspector ships
// under _toolchains/<name> (the leading underscore keeps the bundled node_modules
// out of Go's `./...` package walk). Each toolchain is a directory with its own
// package.json
// and an installed node_modules; inspector wraps the proven linter inside it.
package toolchain

import (
	"os"
	"path/filepath"
)

// Dir returns the absolute path of the managed toolchain named name (e.g.
// "svelte", "typescript") when its eslint binary is installed, and ok=false
// otherwise. Lookup order: $INSPECTOR_HOME, the running executable's directory,
// then the current working directory (dev checkout).
func Dir(name string) (string, bool) {
	var bases []string
	if home := os.Getenv("INSPECTOR_HOME"); home != "" {
		bases = append(bases, home)
	}
	if exe, err := os.Executable(); err == nil {
		bases = append(bases, filepath.Dir(exe))
	}
	if wd, err := os.Getwd(); err == nil {
		bases = append(bases, wd)
	}
	for _, b := range bases {
		dir := filepath.Join(b, "_toolchains", name)
		if _, err := os.Stat(filepath.Join(dir, "node_modules", ".bin", "eslint")); err == nil {
			return dir, true
		}
	}
	return "", false
}
