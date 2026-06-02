package codegraph

import (
	"os"
	"path/filepath"
	"testing"
)

// writeProject lays out files (relative path -> content) under a temp dir and
// returns the root plus the relative file list.
func writeProject(t *testing.T, files map[string]string) (string, []string) {
	t.Helper()
	root := t.TempDir()
	var rels []string
	for rel, content := range files {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		rels = append(rels, rel)
	}
	return root, rels
}

func TestResolveImportRelativeAndIndex(t *testing.T) {
	root, files := writeProject(t, map[string]string{
		"index.js":          `const a = require('./lib/a'); const u = require('./lib/util')`,
		"lib/a.js":          `module.exports = {}`,
		"lib/util/index.js": `module.exports = {}`,
		"test/x.js":         `const e = require('../')`,
	})
	g := Build(root, files)

	cases := map[string][]string{
		"index.js":  {"lib/a.js", "lib/util/index.js"}, // ./lib/a -> a.js, ./lib/util -> util/index.js
		"test/x.js": {"index.js"},                      // ../ -> root index.js (directory import)
	}
	for file, want := range cases {
		got := g.Imports(file)
		if len(got) != len(want) {
			t.Errorf("%s imports = %v, want %v", file, got, want)
			continue
		}
		set := map[string]bool{}
		for _, w := range want {
			set[w] = true
		}
		for _, gimp := range got {
			if !set[gimp] {
				t.Errorf("%s resolved unexpected import %q (want %v)", file, gimp, want)
			}
		}
	}
}

// Two files define handler(). Callers must be attributed to the definition
// they can reach via imports, not merged across both.
func TestCallerDisambiguationByImport(t *testing.T) {
	root, files := writeProject(t, map[string]string{
		"a.js":     `function handler() {}; module.exports = handler`,
		"b.js":     `function handler() {}; module.exports = handler`,
		"usesA.js": `const handler = require('./a'); function go() { handler() }`,
		"usesB.js": `const handler = require('./b'); function go() { handler() }`,
	})
	g := Build(root, files)

	ctx := g.GetContext("handler")
	if len(ctx.Definitions) != 2 {
		t.Fatalf("expected 2 definitions of handler, got %d", len(ctx.Definitions))
	}
	for _, d := range ctx.Definitions {
		var callerFiles []string
		for _, c := range d.Callers {
			callerFiles = append(callerFiles, c.File)
		}
		switch d.File {
		case "a.js":
			if len(callerFiles) != 1 || callerFiles[0] != "usesA.js" {
				t.Errorf("a.js handler callers = %v, want [usesA.js]", callerFiles)
			}
		case "b.js":
			if len(callerFiles) != 1 || callerFiles[0] != "usesB.js" {
				t.Errorf("b.js handler callers = %v, want [usesB.js]", callerFiles)
			}
		default:
			t.Errorf("unexpected def file %s", d.File)
		}
	}
}
