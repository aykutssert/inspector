package semgrep

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/aykutssert/inspector/internal/core"
)

type goldenFinding struct {
	RuleID     string `json:"rule_id"`
	Severity   string `json:"severity"`
	Category   string `json:"category,omitempty"`
	Confidence string `json:"confidence,omitempty"`
	File       string `json:"file"`
	Line       int    `json:"line"`
	Message    string `json:"message"`
}

func TestJavaScriptCustomRulesGolden(t *testing.T) {
	prepareSemgrepTestEnv(t)
	requireUsableSemgrep(t)

	repo := repoRoot(t)
	ruleDir := filepath.Join(repo, "rules", "javascript")
	root := filepath.Join(repo, "internal", "global", "semgrep", "testdata", "custom_rules")

	a := &Analyzer{customDirs: []string{ruleDir}}
	got, err := a.Scan(core.ProjectContext{
		Root:  root,
		Files: []string{"case.tsx", "client.tsx", "express.ts", "incomplete.ts"},
	})
	if err != nil {
		t.Fatal(err)
	}

	normalized := normalizeFindings(got)
	actual, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	actual = append(actual, '\n')

	want, err := os.ReadFile(filepath.Join(root, "expected.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(actual) != string(want) {
		t.Fatalf("custom rule golden mismatch\nwant:\n%s\n\ngot:\n%s", want, actual)
	}
}

func requireUsableSemgrep(t *testing.T) {
	t.Helper()
	required := os.Getenv("INSPECTOR_REQUIRE_SEMGREP") == "1"
	if _, err := exec.LookPath("semgrep"); err != nil {
		if required {
			t.Fatal("semgrep is required but not installed")
		}
		t.Skip("semgrep is not installed")
	}
	cmd := exec.Command("semgrep", "--version")
	cmd.Env = semgrepEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		if required {
			t.Fatalf("semgrep is required but not usable: %v: %s", err, strings.TrimSpace(string(out)))
		}
		t.Skipf("semgrep is not usable in this environment: %v: %s", err, strings.TrimSpace(string(out)))
	}
}

func prepareSemgrepTestEnv(t *testing.T) {
	t.Helper()
	stateDir := t.TempDir()
	t.Setenv("SEMGREP_LOG_FILE", filepath.Join(stateDir, "semgrep.log"))
	t.Setenv("SEMGREP_SETTINGS_FILE", filepath.Join(stateDir, "settings.yml"))
	t.Setenv("SEMGREP_VERSION_CACHE_PATH", filepath.Join(stateDir, "version-cache"))
	t.Setenv("SEMGREP_VERSION_CHECK_TIMEOUT", "0")
	if os.Getenv("SSL_CERT_FILE") == "" {
		if cert := homebrewCertFile(); cert != "" {
			t.Setenv("SSL_CERT_FILE", cert)
			t.Setenv("REQUESTS_CA_BUNDLE", cert)
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			t.Fatal("could not locate repo root")
		}
		wd = parent
	}
}

func normalizeFindings(findings []core.Finding) []goldenFinding {
	out := make([]goldenFinding, 0, len(findings))
	for _, f := range findings {
		out = append(out, goldenFinding{
			RuleID:     f.RuleID,
			Severity:   f.Level,
			Category:   f.Category,
			Confidence: f.Confidence,
			File:       filepath.ToSlash(f.File),
			Line:       f.Line,
			Message:    f.Message,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].RuleID != out[j].RuleID {
			return out[i].RuleID < out[j].RuleID
		}
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out
}
