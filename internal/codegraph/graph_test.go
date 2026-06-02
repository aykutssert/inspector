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

// A caller that imports BOTH modules but binds handler from a.js must be
// attributed only to a.js's handler — the binding pins it, even though b.js is
// also reachable. Resolved must be true (binding-based, high confidence).
func TestBindingPinsCallerToBoundModule(t *testing.T) {
	root, files := writeProject(t, map[string]string{
		"a.js":    `function handler() {}; module.exports = handler`,
		"b.js":    `function handler() {}; module.exports = handler`,
		"uses.js": `const handler = require('./a'); const other = require('./b'); function go() { handler() }`,
	})
	g := Build(root, files)

	for _, d := range g.GetContext("handler").Definitions {
		switch d.File {
		case "a.js":
			if len(d.Callers) != 1 || d.Callers[0].File != "uses.js" {
				t.Errorf("a.js handler callers = %+v, want one from uses.js", d.Callers)
			} else if !d.Callers[0].Resolved {
				t.Errorf("a.js handler caller should be Resolved (binding-based)")
			}
		case "b.js":
			if len(d.Callers) != 0 {
				t.Errorf("b.js handler callers = %+v, want none (binding points to a.js)", d.Callers)
			}
		}
	}
}

// A method call on a dynamic receiver (res.send) has no import binding, so it
// falls back to the reachability heuristic and is reported unresolved.
func TestDynamicReceiverHeuristic(t *testing.T) {
	root, files := writeProject(t, map[string]string{
		"send.js":  `function send() {}; module.exports = send`,
		"route.js": `const send = require('./send'); function handle(res) { res.send() }`,
	})
	g := Build(root, files)

	var found bool
	for _, d := range g.GetContext("send").Definitions {
		if d.File != "send.js" {
			continue
		}
		for _, c := range d.Callers {
			if c.File == "route.js" {
				t.Errorf("res.send() should not be a resolved caller (no binding)")
			}
		}
		for _, c := range d.UnresolvedCallers {
			if c.File == "route.js" {
				found = true
				if c.Resolved {
					t.Errorf("res.send() caller should be heuristic (unresolved), got Resolved=true")
				}
			}
		}
	}
	if !found {
		t.Errorf("expected route.js among send unresolved callers via heuristic")
	}
}

// Aliased imports record calls under the local name, not the symbol. The graph
// must still attribute them to the definition via the binding's imported name
// (ES alias) or the target's default export (CommonJS require).
// JSX component usage (<Foo/>) must register as a caller of the component
// definition, resolved through the default import binding. HTML tags (<div>)
// are not components and must not be recorded.
func TestJSXComponentCallers(t *testing.T) {
	root, files := writeProject(t, map[string]string{
		"Foo.jsx": `export default function Foo() { return <span/> }`,
		"App.jsx": `import Foo from './Foo'; export default function App() { return <div><Foo/></div> }`,
	})
	g := Build(root, files)

	var found bool
	for _, d := range g.GetContext("Foo").Definitions {
		if d.File != "Foo.jsx" {
			continue
		}
		for _, c := range d.Callers {
			if c.File == "App.jsx" {
				found = true
				if !c.Resolved {
					t.Errorf("JSX <Foo/> caller should be resolved via default import")
				}
			}
		}
	}
	if !found {
		t.Errorf("expected App.jsx among Foo callers via JSX usage")
	}
	if len(g.Calls("div")) != 0 || len(g.Calls("span")) != 0 {
		t.Errorf("HTML tags must not be recorded as component calls")
	}
}

func TestAliasCallerResolution(t *testing.T) {
	root, files := writeProject(t, map[string]string{
		"a.js":       `export function handler() {}`,
		"d.js":       `function handler() {}; module.exports = handler`,
		"e.js":       `function handler() {}; module.exports = { handler }`,
		"esAlias.js": `import { handler as h } from './a'; function go() { h() }`,
		"cjsDef.js":  `const h = require('./d'); function go() { h() }`,
		"cjsPair.js": `const { handler: h } = require('./e'); function go() { h() }`,
	})
	g := Build(root, files)

	want := map[string]string{"esAlias.js": "a.js", "cjsDef.js": "d.js", "cjsPair.js": "e.js"}
	got := map[string]bool{}
	for _, d := range g.GetContext("handler").Definitions {
		for _, c := range d.Callers {
			if w, ok := want[c.File]; ok {
				if d.File != w {
					t.Errorf("%s aliased call attributed to %s, want %s", c.File, d.File, w)
				}
				if !c.Resolved {
					t.Errorf("%s aliased caller should be Resolved (binding-based)", c.File)
				}
				got[c.File] = true
			}
		}
	}
	for caller := range want {
		if !got[caller] {
			t.Errorf("missing aliased caller %s for handler", caller)
		}
	}
}
