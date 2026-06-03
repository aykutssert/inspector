package semgrep

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aykutssert/inspector/internal/core"
	"github.com/aykutssert/inspector/internal/execx"
)

type Analyzer struct {
	configs []string
	// customDirs hold user-authored YAML rule packs (one per language adapter).
	// Rules there surface as hints unless the rule's metadata sets confidence
	// explicitly. Non-existent dirs are skipped at scan time.
	customDirs []string
}

// defaultConfigs are registry rule packs covering JS/TS/React plus a general
// security audit. We avoid "auto" because it requires metrics to be enabled
// (semgrep: "Cannot create auto config when metrics are off"), and we always
// run with --metrics=off for privacy.
var defaultConfigs = []string{
	"p/default",
	"p/javascript",
	"p/typescript",
	"p/react",
	"p/security-audit",
	"p/secrets",
}

func New(config string, customDirs ...string) *Analyzer {
	a := &Analyzer{customDirs: customDirs}
	if config == "" {
		a.configs = defaultConfigs
	} else {
		a.configs = []string{config}
	}
	return a
}

var _ core.Analyzer = (*Analyzer)(nil)

func (a *Analyzer) Name() string { return "semgrep" }

func (a *Analyzer) InstallHint() string { return "install semgrep (pip install semgrep)" }

func (a *Analyzer) Available() bool {
	_, err := exec.LookPath("semgrep")
	return err == nil
}

type semgrepOut struct {
	Results []struct {
		CheckID string `json:"check_id"`
		Path    string `json:"path"`
		Start   struct {
			Line int `json:"line"`
		} `json:"start"`
		Extra struct {
			Message  string `json:"message"`
			Severity string `json:"severity"`
			Fix      string `json:"fix"`
			Metadata struct {
				Category string          `json:"category"`
				Cwe      json.RawMessage `json:"cwe"`
				Owasp    json.RawMessage `json:"owasp"`
				// Confidence lets a custom rule opt into "hint" so the agent
				// verifies it rather than trusting it as a deterministic defect.
				Confidence string `json:"confidence"`
			} `json:"metadata"`
		} `json:"extra"`
	} `json:"results"`
	Errors []struct {
		Level string `json:"level"`
		// Type is ["KindString", [spans...]]; we read the kind from element 0.
		Type    []json.RawMessage `json:"type"`
		Message string            `json:"message"`
		Path    string            `json:"path"`
		Spans   []struct {
			File  string `json:"file"`
			Start struct {
				Line int `json:"line"`
			} `json:"start"`
		} `json:"spans"`
	} `json:"errors"`
}

// parseErrorKinds are the semgrep error kinds that mean source code did not
// parse — a likely syntax error. Other error kinds (rule errors, timeouts) are
// tool noise, not findings, so we ignore them.
var parseErrorKinds = map[string]bool{
	"PartialParsing": true,
	"SyntaxError":    true,
	"LexicalError":   true,
}

func errorKind(t []json.RawMessage) string {
	if len(t) == 0 {
		return ""
	}
	var kind string
	_ = json.Unmarshal(t[0], &kind)
	return kind
}

// present reports whether a JSON metadata field carries a real value (semgrep
// emits cwe/owasp as a string or array, or omits them entirely).
func present(raw json.RawMessage) bool {
	return len(raw) > 0 && string(raw) != "null" && string(raw) != `""` && string(raw) != "[]"
}

func (a *Analyzer) Scan(ctx core.ProjectContext) ([]core.Finding, error) {
	// diff mode with no changed files: nothing to scan (never fall back to root)
	if ctx.DiffOnly && len(ctx.Files) == 0 {
		return nil, nil
	}
	args := []string{"--json", "--quiet", "--metrics=off"}
	for _, c := range a.configs {
		args = append(args, "--config", c)
	}
	// User-authored custom rule packs. Skip dirs that don't exist so a project
	// without a rules/ dir scans cleanly instead of failing.
	var loadedDirs []string
	for _, dir := range a.customDirs {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			args = append(args, "--config", dir)
			loadedDirs = append(loadedDirs, dir)
		}
	}
	if len(ctx.Files) > 0 {
		args = append(args, ctx.Files...)
	} else {
		args = append(args, ctx.Root)
	}
	cmd := exec.Command("semgrep", args...)
	cmd.Dir = ctx.Root
	cmd.Env = semgrepEnv()
	out, err := cmd.Output()
	// Without --error, semgrep exits 0 even when findings exist; a non-zero exit
	// is a real failure (config error, crash, partial run). Surface it with the
	// process stderr so the reason (bad config, trust-store error) is visible.
	if err != nil {
		return nil, execx.ErrWithOutput(err, out)
	}
	var parsed semgrepOut
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, err
	}
	prefixes := configPrefixes(loadedDirs)
	var findings []core.Finding
	for _, r := range parsed.Results {
		sev := mapSeverity(r.Extra.Severity)
		cat := classify(r.Extra.Metadata.Category, present(r.Extra.Metadata.Cwe) || present(r.Extra.Metadata.Owasp))
		findings = append(findings, core.Finding{
			Analyzer:   a.Name(),
			RuleID:     cleanRuleID(r.CheckID, prefixes),
			Severity:   sev,
			Level:      sev.String(),
			Category:   cat,
			Confidence: mapConfidence(r.Extra.Metadata.Confidence),
			File:       r.Path,
			Line:       r.Start.Line,
			Message:    r.Extra.Message,
			Fix:        r.Extra.Fix,
		})
	}
	// A failed parse means semgrep could not read the file — likely a syntax
	// error, but possibly grammar it doesn't support. Surface it as a hint
	// (verify, don't trust) rather than a hard rule, to stay low-noise.
	for _, e := range parsed.Errors {
		if !parseErrorKinds[errorKind(e.Type)] {
			continue
		}
		file, line := e.Path, 0
		if len(e.Spans) > 0 {
			if file == "" {
				file = e.Spans[0].File
			}
			line = e.Spans[0].Start.Line
		}
		findings = append(findings, core.Finding{
			Analyzer:   a.Name(),
			RuleID:     "syntax-or-parse-error",
			Severity:   core.SeverityWarning,
			Level:      core.SeverityWarning.String(),
			Category:   "bug",
			Confidence: core.ConfidenceHint,
			File:       file,
			Line:       line,
			Message:    e.Message,
			Fix:        "Confirm this file parses — semgrep could not fully parse it (likely a syntax error).",
		})
	}
	return findings, nil
}

// classify maps semgrep rule metadata to our finding category. A CWE/OWASP tag
// means security regardless of the declared category.
func classify(metaCategory string, hasSecurityTag bool) string {
	if hasSecurityTag {
		return "security"
	}
	switch metaCategory {
	case "security":
		return "security"
	case "performance":
		return "performance"
	case "correctness":
		return "bug"
	case "best-practice", "maintainability", "portability", "compatibility":
		return "quality"
	}
	return ""
}

// configPrefixes turns each loaded custom-rule dir into the dotted namespace
// semgrep prepends to a rule's id when it loads rules from that path (it drops
// the leading slash and replaces "/" with "."). We strip these so a custom
// finding reads "direct-web-storage-access", not the machine's full path.
func configPrefixes(dirs []string) []string {
	prefixes := make([]string, 0, len(dirs))
	for _, d := range dirs {
		p := strings.ReplaceAll(strings.TrimPrefix(d, "/"), "/", ".")
		prefixes = append(prefixes, p+".")
	}
	return prefixes
}

func cleanRuleID(id string, prefixes []string) string {
	for _, p := range prefixes {
		if strings.HasPrefix(id, p) {
			return strings.TrimPrefix(id, p)
		}
	}
	return id
}

// mapConfidence honors a rule's explicit metadata.confidence ("hint"), and
// defaults to rule confidence for registry packs that don't set it.
func mapConfidence(c string) string {
	if c == core.ConfidenceHint {
		return core.ConfidenceHint
	}
	return core.ConfidenceRule
}

func mapSeverity(s string) core.Severity {
	switch s {
	case "ERROR":
		return core.SeverityError
	case "WARNING":
		return core.SeverityWarning
	default:
		return core.SeverityInfo
	}
}

func semgrepEnv() []string {
	env := os.Environ()
	if os.Getenv("SEMGREP_VERSION_CHECK_TIMEOUT") == "" {
		env = append(env, "SEMGREP_VERSION_CHECK_TIMEOUT=0")
	}
	if os.Getenv("SSL_CERT_FILE") == "" {
		if cert := homebrewCertFile(); cert != "" {
			env = append(env, "SSL_CERT_FILE="+cert)
			if os.Getenv("REQUESTS_CA_BUNDLE") == "" {
				env = append(env, "REQUESTS_CA_BUNDLE="+cert)
			}
		}
	}
	return env
}

func homebrewCertFile() string {
	for _, p := range []string{
		"/opt/homebrew/etc/ca-certificates/cert.pem",
		"/usr/local/etc/ca-certificates/cert.pem",
	} {
		if _, err := os.Stat(filepath.Clean(p)); err == nil {
			return p
		}
	}
	return ""
}
