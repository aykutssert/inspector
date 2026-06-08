package reacthint

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/aykutssert/scout/internal/core"
)

// detectNoRenderInRender flags component invocations or helper render functions
// called directly inside JSX expressions (e.g. {Child()} or {renderHeader()})
// rather than being returned or rendered as proper JSX elements (e.g. <Child />).
func detectNoRenderInRender(root *sitter.Node, lang *sitter.Language, src []byte, file string, _ map[string]bool) []core.Finding {
	var out []core.Finding
	seenLine := map[int]bool{}

	_ = runQuery(jsxExpressionQuery, root, lang, func(_ string, exprNode *sitter.Node) {
		walkReact(exprNode, func(node *sitter.Node) {
			if node.Type() != "call_expression" {
				return
			}
			funcNode := node.ChildByFieldName("function")
			name := calleeName(funcNode, src)
			if name == "" {
				return
			}

			if isNoRenderInRenderViolation(name) {
				line := int(node.StartPoint().Row) + 1
				if seenLine[line] {
					return
				}
				seenLine[line] = true

				out = append(out, hint(
					"no-render-in-render", "performance", core.SeverityWarning, file, line,
					"Component or JSX helper '"+name+"()' is invoked directly as a function call inside JSX. This bypasses React's reconciliation, state/hook lifecycle, and performance optimizations.",
					"Render it as a JSX element (e.g., <"+name+" />) or split it into a proper component.",
				))
			}
		})
	})
	return out
}

func isNoRenderInRenderViolation(name string) bool {
	if name == "" {
		return false
	}
	// Case 1: Capitalized name (React Component call)
	if name[0] >= 'A' && name[0] <= 'Z' {
		// Ignore common built-in constructor names
		switch name {
		case "Boolean", "String", "Number", "Object", "Array", "Date", "RegExp", "Error", "Symbol", "Promise", "Map", "Set", "Intl":
			return false
		}
		return true
	}
	// Case 2: Render helper name (e.g. renderHeader, renderItem, render)
	if strings.HasPrefix(name, "render") {
		if len(name) == 6 {
			return true // "render"
		}
		// renderHeader -> 'H' is uppercase
		if name[6] >= 'A' && name[6] <= 'Z' {
			return true
		}
	}
	return false
}
