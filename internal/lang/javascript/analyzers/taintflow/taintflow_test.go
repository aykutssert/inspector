package taintflow

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/aykutssert/scout/internal/core"
)

func scanSource(t *testing.T, name, src string) []core.Finding {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	findings, err := scanFile(path, name)
	if err != nil {
		t.Fatal(err)
	}
	return findings
}

func ruleIDs(findings []core.Finding) []string {
	ids := make([]string, 0, len(findings))
	for _, f := range findings {
		ids = append(ids, f.RuleID)
	}
	sort.Strings(ids)
	return ids
}

func TestNosqlQueryTaintedVar(t *testing.T) {
	src := `
function handler(req, res) {
  const filter = req.body;
  User.find(filter);
}
`
	findings := scanSource(t, "handler.js", src)
	if got := ruleIDs(findings); len(got) != 1 || got[0] != "taint-nosql-query" {
		t.Fatalf("expected taint-nosql-query, got %v", findings)
	}
	if findings[0].Line != 4 {
		t.Fatalf("expected finding on line 4, got %d", findings[0].Line)
	}
}

func TestNosqlQueryDestructuredSafe(t *testing.T) {
	src := `
function handler(req, res) {
  const { email } = req.body;
  User.findOne({ email });
}
`
	findings := scanSource(t, "handler.js", src)
	for _, f := range findings {
		if f.RuleID == "taint-nosql-query" {
			t.Fatalf("expected no taint-nosql-query for destructured field, got %#v", f)
		}
	}
}

func TestMassAssignmentTaintedVar(t *testing.T) {
	src := `
function create(req, res) {
  const data = req.body;
  User.create(data);
}
`
	findings := scanSource(t, "create.js", src)
	if got := ruleIDs(findings); len(got) != 1 || got[0] != "taint-mass-assignment" {
		t.Fatalf("expected taint-mass-assignment, got %v", findings)
	}
}

func TestCommandInjectionTaintedVar(t *testing.T) {
	src := `
function run(req, res) {
  const name = req.query.name;
  exec("ping " + name);
}

function runSafe(req, res) {
  exec("ping -c1 localhost");
}
`
	findings := scanSource(t, "run.js", src)
	if got := ruleIDs(findings); len(got) != 1 || got[0] != "taint-command-injection" {
		t.Fatalf("expected single taint-command-injection, got %v", findings)
	}
}

func TestCodeInjectionTaintedVar(t *testing.T) {
	src := `
function run(req, res) {
  const expr = req.body.expr;
  eval(expr);
}
`
	findings := scanSource(t, "run.js", src)
	if got := ruleIDs(findings); len(got) != 1 || got[0] != "taint-code-injection" {
		t.Fatalf("expected taint-code-injection, got %v", findings)
	}
}

func TestSSRFTaintedVar(t *testing.T) {
	src := `
function proxy(req, res) {
  const target = req.query.url;
  fetch(target);
}
`
	findings := scanSource(t, "proxy.js", src)
	if got := ruleIDs(findings); len(got) != 1 || got[0] != "taint-ssrf" {
		t.Fatalf("expected taint-ssrf, got %v", findings)
	}
}

func TestClosureCallbackPropagation(t *testing.T) {
	src := `
function handler(req, res) {
  const filter = req.body;
  db.connect(function (err, conn) {
    conn.collection("users").find(filter);
  });
}
`
	findings := scanSource(t, "handler.js", src)
	if got := ruleIDs(findings); len(got) != 1 || got[0] != "taint-nosql-query" {
		t.Fatalf("expected taint-nosql-query inside closure, got %v", findings)
	}
}

func TestNoTaintedSourceStaysQuiet(t *testing.T) {
	src := `
function handler(req, res) {
  const filter = { active: true };
  User.find(filter);
  exec("ls -la");
  fetch("https://example.com");
}
`
	findings := scanSource(t, "handler.js", src)
	if len(findings) != 0 {
		t.Fatalf("expected no findings without a tainted source, got %#v", findings)
	}
}

func TestPromptInjectionOpenAIChatCompletions(t *testing.T) {
	src := `
function handler(req, res) {
  const question = req.body.question;
  openai.chat.completions.create({
    model: "gpt-4",
    messages: [{ role: "user", content: ` + "`Answer this: ${question}`" + ` }],
  });
}
`
	findings := scanSource(t, "handler.js", src)
	if got := ruleIDs(findings); len(got) != 1 || got[0] != "taint-prompt-injection" {
		t.Fatalf("expected taint-prompt-injection, got %v", findings)
	}
}

func TestPromptInjectionVercelAISDK(t *testing.T) {
	src := `
function handler(req, res) {
  const prompt = req.body.prompt;
  generateText({ model: openai("gpt-4o"), prompt });
}
`
	findings := scanSource(t, "handler.js", src)
	if got := ruleIDs(findings); len(got) != 1 || got[0] != "taint-prompt-injection" {
		t.Fatalf("expected taint-prompt-injection, got %v", findings)
	}
}

func TestPromptInjectionSafeStaticPrompt(t *testing.T) {
	src := `
function handler(req, res) {
  generateText({ model: openai("gpt-4o"), prompt: "Summarize the weekly report." });
}
`
	findings := scanSource(t, "handler.js", src)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for static prompt, got %#v", findings)
	}
}

func TestDirectSourceArgumentNotDoubleReported(t *testing.T) {
	// req.body passed directly (no variable indirection) is the job of the
	// semgrep nosql-injection-tainted-query rule, not this analyzer.
	src := `
function handler(req, res) {
  User.find(req.body);
}
`
	findings := scanSource(t, "handler.js", src)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for direct source argument, got %#v", findings)
	}
}
