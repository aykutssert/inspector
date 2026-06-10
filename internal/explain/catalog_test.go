package explain_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aykutssert/scout/internal/explain"
)

const fixture = `
- id: test.rule-one
  why: this is bad
  bad: |
    doThing()
  good: |
    doThing({ safe: true })
  fix: pass safe option

- id: test.rule-two
  why: also bad
  bad: old()
  good: new()
  fix: use new
`

func writeTmp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "catalog.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad(t *testing.T) {
	path := writeTmp(t, fixture)
	c, err := explain.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	e, ok := c.Lookup("test.rule-one")
	if !ok {
		t.Fatal("expected test.rule-one to be found")
	}
	if e.Why != "this is bad" {
		t.Errorf("Why = %q, want %q", e.Why, "this is bad")
	}
	if e.Fix != "pass safe option" {
		t.Errorf("Fix = %q, want %q", e.Fix, "pass safe option")
	}
}

func TestLookupMissing(t *testing.T) {
	path := writeTmp(t, fixture)
	c, err := explain.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	_, ok := c.Lookup("nonexistent.rule")
	if ok {
		t.Fatal("expected nonexistent rule to be missing")
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := explain.Load("/nonexistent/catalog.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
