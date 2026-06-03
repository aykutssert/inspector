package oxlint

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aykutssert/inspector/internal/core"
	"github.com/aykutssert/inspector/internal/execx"
)

type Analyzer struct{}

func New() *Analyzer { return &Analyzer{} }

var _ core.Analyzer = (*Analyzer)(nil)

func (a *Analyzer) Name() string { return "oxlint" }

func (a *Analyzer) InstallHint() string { return "install oxlint (npm i -g oxlint)" }

func (a *Analyzer) Available() bool {
	_, err := exec.LookPath("oxlint")
	return err == nil
}

// curatedConfig enables the React/correctness rule sets that catch real bugs
// and codegen smells, while leaving out the stylistic/pedantic categories
// (magic numbers, comment casing, filename case) that would drown the signal.
// Two deliberate per-rule tweaks:
//   - jsx-no-new-function-as-prop OFF: inline handlers are idiomatic React and
//     this rule fires on nearly every component, so it buries the signal.
//   - button-has-type ON: a <button> defaults to type="submit" and silently
//     submits forms; a real bug the default config misses.
//
// The nextjs plugin is appended only for real Next.js projects (see isNextProject);
// its rules (@next/no-img-element, no-html-link-for-pages) fire on any <img>/<a>
// and would be noise in a plain React app.
//
// Written to a temp file per scan so the user's repo is never touched.
const baseConfig = `{
  "plugins": [%s],
  "categories": { "correctness": "warn", "suspicious": "warn", "perf": "warn" },
  "rules": {
    "react/react-in-jsx-scope": "off",
    "react-perf/jsx-no-new-function-as-prop": "off",
    "react/button-has-type": "warn"
  }
}`

// buildConfig assembles the oxlint config, enabling the Next.js plugin only when
// the scanned project is actually a Next.js app.
func buildConfig(next bool) string {
	plugins := `"react", "react-perf", "jsx-a11y"`
	if next {
		plugins += `, "nextjs"`
	}
	return fmt.Sprintf(baseConfig, plugins)
}

// isNextProject reports whether the scan target is a Next.js app. It looks for a
// next.config.* among the scanned files, or a "next" dependency in any relevant
// package.json — the repo root plus the package.json walked up from each scanned
// file's directory. The walk-up makes detection work in monorepos/workspaces
// where the Next.js app lives in a sub-package (apps/web) rather than the root.
func isNextProject(ctx core.ProjectContext) bool {
	// package.json locations to inspect: repo root, plus every directory on the
	// path from each scanned file up to the root.
	dirs := map[string]bool{ctx.Root: true}
	for _, f := range ctx.Files {
		base := filepath.Base(f)
		if strings.HasPrefix(base, "next.config.") {
			return true
		}
		dir := filepath.Dir(f)
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(ctx.Root, dir)
		}
		for {
			dirs[dir] = true
			if dir == ctx.Root || !strings.HasPrefix(dir, ctx.Root) {
				break
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	for dir := range dirs {
		if pkgHasNext(filepath.Join(dir, "package.json")) {
			return true
		}
	}
	return false
}

// pkgHasNext reports whether the package.json at path lists "next" among its
// dependencies or devDependencies.
func pkgHasNext(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if json.Unmarshal(data, &pkg) != nil {
		return false
	}
	if _, ok := pkg.Dependencies["next"]; ok {
		return true
	}
	_, ok := pkg.DevDependencies["next"]
	return ok
}

type oxlintOut struct {
	Diagnostics []struct {
		Message  string `json:"message"`
		Code     string `json:"code"`
		Severity string `json:"severity"`
		Help     string `json:"help"`
		Filename string `json:"filename"`
		Labels   []struct {
			Span struct {
				Line int `json:"line"`
			} `json:"span"`
		} `json:"labels"`
	} `json:"diagnostics"`
}

func (a *Analyzer) Scan(ctx core.ProjectContext) ([]core.Finding, error) {
	if len(ctx.Files) == 0 {
		return nil, nil // diff mode with no JS files, or empty project
	}

	cfg, err := os.CreateTemp("", "inspector-oxlint-*.json")
	if err != nil {
		return nil, err
	}
	defer os.Remove(cfg.Name())
	if _, err := cfg.WriteString(buildConfig(isNextProject(ctx))); err != nil {
		cfg.Close()
		return nil, err
	}
	cfg.Close()

	args := []string{"-c", cfg.Name(), "--format", "json"}
	args = append(args, ctx.Files...)
	cmd := exec.Command("oxlint", args...)
	cmd.Dir = ctx.Root
	out, err := cmd.Output()
	// oxlint exits non-zero when it reports diagnostics; that is not a failure.
	// A real failure produces no JSON on stdout.
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return nil, execx.Err(err)
		}
		if len(out) == 0 {
			return nil, execx.Err(err)
		}
	}

	var parsed oxlintOut
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, err
	}

	var findings []core.Finding
	for _, d := range parsed.Diagnostics {
		plugin, rule := splitCode(d.Code)
		sev := mapSeverity(d.Severity)
		line := 0
		if len(d.Labels) > 0 {
			line = d.Labels[0].Span.Line
		}
		findings = append(findings, core.Finding{
			Analyzer:   a.Name(),
			RuleID:     rule,
			Severity:   sev,
			Level:      sev.String(),
			Category:   classify(plugin),
			Confidence: core.ConfidenceRule,
			File:       d.Filename,
			Line:       line,
			Message:    d.Message,
			Fix:        d.Help,
		})
	}
	return findings, nil
}

// splitCode turns oxlint's "plugin(rule-name)" into its parts. A bare code with
// no parens returns ("", code).
func splitCode(code string) (plugin, rule string) {
	open := strings.IndexByte(code, '(')
	if open < 0 || !strings.HasSuffix(code, ")") {
		return "", code
	}
	return code[:open], code[open+1 : len(code)-1]
}

func classify(plugin string) string {
	switch plugin {
	case "react-perf":
		return "performance"
	case "jsx-a11y":
		return "quality"
	case "react", "typescript", "oxc":
		return "bug"
	case "nextjs":
		return "quality"
	default:
		return "quality"
	}
}

func mapSeverity(s string) core.Severity {
	switch s {
	case "error":
		return core.SeverityError
	case "warning":
		return core.SeverityWarning
	default:
		return core.SeverityInfo
	}
}
