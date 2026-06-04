package reacthint

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

	"github.com/aykutssert/inspector/internal/core"
)

const (
	maxFileBytes = 1 << 20 // 1 MiB; skip larger files instead of parsing
	parseTimeout = 5 * time.Second
)

var jsExt = map[string]bool{
	".js": true, ".jsx": true, ".ts": true, ".tsx": true,
	".mjs": true, ".cjs": true, ".mts": true, ".cts": true,
}

func scanFile(abs, rel string) ([]core.Finding, error) {
	return scanFileWithExternalMemoized(abs, rel, nil)
}

func scanFileWithExternalMemoized(abs, rel string, externalMemoized map[string]bool) ([]core.Finding, error) {
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
			findings = append(findings, d(root, lang, src, rel, externalMemoized)...)
		}
	}
	return findings, nil
}

func reactFiles(files []string) []string {
	var out []string
	for _, rel := range files {
		if jsExt[strings.ToLower(filepath.Ext(rel))] {
			out = append(out, rel)
		}
	}
	return out
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

func walkReact(node *sitter.Node, fn func(*sitter.Node)) {
	if node == nil {
		return
	}
	fn(node)
	for i := 0; i < int(node.NamedChildCount()); i++ {
		walkReact(node.NamedChild(i), fn)
	}
}

func firstNamedChildOfType(node *sitter.Node, typ string) *sitter.Node {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		ch := node.NamedChild(i)
		if ch.Type() == typ {
			return ch
		}
	}
	return nil
}

func parentOfType(node *sitter.Node, typ string) *sitter.Node {
	for p := node.Parent(); p != nil; p = p.Parent() {
		if p.Type() == typ {
			return p
		}
	}
	return nil
}

func enclosingFunction(node *sitter.Node) *sitter.Node {
	for p := node.Parent(); p != nil; p = p.Parent() {
		if isFunctionNode(p) {
			return p
		}
	}
	return nil
}

func isInNestedFunction(node, scope *sitter.Node) bool {
	for p := node.Parent(); p != nil && p != scope; p = p.Parent() {
		if isFunctionNode(p) {
			return true
		}
	}
	return false
}

func isFunctionNode(node *sitter.Node) bool {
	switch node.Type() {
	case "function_declaration", "function_expression", "arrow_function", "method_definition":
		return true
	default:
		return false
	}
}

func fieldNodeText(node *sitter.Node, field string, src []byte) string {
	if node == nil {
		return ""
	}
	return nodeText(node.ChildByFieldName(field), src)
}

func calleeName(node *sitter.Node, src []byte) string {
	if node == nil {
		return ""
	}
	switch node.Type() {
	case "identifier":
		return nodeText(node, src)
	case "member_expression":
		return fieldNodeText(node, "property", src)
	default:
		return ""
	}
}

func lastIdentifierText(node *sitter.Node, src []byte) string {
	var out string
	walkReact(node, func(n *sitter.Node) {
		if n.Type() == "identifier" || n.Type() == "property_identifier" {
			out = nodeText(n, src)
		}
	})
	return out
}

func lastChildText(node *sitter.Node, typ string, src []byte) string {
	var out string
	walkReact(node, func(n *sitter.Node) {
		if n.Type() == typ {
			out = nodeText(n, src)
		}
	})
	return out
}

func directIdentifiers(node *sitter.Node, src []byte) []string {
	var out []string
	for i := 0; i < int(node.NamedChildCount()); i++ {
		ch := node.NamedChild(i)
		if ch.Type() == "identifier" {
			out = append(out, nodeText(ch, src))
		}
	}
	return out
}

func unquoteReact(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
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
