// Package tsc wraps `tsc --noEmit` to surface real TypeScript type errors —
// the class of bug oxlint cannot see because it has no type information.
package tsc

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/aykutssert/scout/internal/core"
	"github.com/aykutssert/scout/internal/execx"
	"github.com/aykutssert/scout/internal/toolchain"
)

type Analyzer struct{}

func New() *Analyzer { return &Analyzer{} }

var _ core.Analyzer = (*Analyzer)(nil)

func (a *Analyzer) Name() string { return "tsc" }

func (a *Analyzer) Available() bool { return true }

// tsc emits "path(line,col): error TS1234: message".
var diagRe = regexp.MustCompile(`^(.+?)\((\d+),(\d+)\): (error|warning) (TS\d+): (.+)$`)
var globalDiagRe = regexp.MustCompile(`^(error|warning) (TS\d+): (.+)$`)

func hasTSFiles(files []string) bool {
	for _, f := range files {
		switch strings.ToLower(filepath.Ext(f)) {
		case ".ts", ".tsx", ".mts", ".cts":
			return true
		}
	}
	return false
}

func (a *Analyzer) Scan(ctx core.ProjectContext) ([]core.Finding, error) {
	if !hasTSFiles(ctx.Files) {
		return nil, nil
	}
	// A repo can hold several TypeScript projects (monorepo / pnpm workspace:
	// apps/web, packages/ui). Type-check each one from its own tsconfig rather
	// than assuming a single root tsconfig, which would miss every sub-package.
	dirs := tsProjectDirs(ctx)
	if len(dirs) == 0 {
		return nil, nil // no tsconfig governs the scanned files; tsc needs one
	}
	var findings []core.Finding
	for _, dir := range dirs {
		fs, err := a.scanProject(ctx.Root, dir)
		if err != nil {
			return nil, err
		}
		findings = append(findings, fs...)
	}
	return findings, nil
}

func (a *Analyzer) scanProject(root, dir string) ([]core.Finding, error) {
	// Type checking resolves third-party types from node_modules — the project's
	// own, or the workspace root's hoisted copy (pnpm/npm/yarn workspaces).
	// Without either, every external import errors as "cannot find module" and
	// floods the report, so we skip and say so rather than emit noise.
	if !hasNodeModules(dir) && !hasNodeModules(root) {
		return []core.Finding{skipNotice(a.Name(),
			"TypeScript project at "+projectLabel(root, dir)+" found but node_modules is missing; type checking was skipped to avoid false module-resolution errors.")}, nil
	}

	bin := resolveTSC(dir, root)
	if bin == "" {
		return []core.Finding{skipNotice(a.Name(),
			"TypeScript project at "+projectLabel(root, dir)+" found but no tsc binary is available (repo or toolchain); type checking was skipped.")}, nil
	}

	cmd := exec.Command(bin, "--noEmit", "--pretty", "false", "-p", "tsconfig.json")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		// tsc exits non-zero when it reports errors; that is expected. A genuine
		// failure (crash/missing executable) produces no parseable diagnostics.
		if _, ok := err.(*exec.ExitError); !ok {
			return nil, execx.Err(err)
		}
		if len(strings.TrimSpace(string(out))) == 0 {
			return nil, execx.Err(err)
		}
	}

	findings := parseDiagnostics(a.Name(), string(out))
	for i := range findings {
		findings[i].File = normalizeFile(root, dir, findings[i].File)
	}
	if len(findings) > 0 || err == nil {
		return findings, nil
	}
	return []core.Finding{unparsedOutputNotice(a.Name(), string(out))}, nil
}

// tsProjectDirs returns the directories of the tsconfig.json files that govern
// the scanned TS files: for each .ts/.tsx file, the nearest tsconfig.json found
// walking up to the repo root. In a monorepo this yields one entry per workspace
// package instead of assuming a single root tsconfig. Result is sorted and
// de-duplicated for deterministic output.
func tsProjectDirs(ctx core.ProjectContext) []string {
	seen := map[string]bool{}
	for _, f := range ctx.Files {
		switch strings.ToLower(filepath.Ext(f)) {
		case ".ts", ".tsx", ".mts", ".cts":
		default:
			continue
		}
		dir := filepath.Dir(f)
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(ctx.Root, dir)
		}
		if d := nearestTSConfigDir(dir, ctx.Root); d != "" {
			seen[d] = true
		}
	}
	out := make([]string, 0, len(seen))
	for d := range seen {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

// nearestTSConfigDir walks up from dir to root (inclusive) and returns the first
// directory that contains a tsconfig.json, or "" if none does.
func nearestTSConfigDir(dir, root string) string {
	for {
		if _, err := os.Stat(filepath.Join(dir, "tsconfig.json")); err == nil {
			return dir
		}
		if dir == root || !strings.HasPrefix(dir, root) {
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func hasNodeModules(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "node_modules"))
	return err == nil
}

// normalizeFile rewrites a tsc diagnostic path (relative to the project dir tsc
// ran in) into a path relative to the repo root, so findings from every
// workspace share one coordinate system. Empty paths (global diagnostics) and
// paths that escape the root pass through unchanged.
func normalizeFile(root, dir, file string) string {
	if file == "" {
		return ""
	}
	abs := file
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(dir, file)
	}
	if rel, err := filepath.Rel(root, abs); err == nil {
		return rel
	}
	return file
}

// projectLabel renders a project directory relative to root for skip messages,
// using "repo root" for the root itself.
func projectLabel(root, dir string) string {
	if rel, err := filepath.Rel(root, dir); err == nil && rel != "." {
		return rel
	}
	return "repo root"
}

func parseDiagnostics(analyzer, output string) []core.Finding {
	var findings []core.Finding
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r")
		if finding, ok := parseFileDiagnostic(analyzer, line); ok {
			findings = append(findings, finding)
			continue
		}
		if finding, ok := parseGlobalDiagnostic(analyzer, line); ok {
			findings = append(findings, finding)
		}
	}
	return findings
}

func parseFileDiagnostic(analyzer, line string) (core.Finding, bool) {
	m := diagRe.FindStringSubmatch(line)
	if m == nil {
		return core.Finding{}, false
	}
	ln, _ := strconv.Atoi(m[2])
	sev := tscSeverity(m[4])
	return core.Finding{
		Analyzer:   analyzer,
		RuleID:     m[5], // TS error code
		Severity:   sev,
		Level:      sev.String(),
		Category:   "bug",
		Confidence: core.ConfidenceRule,
		File:       m[1],
		Line:       ln,
		Message:    m[6],
	}, true
}

func parseGlobalDiagnostic(analyzer, line string) (core.Finding, bool) {
	m := globalDiagRe.FindStringSubmatch(line)
	if m == nil {
		return core.Finding{}, false
	}
	sev := tscSeverity(m[1])
	return core.Finding{
		Analyzer:   analyzer,
		RuleID:     m[2],
		Severity:   sev,
		Level:      sev.String(),
		Category:   "bug",
		Confidence: core.ConfidenceRule,
		Message:    m[3],
	}, true
}

func tscSeverity(raw string) core.Severity {
	if raw == "warning" {
		return core.SeverityWarning
	}
	return core.SeverityError
}

// resolveTSC prefers a project-local tsc (matching the project's TS version),
// then the workspace root's hoisted tsc, then scout's managed toolchain.
func resolveTSC(dir, root string) string {
	for _, base := range []string{dir, root} {
		local := filepath.Join(base, "node_modules", ".bin", "tsc")
		if _, err := os.Stat(local); err == nil {
			return local
		}
	}
	if td, ok := toolchain.Dir("typescript"); ok {
		managed := filepath.Join(td, "node_modules", ".bin", "tsc")
		if _, err := os.Stat(managed); err == nil {
			return managed
		}
	}
	return ""
}

func skipNotice(analyzer, msg string) core.Finding {
	return core.Finding{
		Analyzer:   analyzer,
		RuleID:     "type-check-skipped",
		Severity:   core.SeverityInfo,
		Level:      core.SeverityInfo.String(),
		Category:   "quality",
		Confidence: core.ConfidenceHint,
		Message:    msg,
		Fix:        "run `npm install` in the project so types resolve",
	}
}

func unparsedOutputNotice(analyzer, output string) core.Finding {
	msg := strings.TrimSpace(output)
	if msg == "" {
		msg = "tsc exited non-zero but produced no diagnostics."
	}
	return core.Finding{
		Analyzer:   analyzer,
		RuleID:     "type-check-output-unparsed",
		Severity:   core.SeverityWarning,
		Level:      core.SeverityWarning.String(),
		Category:   "quality",
		Confidence: core.ConfidenceHint,
		Message:    "TypeScript type checking produced output scout could not parse: " + firstLine(msg),
		Fix:        "Run `tsc --noEmit --pretty false -p tsconfig.json` and fix the reported config/parser problem.",
	}
}

func firstLine(text string) string {
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		return text[:i]
	}
	return text
}
