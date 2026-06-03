package jscontext

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseJS(t *testing.T) {
	src := `
import express from 'express'
const lodash = require('./utils/lodash')

export function handler(req, res) {
  const x = helper(req)
  res.send(x)
}

class Router {
  dispatch() { return route() }
}

const arrow = (a) => doThing(a)
`
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.js")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	fp, err := ParseJS(path)
	if err != nil {
		t.Fatal(err)
	}

	wantImports := map[string]bool{"express": true, "./utils/lodash": true}
	for _, im := range fp.Imports {
		delete(wantImports, im.Source)
	}
	if len(wantImports) != 0 {
		t.Errorf("missing imports: %v (got %+v)", wantImports, fp.Imports)
	}

	defNames := map[string]Def{}
	for _, d := range fp.Defs {
		defNames[d.Name] = d
	}
	for _, want := range []string{"handler", "Router", "dispatch", "arrow"} {
		if _, ok := defNames[want]; !ok {
			t.Errorf("missing def %q (got %+v)", want, fp.Defs)
		}
	}
	if !defNames["handler"].Exported {
		t.Errorf("handler should be exported")
	}

	callNames := map[string]bool{}
	for _, c := range fp.Calls {
		callNames[c.Name] = true
	}
	for _, want := range []string{"helper", "send", "route", "doThing"} {
		if !callNames[want] {
			t.Errorf("missing call %q (got %+v)", want, fp.Calls)
		}
	}
}

// #4: a method call records its receiver so db.query() is not conflated with a
// bare query().
func TestParseCallReceiver(t *testing.T) {
	fp := parseSnippet(t, "calls.js", `
function go() {
  query()
  db.query()
}
`)
	var bare, method *Call
	for i := range fp.Calls {
		switch {
		case fp.Calls[i].Name == "query" && fp.Calls[i].Recv == "":
			bare = &fp.Calls[i]
		case fp.Calls[i].Name == "query" && fp.Calls[i].Recv == "db":
			method = &fp.Calls[i]
		}
	}
	if bare == nil {
		t.Errorf("missing bare query() call (got %+v)", fp.Calls)
	}
	if method == nil {
		t.Errorf("missing db.query() call with recv=db (got %+v)", fp.Calls)
	}
}

// #8: defs exposed via CommonJS and ES export variants are flagged exported.
func TestParseExportVariants(t *testing.T) {
	cases := map[string]string{
		"cjs_assign.js":  "function a() {}\nmodule.exports = a",
		"cjs_object.js":  "function b() {}\nmodule.exports = { b }",
		"cjs_dot.js":     "function c() {}\nexports.c = c",
		"es_clause.js":   "function d() {}\nexport { d }",
		"es_default.js":  "function e() {}\nexport default e",
		"cjs_nested.js":  "function f() {}\nmodule.exports.f = f",
		"cjs_pairval.js": "function internalG() {}\nmodule.exports = { publicG: internalG }",
	}
	want := map[string]string{
		"cjs_assign.js": "a", "cjs_object.js": "b", "cjs_dot.js": "c",
		"es_clause.js": "d", "es_default.js": "e",
		"cjs_nested.js": "f", "cjs_pairval.js": "internalG",
	}
	for file, src := range cases {
		fp := parseSnippet(t, file, src)
		name := want[file]
		var found bool
		for _, d := range fp.Defs {
			if d.Name == name {
				found = true
				if !d.Exported {
					t.Errorf("%s: def %q should be exported (got %+v)", file, name, fp.Defs)
				}
			}
		}
		if !found {
			t.Errorf("%s: missing def %q (got %+v)", file, name, fp.Defs)
		}
	}
}

func parseSnippet(t *testing.T, name, src string) *FileParse {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	fp, err := ParseJS(path)
	if err != nil {
		t.Fatal(err)
	}
	return fp
}
