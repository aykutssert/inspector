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

// isTypeScriptFile reports whether path is a TypeScript source file, where tsc
// (not semgrep) is the parse authority for advanced syntax.
func isTypeScriptFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ts", ".tsx", ".mts", ".cts":
		return true
	}
	return false
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
	// User-authored custom rule packs. Skip dirs that don't exist so a project
	// without a rules/ dir scans cleanly instead of failing.
	loadedDirs := existingDirs(a.customDirs)

	out, err := a.run(ctx, a.configs, loadedDirs)
	// Without --error, semgrep exits 0 even when findings exist; a non-zero exit
	// is a real failure (config error, crash, partial run).
	if err == nil {
		return parseFindings(a.Name(), out, loadedDirs)
	}
	exit, ok := err.(*exec.ExitError)
	if !ok {
		return nil, execx.Err(err)
	}
	// A non-zero exit caused by an unreachable registry is a transient
	// infrastructure problem, not a code defect. Reporting a hard analyzer error
	// reads as "your code is broken", so instead we degrade: rerun with only the
	// local rules so our own deterministic checks still produce findings, and
	// emit one notice that registry-backed coverage was reduced.
	registry, local := splitRegistry(a.configs)
	if len(registry) > 0 && isRegistryFailure(string(exit.Stderr)) {
		if len(local)+len(loadedDirs) > 0 {
			if out2, err2 := a.run(ctx, local, loadedDirs); err2 == nil {
				findings, perr := parseFindings(a.Name(), out2, loadedDirs)
				if perr != nil {
					return nil, perr
				}
				return append([]core.Finding{registryNotice(a.Name(), true)}, findings...), nil
			}
		}
		// No local rules to fall back on: report the degraded coverage honestly
		// rather than a misleading code error.
		return []core.Finding{registryNotice(a.Name(), false)}, nil
	}
	// A genuine failure (bad rule, crash, invalid config): surface it with the
	// process stderr so the reason is visible.
	return nil, execx.ErrWithOutput(err, out)
}

// run invokes semgrep with the given registry/local configs plus the loaded
// custom-rule dirs, scanning the changed files (or the whole root). It returns
// semgrep's raw JSON stdout; a non-zero exit comes back as the error.
func (a *Analyzer) run(ctx core.ProjectContext, configs, dirs []string) ([]byte, error) {
	args := []string{"--json", "--quiet", "--metrics=off"}
	for _, c := range configs {
		args = append(args, "--config", c)
	}
	for _, d := range dirs {
		args = append(args, "--config", d)
	}
	if len(ctx.Files) > 0 {
		args = append(args, ctx.Files...)
	} else {
		args = append(args, ctx.Root)
	}
	cmd := exec.Command("semgrep", args...)
	cmd.Dir = ctx.Root
	cmd.Env = semgrepEnv()
	return cmd.Output()
}

// existingDirs keeps only the custom-rule dirs that exist, so a project without
// a rules/ dir scans cleanly instead of failing on a missing --config path.
func existingDirs(dirs []string) []string {
	var out []string
	for _, dir := range dirs {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			out = append(out, dir)
		}
	}
	return out
}

func parseFindings(analyzer string, out []byte, loadedDirs []string) ([]core.Finding, error) {
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
			Analyzer:   analyzer,
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
		// On TypeScript files, tsc is the parse authority. semgrep's TS grammar
		// chokes on valid advanced generics (e.g. Mutate<StoreApi<T>, Mos>) and
		// reports them as syntax errors — a false positive on code that compiles.
		// Drop semgrep parse errors for .ts/.tsx/.mts/.cts and let tsc speak.
		if isTypeScriptFile(file) {
			continue
		}
		findings = append(findings, core.Finding{
			Analyzer:   analyzer,
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

// splitRegistry separates Semgrep registry references (p/…, r/…, or a
// semgrep.dev URL — all fetched over the network) from local rule files/dirs,
// which load offline. The split lets us drop the registry half and rerun on the
// local half when the registry is unreachable.
func splitRegistry(configs []string) (registry, local []string) {
	for _, c := range configs {
		if strings.HasPrefix(c, "p/") || strings.HasPrefix(c, "r/") || strings.HasPrefix(c, "https://semgrep.dev") {
			registry = append(registry, c)
		} else {
			local = append(local, c)
		}
	}
	return registry, local
}

// registryFailureMarkers are substrings Semgrep (and its HTTP layer) prints to
// stderr when it cannot fetch rules from the registry — DNS, connectivity, TLS,
// or a registry 5xx. They distinguish a transient network problem from a real
// rule/config error, so we only degrade coverage when one is present.
var registryFailureMarkers = []string{
	"failed to download",
	"could not reach",
	"registry",
	"connection error",
	"connection refused",
	"max retries exceeded",
	"temporary failure in name resolution",
	"name or service not known",
	"network is unreachable",
	"timed out",
	"read timed out",
	"failed to establish a new connection",
	"sslerror",
	"certificate verify failed",
	"502 server error",
	"503 server error",
	"504 server error",
	"bad gateway",
	"service unavailable",
}

func isRegistryFailure(stderr string) bool {
	s := strings.ToLower(stderr)
	for _, m := range registryFailureMarkers {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}

// registryNotice reports, without raising a false code-defect error, that the
// registry packs could not be downloaded this run. ranLocal distinguishes "we
// still ran our local rules" from "semgrep produced nothing".
func registryNotice(analyzer string, ranLocal bool) core.Finding {
	msg := "Semgrep's registry rule packs could not be downloaded (network or registry failure) and no local rules were configured, so semgrep contributed no findings this run."
	if ranLocal {
		msg = "Semgrep's registry rule packs could not be downloaded (network or registry failure); only local rules ran, so registry-backed security coverage was reduced this run."
	}
	return core.Finding{
		Analyzer:   analyzer,
		RuleID:     "semgrep-registry-unavailable",
		Severity:   core.SeverityWarning,
		Level:      core.SeverityWarning.String(),
		Category:   "quality",
		Confidence: core.ConfidenceHint,
		Message:    msg,
		Fix:        "Restore network access so semgrep can fetch its registry rule packs, then rescan.",
	}
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
