package jsquality

import (
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/aykutssert/scout/internal/core"
)

func detectDeferredAwait(root *sitter.Node, _ *sitter.Language, src []byte, file string) []core.Finding {
	var out []core.Finding
	walkQuality(root, func(node *sitter.Node) {
		if node.Type() != "function_declaration" && node.Type() != "arrow_function" && node.Type() != "function_expression" {
			return
		}
		if !isAsyncFunction(node, src) {
			return
		}
		body := node.ChildByFieldName("body")
		if body == nil || body.Type() != "statement_block" {
			return
		}
		for i := 0; i < int(body.NamedChildCount())-1; i++ {
			stmt := body.NamedChild(i)
			nextStmt := body.NamedChild(i + 1)

			awaitVar := extractAwaitAssignment(stmt, src)
			if awaitVar == "" {
				continue
			}
			if !isEarlyReturnGuard(nextStmt) {
				continue
			}
			condition := nextStmt.ChildByFieldName("condition")
			if condition == nil {
				continue
			}
			if referencesIdentifier(condition, src, awaitVar) {
				continue
			}
			line := int(stmt.StartPoint().Row) + 1
			out = append(out, core.Finding{
				Analyzer:   "js-quality",
				RuleID:     "async-defer-await",
				Severity:   core.SeverityInfo,
				Level:      core.SeverityInfo.String(),
				Category:   "performance",
				Confidence: core.ConfidenceHint,
				File:       file,
				Line:       line,
				Message:    "Await before a guard that doesn't depend on the awaited value. If the guard short-circuits, the async work was wasted. Move the guard before the await so unnecessary work is avoided.",
				Fix:        "Restructure: place the early-return guard before the await expression.",
			})
		}
	})
	return out
}

func extractAwaitAssignment(stmt *sitter.Node, src []byte) string {
	var declarator *sitter.Node
	switch stmt.Type() {
	case "lexical_declaration", "variable_declaration":
		if stmt.NamedChildCount() != 1 {
			return ""
		}
		declarator = stmt.NamedChild(0)
		if declarator.Type() != "variable_declarator" {
			return ""
		}
	default:
		return ""
	}
	value := declarator.ChildByFieldName("value")
	if value == nil || value.Type() != "await_expression" {
		return ""
	}
	name := declarator.ChildByFieldName("name")
	if name == nil {
		return ""
	}
	return name.Content(src)
}

func isEarlyReturnGuard(node *sitter.Node) bool {
	if node.Type() != "if_statement" {
		return false
	}
	cons := node.ChildByFieldName("consequence")
	if cons == nil {
		return false
	}
	return hasReturnStmt(cons)
}

func hasReturnStmt(node *sitter.Node) bool {
	if node.Type() == "return_statement" {
		return true
	}
	if node.Type() == "statement_block" {
		for i := 0; i < int(node.NamedChildCount()); i++ {
			if node.NamedChild(i).Type() == "return_statement" {
				return true
			}
		}
	}
	return false
}

func referencesIdentifier(node *sitter.Node, src []byte, name string) bool {
	found := false
	walkQuality(node, func(n *sitter.Node) {
		if !found && n.Type() == "identifier" && n.Content(src) == name {
			found = true
		}
	})
	return found
}
