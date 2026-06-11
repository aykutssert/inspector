package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMapUsesRegisteredFileParsers(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"dependencies":{"express":"5.0.0"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "server.ts"), []byte(`import express from "express"; const app = express(); app.listen(3000);`), 0o644); err != nil {
		t.Fatal(err)
	}

	repo, err := Map(MapOptions{Root: root}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if repo.Framework != "express" {
		t.Fatalf("framework = %q, want express", repo.Framework)
	}
	if len(repo.EntryPoints) != 1 || repo.EntryPoints[0] != "server.ts" {
		t.Fatalf("entry points = %v, want server.ts", repo.EntryPoints)
	}
}
