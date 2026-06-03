package reacthint

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/typescript/tsx"
	"github.com/smacker/go-tree-sitter/typescript/typescript"

	"github.com/aykutssert/inspector/internal/core"
)

// Analyzer finds React structural and design smells that lint rules miss —
// anti-patterns that are usually-but-not-always wrong, plus consistency and
// AI-slop signals. Every finding is a hint: the agent confirms it against
// context rather than treating it as a defect. This is the layer on top of
// oxlint, not a replacement for it.
type Analyzer struct{}

func New() *Analyzer { return &Analyzer{} }

var _ core.Analyzer = (*Analyzer)(nil)

func (a *Analyzer) Name() string { return "react-hint" }

// Available is always true: this analyzer is pure Go (tree-sitter), no external
// binary to install.
func (a *Analyzer) Available() bool { return true }

const (
	maxFileBytes = 1 << 20 // 1 MiB; skip larger files instead of parsing
	parseTimeout = 5 * time.Second
)

var jsExt = map[string]bool{
	".js": true, ".jsx": true, ".ts": true, ".tsx": true,
	".mjs": true, ".cjs": true, ".mts": true, ".cts": true,
}

func (a *Analyzer) Scan(ctx core.ProjectContext) ([]core.Finding, error) {
	var findings []core.Finding
	for _, rel := range ctx.Files {
		if !jsExt[strings.ToLower(filepath.Ext(rel))] {
			continue
		}
		// A parse failure here is not an analyzer failure — semgrep already
		// surfaces syntax errors. Skip the file and keep scanning.
		fs, err := scanFile(filepath.Join(ctx.Root, rel), rel)
		if err != nil {
			continue
		}
		findings = append(findings, fs...)
	}
	return findings, nil
}

// detector inspects a parsed file and returns any hints it finds.
type detector func(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding

// detectors run on every JS/TS file (no JSX nodes required).
var detectors = []detector{
	detectDerivedState,
	detectSetStateInEffectNoDeps,
	detectPreferUseReducer,
}

// jsxDetectors need JSX grammar (text/elements) and only run on js/jsx/tsx.
var jsxDetectors = []detector{
	detectEmDashInJSX,
	detectRedundantTailwindSize,
}

func scanFile(abs, rel string) ([]core.Finding, error) {
	if info, err := os.Stat(abs); err == nil && info.Size() > maxFileBytes {
		return nil, nil
	}
	src, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	if len(src) > maxFileBytes {
		return nil, nil
	}
	lang := langForPath(abs)
	parser := sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(lang)
	pctx, cancel := context.WithTimeout(context.Background(), parseTimeout)
	defer cancel()
	tree, err := parser.ParseCtx(pctx, nil, src)
	if err != nil {
		return nil, err
	}
	defer tree.Close()
	root := tree.RootNode()

	var findings []core.Finding
	for _, d := range detectors {
		findings = append(findings, d(root, lang, src, rel)...)
	}
	if jsxCapable(abs) {
		for _, d := range jsxDetectors {
			findings = append(findings, d(root, lang, src, rel)...)
		}
	}
	return findings, nil
}

// langForPath selects the grammar by extension. The JS grammar cannot parse TS
// type syntax, so .ts/.tsx route to their own grammars.
func langForPath(path string) *sitter.Language {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ts", ".mts", ".cts":
		return typescript.GetLanguage()
	case ".tsx":
		return tsx.GetLanguage()
	default:
		return javascript.GetLanguage()
	}
}

// jsxCapable reports whether the grammar for path understands JSX nodes. The
// plain TypeScript grammar does not, so a JSX query fails to compile there.
func jsxCapable(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ts", ".mts", ".cts":
		return false
	default:
		return true
	}
}

func hint(rule, cat string, sev core.Severity, file string, line int, msg, fix string) core.Finding {
	return core.Finding{
		Analyzer:   "react-hint",
		RuleID:     rule,
		Severity:   sev,
		Level:      sev.String(),
		Category:   cat,
		Confidence: core.ConfidenceHint,
		File:       file,
		Line:       line,
		Message:    msg,
		Fix:        fix,
	}
}

const derivedStateQuery = `
(call_expression
  function: (identifier) @fn
  arguments: (arguments [(member_expression) (call_expression)] @arg)) @call
`

// detectDerivedState flags useState seeded from a prop/object field or another
// computed value (useState(props.x), useState(compute(x))). Such state won't
// update when its source changes — a frequent React bug. Literal initializers
// (useState(0)) and lazy initializers (useState(() => ...)) are not matched, so
// the legitimate patterns stay quiet.
func detectDerivedState(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding {
	var out []core.Finding
	_ = runMatches(derivedStateQuery, root, lang, func(caps map[string]*sitter.Node) {
		if nodeText(caps["fn"], src) != "useState" {
			return
		}
		call := caps["call"]
		if call == nil {
			return
		}
		out = append(out, hint(
			"no-derived-state", "bug", core.SeverityWarning, file,
			int(call.StartPoint().Row)+1,
			"useState is seeded from a prop or computed value; the state won't update when that source changes.",
			"Compute the value during render (or useMemo), or sync it deliberately when the source changes.",
		))
	})
	return out
}

const effectArrowQuery = `
(call_expression
  function: (identifier) @fn
  arguments: (arguments . (arrow_function) @cb .)) @call
`

const effectFuncQuery = `
(call_expression
  function: (identifier) @fn
  arguments: (arguments . (function_expression) @cb .)) @call
`

const setterCallQuery = `(call_expression function: (identifier) @id)`

var setterRe = regexp.MustCompile(`^set[A-Z]`)

// detectSetStateInEffectNoDeps flags useEffect(fn) with no dependency array
// whose body calls a setX setter. That runs after every render and re-triggers
// itself — a classic infinite-loop smell. The anchors in the query (single
// argument) ensure the dependency array is absent.
func detectSetStateInEffectNoDeps(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding {
	var out []core.Finding
	check := func(caps map[string]*sitter.Node) {
		if nodeText(caps["fn"], src) != "useEffect" {
			return
		}
		cb, call := caps["cb"], caps["call"]
		if cb == nil || call == nil || !callsSetter(cb, lang, src) {
			return
		}
		out = append(out, hint(
			"setstate-in-effect-without-deps", "bug", core.SeverityWarning, file,
			int(call.StartPoint().Row)+1,
			"useEffect has no dependency array and calls a state setter; it runs after every render and can loop.",
			"Add a dependency array, or move the state update out of the effect.",
		))
	}
	_ = runMatches(effectArrowQuery, root, lang, check)
	_ = runMatches(effectFuncQuery, root, lang, check)
	return out
}

const componentQuery = `
(function_declaration name: (identifier) @name) @fn
(variable_declarator name: (identifier) @name value: (arrow_function)) @fn
(variable_declarator name: (identifier) @name value: (function_expression)) @fn
`

// useReducerThreshold is the number of useState hooks in one component/hook
// above which we suggest consolidating with useReducer.
const useReducerThreshold = 4

// detectPreferUseReducer flags a component or hook that declares many useState
// hooks; that state is usually related and easier to manage with useReducer.
func detectPreferUseReducer(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding {
	var out []core.Finding
	_ = runMatches(componentQuery, root, lang, func(caps map[string]*sitter.Node) {
		name := nodeText(caps["name"], src)
		fn := caps["fn"]
		if fn == nil || !isComponentOrHook(name) {
			return
		}
		if countUseState(fn, lang, src) < useReducerThreshold {
			return
		}
		out = append(out, hint(
			"prefer-use-reducer", "quality", core.SeverityInfo, file,
			int(fn.StartPoint().Row)+1,
			name+" declares "+strconv.Itoa(countUseState(fn, lang, src))+" useState hooks; related state is often clearer with useReducer.",
			"Consolidate related state into a single useReducer.",
		))
	})
	return out
}

// isComponentOrHook matches React naming: a component is capitalized, a hook is
// prefixed with "use". Avoids flagging plain helpers that happen to call hooks.
func isComponentOrHook(name string) bool {
	if name == "" {
		return false
	}
	if name[0] >= 'A' && name[0] <= 'Z' {
		return true
	}
	return strings.HasPrefix(name, "use")
}

func countUseState(node *sitter.Node, lang *sitter.Language, src []byte) int {
	n := 0
	_ = runQuery(setterCallQuery, node, lang, func(_ string, id *sitter.Node) {
		if id.Content(src) == "useState" {
			n++
		}
	})
	return n
}

const jsxTextQuery = `(jsx_text) @t`

// detectEmDashInJSX flags an em dash (—) in rendered JSX text. It is a common
// tell of AI-generated copy and an inconsistency in most codebases. A hint, not
// a defect: the author may have wanted it.
func detectEmDashInJSX(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding {
	var out []core.Finding
	seen := map[int]bool{}
	_ = runQuery(jsxTextQuery, root, lang, func(_ string, node *sitter.Node) {
		if !strings.Contains(node.Content(src), "—") {
			return
		}
		line := int(node.StartPoint().Row) + 1
		if seen[line] {
			return
		}
		seen[line] = true
		out = append(out, hint(
			"em-dash-in-jsx-text", "quality", core.SeverityInfo, file, line,
			"Em dash (—) in rendered text; often an AI-generated copy tell and a typographic inconsistency.",
			"Use a regular hyphen or rewrite the sentence if it was not intentional.",
		))
	})
	return out
}

const classNameQuery = `(jsx_attribute (property_identifier) @attr (string) @val)`

var (
	twWidth  = regexp.MustCompile(`(?:^|\s)w-(\S+)`)
	twHeight = regexp.MustCompile(`(?:^|\s)h-(\S+)`)
)

// detectRedundantTailwindSize flags a className with matching w-N and h-N
// utilities (e.g. "w-4 h-4"), which Tailwind expresses more consistently as
// size-N. A design-consistency hint.
func detectRedundantTailwindSize(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding {
	var out []core.Finding
	_ = runMatches(classNameQuery, root, lang, func(caps map[string]*sitter.Node) {
		attr := nodeText(caps["attr"], src)
		if attr != "className" && attr != "class" {
			return
		}
		val := caps["val"]
		if val == nil {
			return
		}
		classes := trimQuotes(val.Content(src))
		if sz, ok := redundantSizeAxis(classes); ok {
			out = append(out, hint(
				"redundant-tailwind-size-axes", "quality", core.SeverityInfo, file,
				int(val.StartPoint().Row)+1,
				"Tailwind classes set w-"+sz+" and h-"+sz+" together; size-"+sz+" is the consistent form.",
				"Replace w-"+sz+" h-"+sz+" with size-"+sz+".",
			))
		}
	})
	return out
}

// redundantSizeAxis returns a width suffix that also appears as a height suffix,
// meaning the two axes are equal and collapsible to size-N.
func redundantSizeAxis(classes string) (string, bool) {
	heights := map[string]bool{}
	for _, m := range twHeight.FindAllStringSubmatch(classes, -1) {
		heights[m[1]] = true
	}
	for _, m := range twWidth.FindAllStringSubmatch(classes, -1) {
		if heights[m[1]] {
			return m[1], true
		}
	}
	return "", false
}

// callsSetter reports whether the subtree calls an identifier that looks like a
// React state setter (setX). Used to gate the depsless-effect hint.
func callsSetter(node *sitter.Node, lang *sitter.Language, src []byte) bool {
	found := false
	_ = runQuery(setterCallQuery, node, lang, func(_ string, id *sitter.Node) {
		if setterRe.MatchString(id.Content(src)) {
			found = true
		}
	})
	return found
}

func runQuery(q string, root *sitter.Node, lang *sitter.Language, fn func(name string, node *sitter.Node)) error {
	query, err := sitter.NewQuery([]byte(q), lang)
	if err != nil {
		return err
	}
	defer query.Close()
	qc := sitter.NewQueryCursor()
	defer qc.Close()
	qc.Exec(query, root)
	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}
		for _, c := range m.Captures {
			fn(query.CaptureNameForId(c.Index), c.Node)
		}
	}
	return nil
}

func runMatches(q string, root *sitter.Node, lang *sitter.Language, fn func(caps map[string]*sitter.Node)) error {
	query, err := sitter.NewQuery([]byte(q), lang)
	if err != nil {
		return err
	}
	defer query.Close()
	qc := sitter.NewQueryCursor()
	defer qc.Close()
	qc.Exec(query, root)
	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}
		caps := map[string]*sitter.Node{}
		for _, c := range m.Captures {
			caps[query.CaptureNameForId(c.Index)] = c.Node
		}
		fn(caps)
	}
	return nil
}

func nodeText(node *sitter.Node, src []byte) string {
	if node == nil {
		return ""
	}
	return node.Content(src)
}

func trimQuotes(s string) string {
	return strings.Trim(s, "'\"`")
}
