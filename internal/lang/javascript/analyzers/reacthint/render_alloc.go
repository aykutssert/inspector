package reacthint

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/aykutssert/inspector/internal/core"
)

const jsxExpressionQuery = `(jsx_expression) @expr`

// detectRenderTimeAllocation flags fresh allocations embedded directly in JSX
// expressions: object literals, regex literals, and new Date(...). These are
// recreated on every render and can defeat memoization or do avoidable work in
// hot render paths. It stays JSX-scoped so ordinary function code is not flagged.
func detectRenderTimeAllocation(root *sitter.Node, lang *sitter.Language, src []byte, file string, externalMemoized map[string]bool) []core.Finding {
	var out []core.Finding
	seen := map[int]bool{}
	memoized := memoizedComponents(root, src, externalMemoized)
	_ = runQuery(jsxExpressionQuery, root, lang, func(_ string, node *sitter.Node) {
		kind := renderAllocationKind(node, src)
		if kind == "" {
			return
		}
		// dangerouslySetInnerHTML={{ __html: ... }} requires that object wrapper
		// by the React API — it is not an avoidable allocation. The real concern
		// on this prop is XSS, which the security layer (semgrep) reports. Flagging
		// it here is a wrong-level signal that buries the security finding.
		if isDangerouslySetInnerHTMLValue(node, src) {
			return
		}
		if isMemoizedChildPropExpression(node, memoized, src) {
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

// isDangerouslySetInnerHTMLValue reports whether the jsx_expression is the value
// of a dangerouslySetInnerHTML attribute (jsx_attribute > property_identifier +
// jsx_expression).
func isDangerouslySetInnerHTMLValue(node *sitter.Node, src []byte) bool {
	parent := node.Parent()
	if parent == nil || parent.Type() != "jsx_attribute" {
		return false
	}
	name := parent.NamedChild(0)
	return name != nil && name.Content(src) == "dangerouslySetInnerHTML"
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
