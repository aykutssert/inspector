package reacthint

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/aykutssert/scout/internal/core"
)

// detectStableEmptyFallback flags inline empty array/object literals used as
// fallbacks (e.g., items || [] or props ?? {}) inside React components or hooks.
// These allocate fresh references on every render, breaking child component
// memoization.
func detectStableEmptyFallback(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding {
	var out []core.Finding
	seenLine := map[int]bool{}

	_ = runMatches(componentQuery, root, lang, func(caps map[string]*sitter.Node) {
		name := nodeText(caps["name"], src)
		fn := caps["fn"]
		if fn == nil || !isComponentOrHook(name) {
			return
		}

		walkReact(fn, func(node *sitter.Node) {
			if node.Type() != "binary_expression" {
				return
			}
			left := node.ChildByFieldName("left")
			right := node.ChildByFieldName("right")
			if left == nil || right == nil {
				return
			}

			// Get the operator
			op := strings.TrimSpace(string(src[left.EndByte():right.StartByte()]))
			if op != "||" && op != "??" {
				return
			}

			// Check if right is empty array or empty object
			isVal := false
			kind := ""
			fallbackSuggestion := ""
			if right.Type() == "array" && right.NamedChildCount() == 0 {
				isVal = true
				kind = "empty array literal"
				fallbackSuggestion = "const EMPTY_ARRAY = [];"
			} else if right.Type() == "object" && right.NamedChildCount() == 0 {
				isVal = true
				kind = "empty object literal"
				fallbackSuggestion = "const EMPTY_OBJECT = {};"
			}

			if isVal {
				line := int(right.StartPoint().Row) + 1
				if seenLine[line] {
					return
				}
				seenLine[line] = true

				out = append(out, hint(
					"prefer-stable-empty-fallback", "performance", core.SeverityInfo, file, line,
					"Use of an inline "+kind+" as a fallback creates a new reference on every render, which can bypass child component memoization.",
					"Define a stable empty fallback variable outside the component (e.g. '"+fallbackSuggestion+"') and use it instead.",
				))
			}
		})
	})
	return out
}
