package nesthint

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

func walk(n *sitter.Node, fn func(*sitter.Node)) {
	if n == nil {
		return
	}
	fn(n)
	for i := 0; i < int(n.NamedChildCount()); i++ {
		walk(n.NamedChild(i), fn)
	}
}

func firstChildOfType(n *sitter.Node, typ string) *sitter.Node {
	var found *sitter.Node
	walk(n, func(c *sitter.Node) {
		if found == nil && c.Type() == typ {
			found = c
		}
	})
	return found
}

// firstDirectChildOfType returns the first direct named child of n with the
// given type, without recursive descent — so callers read the node actually
// passed at this level (e.g. the argument of a call) rather than a nested match.
func firstDirectChildOfType(n *sitter.Node, typ string) *sitter.Node {
	if n == nil {
		return nil
	}
	for i := 0; i < int(n.NamedChildCount()); i++ {
		if c := n.NamedChild(i); c.Type() == typ {
			return c
		}
	}
	return nil
}

// bareIdentifierName returns the identifier text when node is a plain identifier
// (optionally wrapped in parentheses), else "". It is the strict reader for a
// forwardRef(() => Module) body: anything richer than a bare name is ambiguous.
func bareIdentifierName(node *sitter.Node, src []byte) string {
	for node != nil && node.Type() == "parenthesized_expression" && node.NamedChildCount() > 0 {
		node = node.NamedChild(0)
	}
	if node == nil {
		return ""
	}
	switch node.Type() {
	case "identifier", "type_identifier":
		return text(node, src)
	}
	return ""
}

func functionReturnIdentifierName(fn *sitter.Node, src []byte) string {
	body := firstDirectChildOfType(fn, "statement_block")
	if body == nil {
		return ""
	}
	for i := 0; i < int(body.NamedChildCount()); i++ {
		stmt := body.NamedChild(i)
		if stmt.Type() != "return_statement" {
			continue
		}
		for j := 0; j < int(stmt.NamedChildCount()); j++ {
			if name := bareIdentifierName(stmt.NamedChild(j), src); name != "" {
				return name
			}
		}
		return ""
	}
	return ""
}

func lastChildOfTypeText(n *sitter.Node, typ string, src []byte) string {
	var out string
	walk(n, func(c *sitter.Node) {
		if c.Type() == typ {
			out = text(c, src)
		}
	})
	return out
}

func text(n *sitter.Node, src []byte) string {
	if n == nil {
		return ""
	}
	return string(src[n.StartByte():n.EndByte()])
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
func line(n *sitter.Node) int {
	return int(n.StartPoint().Row) + 1
}
