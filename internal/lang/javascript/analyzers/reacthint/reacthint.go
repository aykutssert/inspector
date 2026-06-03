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
	detectGodComponent,
	detectRepeatedLiteral,
}

// jsxDetectors need JSX grammar (text/elements) and only run on js/jsx/tsx.
var jsxDetectors = []detector{
	detectEmDashInJSX,
	detectRenderTimeAllocation,
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

const (
	godComponentVeryLargeLines = 160
	godComponentBusyLines      = 100
	godComponentHookThreshold  = 8
	godComponentPropThreshold  = 12
	godComponentBusyHooks      = 5
	godComponentBusyProps      = 8
)

// detectGodComponent flags React components whose size or public prop/hook
// surface has crossed a maintainability threshold. This is intentionally a
// hint: large components can be valid, but they are expensive context for an
// agent and usually hide extractable UI or custom hooks.
func detectGodComponent(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding {
	var out []core.Finding
	_ = runMatches(componentQuery, root, lang, func(caps map[string]*sitter.Node) {
		name := nodeText(caps["name"], src)
		fn := caps["fn"]
		if fn == nil || !isComponentName(name) {
			return
		}
		lines := int(fn.EndPoint().Row-fn.StartPoint().Row) + 1
		hooks := countReactHooks(fn, lang, src)
		props := countComponentProps(fn, src)
		if !isGodComponent(lines, hooks, props) {
			return
		}
		out = append(out, hint(
			"god-component", "quality", core.SeverityInfo, file,
			int(fn.StartPoint().Row)+1,
			name+" is large/complex ("+strconv.Itoa(lines)+" lines, "+strconv.Itoa(hooks)+" hooks, "+strconv.Itoa(props)+" props); this is hard to review and weak context for agents.",
			"Split unrelated UI into child components and move stateful behavior into focused custom hooks.",
		))
	})
	return out
}

func isGodComponent(lines, hooks, props int) bool {
	if lines >= godComponentVeryLargeLines {
		return true
	}
	if hooks >= godComponentHookThreshold || props >= godComponentPropThreshold {
		return true
	}
	return lines >= godComponentBusyLines && (hooks >= godComponentBusyHooks || props >= godComponentBusyProps)
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

func isComponentName(name string) bool {
	return name != "" && name[0] >= 'A' && name[0] <= 'Z'
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

func countReactHooks(node *sitter.Node, lang *sitter.Language, src []byte) int {
	n := 0
	_ = runQuery(setterCallQuery, node, lang, func(_ string, id *sitter.Node) {
		if isHookName(id.Content(src)) {
			n++
		}
	})
	return n
}

func isHookName(name string) bool {
	return len(name) > 3 && strings.HasPrefix(name, "use") && name[3] >= 'A' && name[3] <= 'Z'
}

func countComponentProps(node *sitter.Node, src []byte) int {
	text := node.Content(src)
	open := strings.Index(text, "{")
	if open == -1 {
		return 0
	}
	close := matchingBrace(text, open)
	if close == -1 || !braceLooksLikeParamDestructure(text, close) {
		return 0
	}
	return countTopLevelCommaItems(text[open+1 : close])
}

func matchingBrace(text string, open int) int {
	depth := 0
	for i := open; i < len(text); i++ {
		switch text[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func braceLooksLikeParamDestructure(text string, close int) bool {
	rest := strings.TrimSpace(text[close+1:])
	if strings.HasPrefix(rest, ")") || strings.HasPrefix(rest, "=>") || strings.HasPrefix(rest, ",") {
		return true
	}
	if !strings.HasPrefix(rest, ":") {
		return false
	}
	closeParen := strings.Index(rest, ")")
	body := strings.Index(rest, "{")
	return closeParen != -1 && (body == -1 || closeParen < body)
}

func countTopLevelCommaItems(text string) int {
	count, depth, hasToken := 0, 0, false
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '{', '[', '(':
			depth++
			hasToken = true
		case '}', ']', ')':
			if depth > 0 {
				depth--
			}
			hasToken = true
		case ',':
			if depth == 0 {
				if hasToken {
					count++
				}
				hasToken = false
				continue
			}
			hasToken = true
		case ' ', '\n', '\r', '\t':
		default:
			hasToken = true
		}
	}
	if hasToken {
		count++
	}
	return count
}

const (
	repeatedStringThreshold = 4
	repeatedNumberThreshold = 3
	maxRepeatedLiteralHints = 5
)

const literalQuery = `[(string) (number)] @lit`

type literalStat struct {
	kind      string
	display   string
	firstLine int
	count     int
}

// detectRepeatedLiteral flags repeated magic strings/numbers in one file. The
// signal is grouped per literal to avoid noisy output: a repeated value gets one
// hint at its first occurrence, not one finding per use.
func detectRepeatedLiteral(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding {
	stats := map[string]*literalStat{}
	_ = runQuery(literalQuery, root, lang, func(_ string, node *sitter.Node) {
		if isInsideNodeType(node, "import_statement") || isInsideNodeType(node, "export_statement") {
			return
		}
		kind, value, ok := normalizedLiteral(node, src)
		if !ok {
			return
		}
		key := kind + ":" + value
		stat := stats[key]
		if stat == nil {
			stat = &literalStat{
				kind:      kind,
				display:   value,
				firstLine: int(node.StartPoint().Row) + 1,
			}
			stats[key] = stat
		}
		stat.count++
	})

	out := make([]core.Finding, 0, len(stats))
	for _, stat := range stats {
		if !isRepeatedLiteral(stat) {
			continue
		}
		out = append(out, hint(
			"repeated-magic-literal", "quality", core.SeverityInfo, file, stat.firstLine,
			stat.kind+" literal "+stat.display+" is repeated "+strconv.Itoa(stat.count)+" times in this file; repeated domain values are easy to mistype and hard for agents to safely change.",
			"Extract the value to a named constant, enum, route map, or shared configuration when the repetitions refer to the same concept.",
		))
	}
	sortFindingsByLine(out)
	if len(out) > maxRepeatedLiteralHints {
		out = out[:maxRepeatedLiteralHints]
	}
	return out
}

func normalizedLiteral(node *sitter.Node, src []byte) (kind, value string, ok bool) {
	text := strings.TrimSpace(node.Content(src))
	switch node.Type() {
	case "string":
		value = normalizeStringLiteral(text)
		if !isMagicStringCandidate(value) {
			return "", "", false
		}
		return "string", strconv.Quote(value), true
	case "number":
		value = normalizeNumberLiteral(text)
		if !isMagicNumberCandidate(value) {
			return "", "", false
		}
		return "number", value, true
	default:
		return "", "", false
	}
}

func normalizeStringLiteral(text string) string {
	if len(text) < 2 {
		return text
	}
	quote := text[0]
	if (quote == '"' || quote == '\'') && text[len(text)-1] == quote {
		if unquoted, err := strconv.Unquote(text); err == nil {
			return unquoted
		}
		return text[1 : len(text)-1]
	}
	return text
}

func normalizeNumberLiteral(text string) string {
	return strings.ReplaceAll(strings.ToLower(text), "_", "")
}

func isMagicStringCandidate(value string) bool {
	if len(strings.TrimSpace(value)) < 4 {
		return false
	}
	switch value {
	case "true", "false", "null", "undefined", "use strict":
		return false
	default:
		return true
	}
}

func isMagicNumberCandidate(value string) bool {
	switch value {
	case "", "0", "1", "2", "-1":
		return false
	default:
		return true
	}
}

func isRepeatedLiteral(stat *literalStat) bool {
	switch stat.kind {
	case "string":
		return stat.count >= repeatedStringThreshold
	case "number":
		return stat.count >= repeatedNumberThreshold
	default:
		return false
	}
}

func isInsideNodeType(node *sitter.Node, typ string) bool {
	for parent := node.Parent(); parent != nil; parent = parent.Parent() {
		if parent.Type() == typ {
			return true
		}
	}
	return false
}

func sortFindingsByLine(findings []core.Finding) {
	for i := 1; i < len(findings); i++ {
		for j := i; j > 0 && findings[j].Line < findings[j-1].Line; j-- {
			findings[j], findings[j-1] = findings[j-1], findings[j]
		}
	}
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

const jsxExpressionQuery = `(jsx_expression) @expr`

// detectRenderTimeAllocation flags fresh allocations embedded directly in JSX
// expressions: object literals, regex literals, and new Date(...). These are
// recreated on every render and can defeat memoization or do avoidable work in
// hot render paths. It stays JSX-scoped so ordinary function code is not flagged.
func detectRenderTimeAllocation(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding {
	var out []core.Finding
	seen := map[int]bool{}
	_ = runQuery(jsxExpressionQuery, root, lang, func(_ string, node *sitter.Node) {
		kind := renderAllocationKind(node, src)
		if kind == "" {
			return
		}
		line := int(node.StartPoint().Row) + 1
		if seen[line] {
			return
		}
		seen[line] = true
		out = append(out, hint(
			"render-time-allocation", "performance", core.SeverityInfo, file, line,
			"JSX contains "+kind+" created during render; this work repeats on every render and can break memoization by changing identity.",
			"Move stable values outside the component, or wrap render-dependent values in useMemo when identity matters.",
		))
	})
	return out
}

func renderAllocationKind(node *sitter.Node, src []byte) string {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		ch := node.NamedChild(i)
		switch ch.Type() {
		case "object":
			return "an inline object"
		case "regex":
			return "a regex literal"
		case "new_expression":
			if strings.HasPrefix(strings.TrimSpace(ch.Content(src)), "new Date") {
				return "new Date(...)"
			}
		}
	}
	return ""
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
