// Package taintflow performs intra-procedural taint tracking for JavaScript
// and TypeScript: it marks local variables assigned from request-derived
// sources (req.body, req.query, req.params, req.headers, req.cookies) and
// flags when those variables flow untransformed into security-sensitive
// sinks (database queries, mass-assignment, command/code execution, outbound
// fetches) within the same function.
//
// This complements the single-line semgrep rules, which only catch
// req.body passed directly as a call argument. The common real-world shape
// is one level removed: `const filter = req.body; ... ; Model.find(filter);`
// — variable indirection that a pattern matcher cannot see across
// statements.
package taintflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/typescript/tsx"
	"github.com/smacker/go-tree-sitter/typescript/typescript"

	"github.com/aykutssert/scout/internal/core"
)

const (
	maxFileBytes = 1 << 20
	parseTimeout = 5 * time.Second
)

type Analyzer struct{}

func New() *Analyzer { return &Analyzer{} }

var _ core.Analyzer = (*Analyzer)(nil)

func (a *Analyzer) Name() string { return "taint-flow" }

func (a *Analyzer) Available() bool { return true }

func (a *Analyzer) Scan(ctx core.ProjectContext) ([]core.Finding, error) {
	var findings []core.Finding
	for _, rel := range ctx.Files {
		switch strings.ToLower(filepath.Ext(rel)) {
		case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs", ".mts", ".cts":
		default:
			continue
		}
		fs, err := scanFile(filepath.Join(ctx.Root, rel), rel)
		if err != nil {
			continue
		}
		findings = append(findings, fs...)
	}
	return findings, nil
}

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

	parser := sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(langForPath(abs))
	pctx, cancel := context.WithTimeout(context.Background(), parseTimeout)
	defer cancel()
	tree, err := parser.ParseCtx(pctx, nil, src)
	if err != nil {
		return nil, err
	}
	defer tree.Close()

	var findings []core.Finding
	for _, fn := range functionBodies(tree.RootNode()) {
		findings = append(findings, analyzeFunction(fn, src, rel)...)
	}
	return findings, nil
}

var functionNodeTypes = map[string]bool{
	"function_declaration": true,
	"function_expression":  true,
	"arrow_function":       true,
	"method_definition":    true,
}

// functionBodies returns the statement_block body of every function-like
// node in the tree (declarations, expressions, arrows, methods), at any
// nesting depth. Arrow functions with an expression body (no braces) are
// skipped: they cannot contain variable declarations or sink calls beyond
// the single expression, so there is nothing for this analyzer to track.
func functionBodies(root *sitter.Node) []*sitter.Node {
	var out []*sitter.Node
	walkAll(root, func(node *sitter.Node) {
		if !functionNodeTypes[node.Type()] {
			return
		}
		body := node.ChildByFieldName("body")
		if body != nil && body.Type() == "statement_block" {
			out = append(out, body)
		}
	})
	return out
}

// analyzeFunction runs the two-pass taint analysis over a single function
// body: collect locally-tainted variables, then look for sink calls (in this
// function and any nested closures) that consume a tainted variable.
func analyzeFunction(body *sitter.Node, src []byte, file string) []core.Finding {
	tainted := map[string]taintInfo{}
	walkScope(body, func(node *sitter.Node) {
		switch node.Type() {
		case "variable_declarator":
			collectDeclarator(node, src, tainted)
		case "assignment_expression":
			collectAssignment(node, src, tainted)
		}
	})
	if len(tainted) == 0 {
		return nil
	}

	var findings []core.Finding
	walkAll(body, func(node *sitter.Node) {
		if node.Type() != "call_expression" {
			return
		}
		findings = append(findings, checkSink(node, src, file, tainted)...)
	})
	return findings
}

// walkAll visits node and every descendant, regardless of type.
func walkAll(node *sitter.Node, fn func(*sitter.Node)) {
	if node == nil {
		return
	}
	fn(node)
	for i := 0; i < int(node.NamedChildCount()); i++ {
		walkAll(node.NamedChild(i), fn)
	}
}

// walkScope is like walkAll but does not descend into nested function-like
// nodes, so declarations inside a closure are not attributed to the
// enclosing function's scope.
func walkScope(node *sitter.Node, fn func(*sitter.Node)) {
	if node == nil {
		return
	}
	fn(node)
	for i := 0; i < int(node.NamedChildCount()); i++ {
		ch := node.NamedChild(i)
		if functionNodeTypes[ch.Type()] {
			continue
		}
		walkScope(ch, fn)
	}
}
