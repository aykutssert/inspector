package lang

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestDetectLanguageByExtension(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"foo.js", "javascript"},
		{"foo.jsx", "javascript"},
		{"foo.ts", "javascript"},
		{"foo.tsx", "javascript"},
		{"foo.mjs", "javascript"},
		{"foo.cjs", "javascript"},
		{"foo.mts", "javascript"},
		{"foo.cts", "javascript"},
		{"foo.svelte", "svelte"},
		{"foo.py", ""},
		{"foo.go", ""},
		{"foo.rs", ""},
	}
	for _, tt := range tests {
		if got := DetectLanguage(tt.path, ""); got != tt.want {
			t.Errorf("DetectLanguage(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestDetectLanguageByContent(t *testing.T) {
	dir := t.TempDir()
	path := "script"
	full := filepath.Join(dir, path)
	if err := os.WriteFile(full, []byte(`#!/usr/bin/env node
console.log("hello");
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := DetectLanguage(path, dir); got != "javascript" {
		t.Errorf("DetectLanguage(%q) by shebang = %q, want javascript", path, got)
	}
}

func TestDetectLanguagesGroups(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"a.js":       `const x = 1;`,
		"b.ts":       `const x: number = 1;`,
		"c.svelte":   `<script>let x;</script>`,
		"d.py":       `x = 1`,
		"e.go":       `package main`,
	}
	for rel, body := range files {
		path := filepath.Join(dir, rel)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	all := make([]string, 0, len(files))
	for rel := range files {
		all = append(all, rel)
	}

	groups := DetectLanguages(all, dir)
	if len(groups) != 2 {
		t.Fatalf("expected 2 language groups (javascript, svelte), got %d: %v", len(groups), keys(groups))
	}
	if len(groups["javascript"]) != 2 {
		t.Fatalf("expected 2 javascript files, got %d: %v", len(groups["javascript"]), groups["javascript"])
	}
	if len(groups["svelte"]) != 1 {
		t.Fatalf("expected 1 svelte file, got %d", len(groups["svelte"]))
	}
}

func TestRegistryDetectWithRoot(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.js"), []byte(`const x = 1;`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.py"), []byte(`x = 1`), 0o644); err != nil {
		t.Fatal(err)
	}

	files := []string{"a.js", "b.py"}
	reg := NewRegistry()
	langs := reg.Detect(files, dir)

	found := false
	for _, l := range langs {
		if l == "javascript" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected javascript in detected languages, got %v", langs)
	}
	for _, l := range langs {
		if l == "python" {
			t.Fatalf("unexpected python in detected languages: %v", langs)
		}
	}
}

func TestGroupByLanguage(t *testing.T) {
	dir := t.TempDir()
	files := []string{"a.js", "b.ts", "c.svelte"}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte(`x`), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	reg := NewRegistry()
	groups := reg.GroupByLanguage(files, dir)

	if len(groups["javascript"]) != 2 {
		t.Fatalf("expected 2 javascript files, got %d: %v", len(groups["javascript"]), groups["javascript"])
	}
	if len(groups["svelte"]) != 1 {
		t.Fatalf("expected 1 svelte file, got %d", len(groups["svelte"]))
	}
}

func keys(m map[string][]string) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
