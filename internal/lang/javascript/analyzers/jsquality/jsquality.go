// Package jsquality holds language-level (framework-agnostic) JS/TS hints.
//
// It exists to separate signals that apply to ANY JavaScript/TypeScript code
// from the React-shaped smells in the reacthint pack. reacthint only runs when
// a React/Next signal is present (see reactPack.Detect); putting a universal
// smell like repeated magic literals there would silently mute it on plain
// Node/backend code. This analyzer is wired into the JavaScript pack, so it
// runs on every JS/TS project regardless of framework.
package jsquality

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/typescript/tsx"
	"github.com/smacker/go-tree-sitter/typescript/typescript"

	"github.com/aykutssert/scout/internal/architecture/duplication"
	"github.com/aykutssert/scout/internal/core"
	"github.com/aykutssert/scout/internal/lang/javascript/jsproject"
)

const (
	maxFileBytes = 1 << 20 // 1 MiB; skip larger files instead of parsing
	parseTimeout = 5 * time.Second
)

var jsExt = map[string]bool{
	".js": true, ".jsx": true, ".ts": true, ".tsx": true,
	".mjs": true, ".cjs": true, ".mts": true, ".cts": true,
}

// Analyzer reports framework-agnostic JS/TS quality hints. It is pure Go
// (tree-sitter) with no external binary, so it is always available.
type Analyzer struct{}

func New() *Analyzer { return &Analyzer{} }

var _ core.Analyzer = (*Analyzer)(nil)

func (a *Analyzer) Name() string { return "js-quality" }

func (a *Analyzer) Available() bool { return true }

func (a *Analyzer) Scan(ctx core.ProjectContext) ([]core.Finding, error) {
	var findings []core.Finding
	for _, rel := range ctx.Files {
		if !jsExt[strings.ToLower(filepath.Ext(rel))] {
			continue
		}
		// Quality/design smells (repeated literals, god-class, large function) are
		// noise in test/example/fixture code: repeated values and long bodies are
		// idiomatic there. Skip these files to keep the actionable rate high.
		if jsproject.IsTestOrExampleFile(rel) {
			continue
		}
		// A parse failure here is not an analyzer failure — other tools already
		// surface syntax errors. Skip the file and keep scanning.
		fs, err := scanFile(filepath.Join(ctx.Root, rel), rel)
		if err != nil {
			continue
		}
		findings = append(findings, fs...)
	}
	return findings, nil
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
	var findings []core.Finding
	findings = append(findings, detectRepeatedLiteral(tree.RootNode(), lang, src, rel)...)
	findings = append(findings, detectComplexity(tree.RootNode(), lang, src, rel)...)
	if isTypeScriptFile(abs) {
		findings = append(findings, detectNonNullAssertionSpam(tree.RootNode(), lang, src, rel)...)
	}
	findings = append(findings, detectSequentialAwaits(tree.RootNode(), lang, src, rel)...)
	findings = append(findings, detectPerformancePatterns(tree.RootNode(), lang, src, rel)...)
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

func runQuery(q string, root *sitter.Node, lang *sitter.Language, fn func(node *sitter.Node)) {
	query, err := sitter.NewQuery([]byte(q), lang)
	if err != nil {
		return
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
			fn(c.Node)
		}
	}
}

const (
	repeatedStringThreshold = 4
	repeatedNumberThreshold = 3
	maxRepeatedLiteralHints = 5
)

const literalQuery = `[(string) (number)] @lit`

// detectRepeatedLiteral flags repeated magic strings/numbers in one file. The
// signal is grouped per literal to avoid noisy output: a repeated value gets one
// hint at its first occurrence, not one finding per use.
func detectRepeatedLiteral(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding {
	var lits []duplication.Literal
	runQuery(literalQuery, root, lang, func(node *sitter.Node) {
		if isInsideNodeType(node, "import_statement") || isInsideNodeType(node, "export_statement") {
			return
		}
		kind, value, ok := normalizedLiteral(node, src)
		if !ok {
			return
		}
		lits = append(lits, duplication.Literal{
			Value: value,
			Kind:  kind,
			Line:  int(node.StartPoint().Row) + 1,
		})
	})

	rules := []duplication.Rule{
		{
			ID:              "repeated-magic-literal",
			ThresholdString: repeatedStringThreshold,
			ThresholdNumber: repeatedNumberThreshold,
			MaxViolations:   maxRepeatedLiteralHints,
		},
	}

	violations := duplication.Analyze(file, lits, rules)

	var out []core.Finding
	for _, v := range violations {
		out = append(out, hint(
			v.RuleID, "quality", core.SeverityInfo, v.File, v.Line,
			v.Message,
			"Extract the value to a named constant, enum, route map, or shared configuration when the repetitions refer to the same concept.",
		))
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

func isInsideNodeType(node *sitter.Node, typ string) bool {
	for parent := node.Parent(); parent != nil; parent = parent.Parent() {
		if parent.Type() == typ {
			return true
		}
	}
	return false
}

func hint(rule, cat string, sev core.Severity, file string, line int, msg, fix string) core.Finding {
	return core.Finding{
		Analyzer:   "js-quality",
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

func isTypeScriptFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ts", ".tsx", ".mts", ".cts":
		return true
	}
	return false
}

const nonNullAssertionQuery = `(non_null_expression) @non_null`

func detectNonNullAssertionSpam(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding {
	var occurrences []*sitter.Node
	runQuery(nonNullAssertionQuery, root, lang, func(node *sitter.Node) {
		occurrences = append(occurrences, node)
	})

	if len(occurrences) > 10 {
		first := occurrences[0]
		line := int(first.StartPoint().Row) + 1
		msg := "This file contains an excessive number of non-null assertions (" + strconv.Itoa(len(occurrences)) + "). " +
			"Spamming the non-null assertion operator ('!') bypasses TypeScript's type-safety guarantees and increases the risk of runtime crashes if a value is null or undefined."

		return []core.Finding{hint(
			"non-null-assertion-spam", "quality", core.SeverityWarning, file, line,
			msg,
			"Use optional chaining ('?.'), nullish coalescing ('??'), or explicit runtime checks to handle potentially null or undefined values safely.",
		)}
	}
	return nil
}

func findBlockStatement(node *sitter.Node) (*sitter.Node, *sitter.Node) {
	curr := node
	for curr != nil {
		parent := curr.Parent()
		if parent != nil {
			pt := parent.Type()
			if pt == "statement_block" || pt == "program" {
				return curr, parent
			}
		}
		curr = parent
	}
	return nil, nil
}

const awaitQuery = `(await_expression) @await`

func detectSequentialAwaits(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding {
	groups := make(map[uint32][]*sitter.Node)
	seenStmts := make(map[uint32]bool)

	runQuery(awaitQuery, root, lang, func(node *sitter.Node) {
		stmt, parent := findBlockStatement(node)
		if stmt == nil || parent == nil {
			return
		}
		stmtKey := stmt.StartByte()
		if seenStmts[stmtKey] {
			return
		}
		seenStmts[stmtKey] = true

		parentKey := parent.StartByte()
		groups[parentKey] = append(groups[parentKey], stmt)
	})

	var out []core.Finding
	for _, stmts := range groups {
		sort.Slice(stmts, func(i, j int) bool {
			return stmts[i].StartByte() < stmts[j].StartByte()
		})

		for i := 0; i < len(stmts)-1; i++ {
			stmt1 := stmts[i]
			stmt2 := stmts[i+1]

			if !areConsecutiveStatements(stmt1, stmt2) {
				continue
			}

			declaredNames := make(map[string]bool)
			collectDeclaredNames(stmt1, src, declaredNames)

			if len(declaredNames) > 0 && referencesAny(stmt2, src, declaredNames) {
				continue
			}

			line := int(stmt2.StartPoint().Row) + 1
			out = append(out, hint(
				"sequential-awaits-independent", "performance", core.SeverityWarning, file, line,
				"Consecutive awaits are independent. These asynchronous operations can be parallelized with Promise.all to reduce latency.",
				"Consider using Promise.all to run these operations in parallel, e.g. const [a, b] = await Promise.all([foo(), bar()]).",
			))
		}
	}
	return out
}

func areConsecutiveStatements(stmt1, stmt2 *sitter.Node) bool {
	parent := stmt1.Parent()
	if parent == nil || parent != stmt2.Parent() {
		return false
	}

	idx1 := -1
	idx2 := -1
	for k := 0; k < int(parent.ChildCount()); k++ {
		child := parent.Child(k)
		if child.StartByte() == stmt1.StartByte() && child.EndByte() == stmt1.EndByte() {
			idx1 = k
		}
		if child.StartByte() == stmt2.StartByte() && child.EndByte() == stmt2.EndByte() {
			idx2 = k
		}
	}

	if idx1 == -1 || idx2 == -1 || idx1 >= idx2 {
		return false
	}

	for k := idx1 + 1; k < idx2; k++ {
		child := parent.Child(k)
		if isStatement(child) {
			return false
		}
	}
	return true
}

func isStatement(node *sitter.Node) bool {
	t := node.Type()
	return strings.HasSuffix(t, "_statement") || strings.HasSuffix(t, "_declaration")
}

func collectDeclaredNames(node *sitter.Node, src []byte, names map[string]bool) {
	if node == nil {
		return
	}

	t := node.Type()
	if t == "lexical_declaration" || t == "variable_declaration" {
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child.Type() == "variable_declarator" {
				if child.ChildCount() > 0 {
					collectIdentifiers(child.Child(0), src, names)
				}
			}
		}
	}
}

func collectIdentifiers(node *sitter.Node, src []byte, names map[string]bool) {
	if node == nil {
		return
	}
	if node.Type() == "identifier" {
		names[string(node.Content(src))] = true
		return
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		collectIdentifiers(node.Child(i), src, names)
	}
}

func referencesAny(node *sitter.Node, src []byte, names map[string]bool) bool {
	if node == nil {
		return false
	}
	if node.Type() == "identifier" {
		if names[string(node.Content(src))] {
			return true
		}
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		if referencesAny(node.Child(i), src, names) {
			return true
		}
	}
	return false
}
