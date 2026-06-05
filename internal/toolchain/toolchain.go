// Package toolchain locates managed Node lint toolchains that scout ships
// under _toolchains/<name> (the leading underscore keeps the bundled node_modules
// out of Go's `./...` package walk). Each toolchain is a directory with its own
// package.json
// and an installed node_modules; scout wraps the proven linter inside it.
package toolchain

import (
	"os"
	"path/filepath"
)

// Dir returns the absolute path of the managed toolchain named name (e.g.
// "svelte", "typescript") when its eslint binary is installed, and ok=false
// otherwise. Lookup order: $SCOUT_HOME, the running executable's directory,
// then the current working directory (dev checkout).
func Dir(name string) (string, bool) {
	if bin, ok := Bin(name, "eslint"); ok {
		// bin = <dir>/node_modules/.bin/eslint → climb back to <dir>.
		return filepath.Dir(filepath.Dir(filepath.Dir(bin))), true
	}
	return "", false
}

// Bin returns the absolute path of an installed binary inside the managed
// toolchain named name (e.g. Bin("knip", "knip") →
// _toolchains/knip/node_modules/.bin/knip), and ok=false when it is not
// installed. Lookup order matches Dir: $SCOUT_HOME, the running
// executable's directory, then the current working directory.
func Bin(name, bin string) (string, bool) {
	for _, b := range bases() {
		p := filepath.Join(b, "_toolchains", name, "node_modules", ".bin", bin)
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
	}
	return "", false
}

func bases() []string {
	var bases []string
	if home := os.Getenv("SCOUT_HOME"); home != "" {
		bases = append(bases, home)
	}
	if exe, err := os.Executable(); err == nil {
		bases = append(bases, filepath.Dir(exe))
	}
	if wd, err := os.Getwd(); err == nil {
		bases = append(bases, wd)
	}
	return bases
}
