package oxlint

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/aykutssert/inspector/internal/core"
	"github.com/aykutssert/inspector/internal/execx"
	"github.com/aykutssert/inspector/internal/lang/javascript/jsproject"
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
//   - rules-of-hooks ON (error): conditional/loop hook calls break React's hook
//     ordering — a real crash-class bug. Not enabled by the default categories,
//     so it must be set explicitly.
//   - exhaustive-deps ON (warn): a useEffect/useCallback with missing deps is
//     the most common React bug source (stale closures); warn, since it has
//     occasional false positives the agent should verify.
const reactRules = `
    "react/react-in-jsx-scope": "off",
    "react-perf/jsx-no-new-function-as-prop": "off",
    "react/button-has-type": "warn",
    "react-hooks/rules-of-hooks": "error",
    "react-hooks/exhaustive-deps": "warn"
  `

// coreNoiseRules are low-signal style/churn checks that bury security and bug
// findings in large JS repos. Dead-code signal is handled by the knip analyzer,
// so oxlint should not also report every unused local or shadowed identifier.
var coreNoiseRuleList = []string{
	"no-unused-vars",
	"no-shadow",
}

var coreNoiseRules = map[string]bool{
	"no-unused-vars": true,
	"no-shadow":      true,
}

// buildConfig assembles the oxlint config. The React-family plugins
// (react, react-perf, jsx-a11y) are enabled only for actual React projects:
// their rules (e.g. react/no-this-in-sfc) otherwise misfire on plain Node/Express
// code, where any function using `this` is mistaken for a stateless component.
// The Next.js plugin is appended only for real Next.js apps.
func buildConfig(react, next bool) string {
	var plugins []string
	rules := coreRuleOverrides()
	if react {
		plugins = append(plugins, `"react"`, `"react-perf"`, `"jsx-a11y"`, `"react-hooks"`)
		rules = rules + "," + reactRules
	}
	if next {
		plugins = append(plugins, `"nextjs"`)
	}
	return fmt.Sprintf(baseConfig, strings.Join(plugins, ", "), rules)
}

func coreRuleOverrides() string {
	var rules []string
	for _, rule := range coreNoiseRuleList {
		rules = append(rules, `"`+rule+`": "off"`)
	}
	return "\n    " + strings.Join(rules, ",\n    ") + "\n  "
}

// relevantPkgDirs returns the package.json locations worth inspecting: the repo
// root plus every directory on the path from each scanned file up to the root.
func relevantPkgDirs(ctx core.ProjectContext) map[string]bool { return jsproject.RelevantPkgDirs(ctx) }

// isNextProject reports whether the scan target is a Next.js app: a next.config.*
// among the scanned files, or a "next" dependency in any relevant package.json.
func isNextProject(ctx core.ProjectContext) bool { return jsproject.IsNext(ctx) }

// isReactProject reports whether the scan target is a React app. It delegates to
// jsproject so oxlint's plugin gating and the React hint pack share one
// definition.
func isReactProject(ctx core.ProjectContext) bool { return jsproject.IsReact(ctx) }

// pkgHasDep reports whether the package.json at path lists dep among its
// dependencies or devDependencies.
func pkgHasDep(path, dep string) bool { return jsproject.PkgHasDep(path, dep) }

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
		if isCoreNoiseRule(plugin, rule) {
			continue
		}
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

func isCoreNoiseRule(plugin, rule string) bool {
	if plugin != "" && plugin != "eslint" && plugin != "oxc" {
		return false
	}
	return coreNoiseRules[rule]
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
	case "react", "react-hooks", "typescript", "oxc":
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
