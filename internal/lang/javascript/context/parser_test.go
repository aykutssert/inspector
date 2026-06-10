package jscontext

import (
	"os"
	"path/filepath"
	"testing"
)

func TestJSParserParsesJSFile(t *testing.T) {
	dir := t.TempDir()
	path := "hello.ts"
	content := `
import { useState } from "react";
export function hello(name: string) { return "hi " + name; }
`
	if err := os.WriteFile(filepath.Join(dir, path), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	p := JSParser{}
	fp, err := p.Parse(dir, path)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if fp == nil {
		t.Fatal("Parse returned nil")
	}
	if fp.Path != path {
		t.Fatalf("want path %q, got %q", path, fp.Path)
	}
	if len(fp.Imports) != 1 {
		t.Fatalf("expected 1 import, got %d", len(fp.Imports))
	}
	if len(fp.Defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(fp.Defs))
	}
	if fp.Defs[0].Name != "hello" {
		t.Fatalf("want def name hello, got %q", fp.Defs[0].Name)
	}
	if !fp.Defs[0].Exported {
		t.Fatal("expected exported def")
	}
}
