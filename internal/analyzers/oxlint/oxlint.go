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

// baseConfig enables the correctness rule sets that catch real bugs and codegen
// smells, while leaving out the stylistic/pedantic categories (magic numbers,
// comment casing, filename case) that would drown the signal. The %s holes are
// the plugin list and the rule-override block, both assembled by buildConfig so
// React-only rules are referenced only when the React plugins are enabled.
//
// Written to a temp file per scan so the user's repo is never touched.
const baseConfig = `{
  "plugins": [%s],
  "categories": { "correctness": "warn", "suspicious": "warn", "perf": "warn" },
  "rules": {%s}
}`

// reactRules are the per-rule tweaks that only make sense with the React plugins
// loaded:
//   - react-in-jsx-scope OFF: noise under the modern JSX transform.
//   - jsx-no-new-function-as-prop OFF: inline handlers are idiomatic React and
//     this rule fires on nearly every component, so it buries the signal.
//   - button-has-type ON: a <button> defaults to type="submit" and silently
//     submits forms; a real bug the default config misses.
const reactRules = `
    "react/react-in-jsx-scope": "off",
    "react-perf/jsx-no-new-function-as-prop": "off",
    "react/button-has-type": "warn"
  `

// buildConfig assembles the oxlint config. The React-family plugins
// (react, react-perf, jsx-a11y) are enabled only for actual React projects:
// their rules (e.g. react/no-this-in-sfc) otherwise misfire on plain Node/Express
// code, where any function using `this` is mistaken for a stateless component.
// The Next.js plugin is appended only for real Next.js apps.
func buildConfig(react, next bool) string {
	var plugins []string
	rules := ""
	if react {
		plugins = append(plugins, `"react"`, `"react-perf"`, `"jsx-a11y"`)
		rules = reactRules
	}
	if next {
		plugins = append(plugins, `"nextjs"`)
	}
	return fmt.Sprintf(baseConfig, strings.Join(plugins, ", "), rules)
}

// relevantPkgDirs returns the package.json locations worth inspecting: the repo
// root plus every directory on the path from each scanned file up to the root.
// The walk-up makes dependency detection work in monorepos/workspaces where an
// app lives in a sub-package (apps/web) rather than the root.
func relevantPkgDirs(ctx core.ProjectContext) map[string]bool {
	dirs := map[string]bool{ctx.Root: true}
	for _, f := range ctx.Files {
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
	return dirs
}

// isNextProject reports whether the scan target is a Next.js app: a next.config.*
// among the scanned files, or a "next" dependency in any relevant package.json.
func isNextProject(ctx core.ProjectContext) bool {
	for _, f := range ctx.Files {
		if strings.HasPrefix(filepath.Base(f), "next.config.") {
			return true
		}
	}
	for dir := range relevantPkgDirs(ctx) {
		if pkgHasDep(filepath.Join(dir, "package.json"), "next") {
			return true
		}
	}
	return false
}

// isReactProject reports whether the scan target is a React app: any scanned
// .jsx/.tsx file, or a "react" dependency in any relevant package.json. Used to
// gate the React-family oxlint plugins so their rules don't misfire on plain
// Node/backend code.
func isReactProject(ctx core.ProjectContext) bool {
	for _, f := range ctx.Files {
		switch strings.ToLower(filepath.Ext(f)) {
		case ".jsx", ".tsx":
			return true
		}
	}
	for dir := range relevantPkgDirs(ctx) {
		if pkgHasDep(filepath.Join(dir, "package.json"), "react") {
			return true
		}
	}
	return false
}

// pkgHasDep reports whether the package.json at path lists dep among its
// dependencies or devDependencies.
func pkgHasDep(path, dep string) bool {
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
	if _, ok := pkg.Dependencies[dep]; ok {
		return true
	}
	_, ok := pkg.DevDependencies[dep]
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
	next := isNextProject(ctx)
	// Next.js implies React, so its presence also enables the React plugins.
	react := next || isReactProject(ctx)
	if _, err := cfg.WriteString(buildConfig(react, next)); err != nil {
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
