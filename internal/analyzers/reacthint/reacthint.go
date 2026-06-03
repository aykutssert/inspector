package reacthint

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/typescript/tsx"
	"github.com/smacker/go-tree-sitter/typescript/typescript"

	"github.com/aykutssert/inspector/internal/core"
)

// Analyzer finds React structural smells that lint rules miss — anti-patterns
// that are usually-but-not-always wrong. Every finding is a hint: the agent
// confirms it against context rather than treating it as a defect. This is the
// layer on top of oxlint, not a replacement for it.
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

var detectors = []detector{
	detectStateFromProp,
	detectSetStateInEffectNoDeps,
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

const stateFromPropQuery = `
(call_expression
  function: (identifier) @fn
  arguments: (arguments (member_expression) @arg)) @call
`

// detectStateFromProp flags useState(props.x) / useState(obj.field). State
// seeded from a prop or object field does not update when that source changes —
// a frequent React bug. A bare literal/identifier initializer is fine and not
// flagged, to keep noise down.
func detectStateFromProp(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding {
	var out []core.Finding
	_ = runMatches(stateFromPropQuery, root, lang, func(caps map[string]*sitter.Node) {
		if nodeText(caps["fn"], src) != "useState" {
			return
		}
		call := caps["call"]
		if call == nil {
			return
		}
		out = append(out, core.Finding{
			Analyzer:   "react-hint",
			RuleID:     "state-initialized-from-prop",
			Severity:   core.SeverityWarning,
			Level:      core.SeverityWarning.String(),
			Category:   "bug",
			Confidence: core.ConfidenceHint,
			File:       file,
			Line:       int(call.StartPoint().Row) + 1,
			Message:    "useState is seeded from a prop or object field; the state won't update when that source changes.",
			Fix:        "Derive the value during render, or sync it deliberately when the prop changes.",
		})
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
// argument, no second arg) ensure the dependency array is absent.
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
		out = append(out, core.Finding{
			Analyzer:   "react-hint",
			RuleID:     "setstate-in-effect-without-deps",
			Severity:   core.SeverityWarning,
			Level:      core.SeverityWarning.String(),
			Category:   "bug",
			Confidence: core.ConfidenceHint,
			File:       file,
			Line:       int(call.StartPoint().Row) + 1,
			Message:    "useEffect has no dependency array and calls a state setter; it runs after every render and can loop.",
			Fix:        "Add a dependency array, or move the state update out of the effect.",
		})
	}
	_ = runMatches(effectArrowQuery, root, lang, check)
	_ = runMatches(effectFuncQuery, root, lang, check)
	return out
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
