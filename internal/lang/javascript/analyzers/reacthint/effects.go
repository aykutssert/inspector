package reacthint

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/aykutssert/scout/internal/core"
)

func detectUseEffectMissingCleanup(root *sitter.Node, _ *sitter.Language, src []byte, file string) []core.Finding {
	if isEffectPerformanceTestFile(file) {
		return nil
	}
	var out []core.Finding
	walkReact(root, func(node *sitter.Node) {
		if node.Type() != "call_expression" || calleeName(node.ChildByFieldName("function"), src) != "useEffect" {
			return
		}
		args := node.ChildByFieldName("arguments")
		if args == nil || args.NamedChildCount() == 0 {
			return
		}
		callback := args.NamedChild(0)
		if callback == nil || (callback.Type() != "arrow_function" && callback.Type() != "function_expression") {
			return
		}
		resource := effectResourceKind(callback, src)
		if resource == "" || effectHasCleanupReturn(callback) {
			return
		}
		out = append(out, hint(
			"react-useeffect-missing-cleanup", "bug", core.SeverityWarning, file,
			int(node.StartPoint().Row)+1,
			"useEffect registers "+resource+" but returns no cleanup function; repeated mounts can leak listeners, subscriptions, or timers.",
			"Return a cleanup function that removes the listener, unsubscribes, or clears the interval.",
		))
	})
	return out
}

func effectResourceKind(callback *sitter.Node, src []byte) string {
	kind := ""
	walkReact(callback, func(node *sitter.Node) {
		if kind != "" || node.Type() != "call_expression" || isInNestedFunction(node, callback) {
			return
		}
		fn := node.ChildByFieldName("function")
		switch calleeName(fn, src) {
		case "addEventListener":
			kind = "an event listener"
		case "setInterval":
			kind = "an interval"
		case "subscribe":
			kind = "a subscription"
		}
	})
	return kind
}

func effectHasCleanupReturn(callback *sitter.Node) bool {
	found := false
	walkReact(callback, func(node *sitter.Node) {
		if found || node.Type() != "return_statement" || isInNestedFunction(node, callback) {
			return
		}
		found = node.NamedChildCount() > 0
	})
	return found
}

func isEffectPerformanceTestFile(file string) bool {
	path := "/" + strings.ToLower(strings.ReplaceAll(file, "\\", "/"))
	return strings.Contains(path, "/__tests__/") ||
		strings.Contains(path, "/__testfixtures__/") ||
		strings.Contains(path, "/fixtures/") ||
		strings.Contains(path, ".test.") ||
		strings.Contains(path, ".spec.")
}

func isContextProviderElement(node *sitter.Node, src []byte) bool {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child.Type() == "member_expression" || child.Type() == "nested_identifier" {
			return strings.HasSuffix(nodeText(child, src), ".Provider")
		}
		if child.Type() == "identifier" {
			return false
		}
	}
	return false
}

func isContextProviderValue(node *sitter.Node, src []byte) bool {
	attr := parentOfType(node, "jsx_attribute")
	if attr == nil || jsxAttributeName(attr, src) != "value" {
		return false
	}
	for parent := attr.Parent(); parent != nil; parent = parent.Parent() {
		if parent.Type() == "jsx_opening_element" || parent.Type() == "jsx_self_closing_element" {
			return isContextProviderElement(parent, src)
		}
		if parent.Type() == "jsx_element" {
			return false
		}
	}
	return false
}
