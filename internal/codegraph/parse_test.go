package codegraph

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
