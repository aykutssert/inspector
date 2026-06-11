package context

import (
	"os"
	"path/filepath"
	"testing"
)

type fixtureParser struct {
	files map[string]*FileParse
}

func (p fixtureParser) Parse(_ string, path string) (*FileParse, error) {
	return p.files[path], nil
}

func TestBuildRepoMapFromFileParser(t *testing.T) {
	root := t.TempDir()
	writeContextFixture(t, root, "a.ts", "import { value } from './b'\nimport React from 'react'\nexport function run() { return value }\n")
	writeContextFixture(t, root, "b.ts", "export const value = 1\n")

	parser := fixtureParser{files: map[string]*FileParse{
		"a.ts": {
			Language: "typescript",
			Imports: []Import{
				{Source: "./b", Target: "b.ts"},
				{Source: "react", Package: "react"},
			},
			Defs: []Def{{Name: "run", Kind: "function", Line: 3, EndLine: 3, Exported: true}},
		},
		"b.ts": {
			Language: "typescript",
			Defs:     []Def{{Name: "value", Kind: "const", Line: 1, EndLine: 1, Exported: true}},
		},
	}}

	repo, err := BuildRepoMap(root, []string{"a.ts", "b.ts"}, parser)
	if err != nil {
		t.Fatal(err)
	}
	if repo.Language != "typescript" {
		t.Fatalf("language = %q, want typescript", repo.Language)
	}
	if len(repo.HotFiles) != 1 || repo.HotFiles[0].Path != "b.ts" || repo.HotFiles[0].ImportedBy != 1 {
		t.Fatalf("hot files = %#v, want b.ts imported once", repo.HotFiles)
	}
	a := findFileNode(repo, "a.ts")
	if a == nil || len(a.Deps) != 1 || a.Deps[0] != "react" {
		t.Fatalf("a.ts deps = %#v, want react", a)
	}
	if len(a.Exports) != 1 || a.Exports[0].Name != "run" {
		t.Fatalf("a.ts exports = %#v, want run", a.Exports)
	}
}

func findFileNode(repo RepoMap, path string) *FileNode {
	for _, dir := range repo.Dirs {
		for i := range dir.Files {
			if dir.Files[i].Path == path {
				return &dir.Files[i]
			}
		}
	}
	return nil
}

func writeContextFixture(t *testing.T, root, path, body string) {
	t.Helper()
	absolute := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(absolute, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
