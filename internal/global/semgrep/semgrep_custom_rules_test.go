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

	"github.com/aykutssert/scout/internal/core"
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
		Files: []string{"animations.tsx", "app/layout.tsx", "app/page.tsx", "app/script.tsx", "app/subdir/page.tsx", "bun.ts", "case.tsx", "client.tsx", "client_data_fetch.tsx", "design.tsx", "detox.ts", "dynamic_import.ts", "express.ts", "express_sensitive_route_no_auth.ts", "flex_space.tsx", "heavy_import_no_dynamic.tsx", "image_sizes.tsx", "incomplete.ts", "inputs.tsx", "jotai.ts", "js_async_foreach.ts", "next_async_client_component.tsx", "next_no_redirect_in_try_catch.tsx", "nextjs_no_use_search_params_without_suspense.tsx", "node_promise.ts", "page.tsx", "pages/index.tsx", "react19.tsx", "redux.tsx", "regex_dos_candidate.ts", "rn.tsx", "rn_animation.tsx", "route.ts", "scripts.tsx", "security_injection.ts", "server.tsx", "stream_pipe.ts", "server_api_hop.tsx", "server_auth_actions.tsx", "server_no_mutable_module_state.tsx", "skipped_tests.ts", "test_assertionless.ts", "tx_external_io.ts", "tanstack_start.tsx", "tanstack_start_redirect_in_try_catch.tsx", "tanstack_writes.tsx", "unnecessary_client.tsx", "zod.ts"},
	})
	if err != nil {
		t.Fatal(err)
	}

	normalized := normalizeFindings(got)
	byNS := groupFindingsByNamespace(normalized)

	// UPDATE_EXPECTED=true regenerates the per-namespace goldens instead of
	// hand-editing them. Each namespace lives in expected_<ns>.json so that
	// a new rule in one namespace only touches its own golden file — no
	// line-shift churn across unrelated namespaces.
	if os.Getenv("UPDATE_EXPECTED") == "true" {
		for ns, findings := range byNS {
			path := filepath.Join(root, "expected_"+ns+".json")
			data, merr := json.MarshalIndent(findings, "", "  ")
			if merr != nil {
				t.Fatalf("marshal namespace %q: %v", ns, merr)
			}
			data = append(data, '\n')
			if werr := os.WriteFile(path, data, 0o644); werr != nil {
				t.Fatalf("write golden %s: %v", path, werr)
			}
			t.Logf("updated golden %s", path)
		}
		// Remove stale namespace files for namespaces that produced zero findings.
		existing, _ := filepath.Glob(filepath.Join(root, "expected_*.json"))
		for _, f := range existing {
			base := filepath.Base(f)
			ns := strings.TrimPrefix(strings.TrimSuffix(base, ".json"), "expected_")
			if _, ok := byNS[ns]; !ok {
				t.Logf("removing stale golden %s", base)
				os.Remove(f)
			}
		}
		return
	}

	// Verify each namespace matches its golden.
	for ns, findings := range byNS {
		path := filepath.Join(root, "expected_"+ns+".json")
		want, rerr := os.ReadFile(path)
		if rerr != nil {
			t.Fatalf("missing golden for namespace %q — run with UPDATE_EXPECTED=true to generate it", ns)
		}
		actual, merr := json.MarshalIndent(findings, "", "  ")
		if merr != nil {
			t.Fatal(merr)
		}
		actual = append(actual, '\n')
		if string(actual) != string(want) {
			t.Fatalf("golden mismatch for namespace %q\nwant:\n%s\n\ngot:\n%s", ns, want, actual)
		}
	}

	// Detect stale golden files (namespace was removed but file remains).
	existing, _ := filepath.Glob(filepath.Join(root, "expected_*.json"))
	for _, f := range existing {
		base := filepath.Base(f)
		ns := strings.TrimPrefix(strings.TrimSuffix(base, ".json"), "expected_")
		if _, ok := byNS[ns]; !ok {
			t.Errorf("stale golden %s — namespace %q produced no findings; delete the file or run UPDATE_EXPECTED=true", base, ns)
		}
	}
}

// groupFindingsByNamespace splits findings by the namespace prefix of rule_id
// (the part before the first "."). Each namespace maps to its own golden file.
func groupFindingsByNamespace(findings []goldenFinding) map[string][]goldenFinding {
	byNS := make(map[string][]goldenFinding)
	for _, f := range findings {
		ns := namespaceOf(f.RuleID)
		byNS[ns] = append(byNS[ns], f)
	}
	return byNS
}

func namespaceOf(ruleID string) string {
	if i := strings.IndexByte(ruleID, '.'); i >= 0 {
		return ruleID[:i]
	}
	return ruleID
}

func requireUsableSemgrep(t *testing.T) {
	t.Helper()
	required := os.Getenv("SCOUT_REQUIRE_SEMGREP") == "1"
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
// rules/javascript ships without a matching firing in any namespace golden.
// A new rule must come with a fixture that proves it fires, or this test breaks.
func TestEveryCustomRuleHasGoldenCoverage(t *testing.T) {
	repo := repoRoot(t)
	ruleDir := filepath.Join(repo, "rules", "javascript")
	goldenDir := filepath.Join(repo, "internal", "global", "semgrep", "testdata", "custom_rules")

	// Concatenate all namespace golden files for the substring search.
	goldenFiles, err := filepath.Glob(filepath.Join(goldenDir, "expected_*.json"))
	if err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	for _, gf := range goldenFiles {
		data, rerr := os.ReadFile(gf)
		if rerr != nil {
			t.Fatal(rerr)
		}
		sb.Write(data)
	}
	golden := sb.String()

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

func TestValidateCustomRulesOffline(t *testing.T) {
	requireUsableSemgrep(t)

	repo := repoRoot(t)
	ruleDir := filepath.Join(repo, "rules", "javascript")

	tempDir := t.TempDir()

	cmd := exec.Command("semgrep", "scan", "--config", ruleDir, "--disable-version-check", "--metrics=off", tempDir)
	cmd.Dir = repo
	cmd.Env = semgrepEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("semgrep rule validation failed: %v\nOutput:\n%s", err, string(out))
	}
}
