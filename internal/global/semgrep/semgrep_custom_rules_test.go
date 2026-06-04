package semgrep

import (
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
		Files: []string{"case.tsx", "client.tsx", "express.ts", "incomplete.ts", "rn.tsx"},
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

// TestEveryCustomRuleHasGoldenCoverage fails when a rule file under
// rules/javascript ships without a matching firing in the golden expected.json.
// It is the cheap guard (no semgrep needed) that keeps rule authors honest: a
// new rule must come with a fixture that proves it fires, or this test breaks.
func TestEveryCustomRuleHasGoldenCoverage(t *testing.T) {
	repo := repoRoot(t)
	ruleDir := filepath.Join(repo, "rules", "javascript")

	expected, err := os.ReadFile(filepath.Join(repo,
		"internal", "global", "semgrep", "testdata", "custom_rules", "expected.json"))
	if err != nil {
		t.Fatal(err)
	}
	golden := string(expected)

	idLine := regexp.MustCompile(`(?m)^\s*-\s*id:\s*(\S+)`)
	var missing []string
	err = filepath.WalkDir(ruleDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if ext := filepath.Ext(path); ext != ".yaml" && ext != ".yml" {
			return nil
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		// The finding's rule_id is namespaced by the rule file's parent dir
		// (e.g. general/hardcoded-secret.yaml -> "general.hardcoded-secret").
		ns := filepath.Base(filepath.Dir(path))
		for _, m := range idLine.FindAllStringSubmatch(string(data), -1) {
			id := ns + "." + strings.Trim(m[1], `"'`)
			if !strings.Contains(golden, `"rule_id": "`+id+`"`) {
				missing = append(missing, id+"  ("+filepath.Base(path)+")")
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("custom rules with no golden fixture coverage (add a trigger to testdata/custom_rules and rerun the golden test):\n  %s",
			strings.Join(missing, "\n  "))
	}
}
