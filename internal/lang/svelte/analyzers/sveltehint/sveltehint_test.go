package sveltehint

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aykutssert/inspector/internal/core"
)

func TestGlobalDOMQueriesAreReported(t *testing.T) {
	src := `<script>
  document.getElementById("app");
  document.querySelector(".card");
  document.querySelectorAll("button");
  document.getElementsByClassName("item");
  document.getElementsByTagName("li");
</script>

<main>ok</main>
`
	findings := scanSource(t, "Component.svelte", src)
	if len(findings) != 5 {
		t.Fatalf("findings len = %d, want 5: %#v", len(findings), findings)
	}
	wantLines := []int{2, 3, 4, 5, 6}
	for i, f := range findings {
		if f.RuleID != "svelte.global-dom-query" {
			t.Fatalf("rule id = %q", f.RuleID)
		}
		if f.Severity != core.SeverityWarning || f.Category != "quality" || f.Confidence != core.ConfidenceHint {
			t.Fatalf("bad finding shape: %#v", f)
		}
		if f.Line != wantLines[i] {
			t.Fatalf("line %d = %d, want %d (%#v)", i, f.Line, wantLines[i], f)
		}
	}
}

func TestGlobalDOMQueriesIgnoreStringsCommentsMarkupAndElementQueries(t *testing.T) {
	src := `<p>document.querySelector(".from-markup")</p>
<script>
  // document.querySelector(".from-comment")
  const text = "document.querySelector('.from-string')";
  const root = document.createElement("div");
  root.querySelector(".local");
  window.document.querySelector(".window-document");
</script>
`
	if findings := scanSource(t, "Safe.svelte", src); len(findings) != 0 {
		t.Fatalf("expected no findings, got %#v", findings)
	}
}

func TestTypeScriptScriptIsParsed(t *testing.T) {
	src := `<script lang="ts">
  const selector: string = ".card";
  document.querySelector(selector);
</script>
`
	findings := scanSource(t, "Typed.svelte", src)
	if len(findings) != 1 {
		t.Fatalf("findings len = %d, want 1: %#v", len(findings), findings)
	}
	if findings[0].Line != 3 {
		t.Fatalf("line = %d, want 3", findings[0].Line)
	}
}

func TestLineNumbersAccountForMarkupBeforeScript(t *testing.T) {
	src := `<h1>Title</h1>
<p>Intro</p>

<script>
  document.querySelector(".card");
</script>
`
	findings := scanSource(t, "Offset.svelte", src)
	if len(findings) != 1 {
		t.Fatalf("findings len = %d, want 1: %#v", len(findings), findings)
	}
	if findings[0].Line != 5 {
		t.Fatalf("line = %d, want 5", findings[0].Line)
	}
}

func TestExternalScriptIsSkipped(t *testing.T) {
	src := `<script
  src="./external.js"
></script>
<script>
  document.querySelector(".local");
</script>
`
	findings := scanSource(t, "External.svelte", src)
	if len(findings) != 1 {
		t.Fatalf("findings len = %d, want 1: %#v", len(findings), findings)
	}
	if findings[0].Line != 5 {
		t.Fatalf("line = %d, want 5", findings[0].Line)
	}
}

func TestScanNoSvelteFiles(t *testing.T) {
	got, err := New().Scan(core.ProjectContext{Root: t.TempDir(), Files: []string{"a.ts", "b.jsx"}})
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %#v", got)
	}
}

func scanSource(t *testing.T, name, src string) []core.Finding {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := New().Scan(core.ProjectContext{Root: root, Files: []string{name}})
	if err != nil {
		t.Fatal(err)
	}
	return got
}
