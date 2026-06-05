package reacthint

import (
	"strconv"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/aykutssert/scout/internal/core"
)

func detectMemoizedChildUnstableProp(root *sitter.Node, _ *sitter.Language, src []byte, file string, externalMemoized map[string]bool) []core.Finding {
	memoized := memoizedComponents(root, src, externalMemoized)
	if len(memoized) == 0 {
		return nil
	}
	var out []core.Finding
	seen := map[string]bool{}
	walkReact(root, func(node *sitter.Node) {
		if node.Type() != "jsx_self_closing_element" && node.Type() != "jsx_opening_element" {
			return
		}
		component := jsxElementName(node, src)
		if !memoized[component] {
			return
		}
		var unstable []string
		for i := 0; i < int(node.NamedChildCount()); i++ {
			attr := node.NamedChild(i)
			if attr.Type() != "jsx_attribute" {
				continue
			}
			prop := jsxAttributeName(attr, src)
			expr := firstNamedChildOfType(attr, "jsx_expression")
			if prop == "" || expr == nil {
				continue
			}
			if kind := unstablePropKind(expr, src); kind != "" {
				unstable = append(unstable, prop+" ("+kind+")")
			}
		}
		if len(unstable) == 0 {
			return
		}
		line := int(node.StartPoint().Row) + 1
		key := file + ":" + strconv.Itoa(line) + ":" + component
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, hint(
			"memoized-child-unstable-prop", "performance", core.SeverityInfo, file, line,
			component+" is memoized but receives unstable prop values: "+strings.Join(unstable, ", ")+". These values get a new identity on every render and can defeat React.memo.",
			"Pass stable values: move constants outside the component, or wrap render-dependent objects/functions in useMemo/useCallback.",
		))
	})
	sortFindingsByLine(out)
	return out
}

func memoizedComponents(root *sitter.Node, src []byte, external map[string]bool) map[string]bool {
	out := map[string]bool{}
	for name := range external {
		out[name] = true
	}
	walkReact(root, func(node *sitter.Node) {
		if node.Type() != "variable_declarator" {
			return
		}
		nameNode := node.ChildByFieldName("name")
		valueNode := node.ChildByFieldName("value")
		if nameNode == nil || valueNode == nil || nameNode.Type() != "identifier" || valueNode.Type() != "call_expression" {
			return
		}
		if isReactMemoCall(valueNode, src) {
			out[nodeText(nameNode, src)] = true
		}
	})
	return out
}

func isReactMemoCall(call *sitter.Node, src []byte) bool {
	fn := call.ChildByFieldName("function")
	if fn == nil {
		return false
	}
	switch fn.Type() {
	case "identifier":
		return nodeText(fn, src) == "memo"
	case "member_expression":
		return fieldNodeText(fn, "property", src) == "memo"
	default:
		return false
	}
}

func isMemoizedChildPropExpression(expr *sitter.Node, memoized map[string]bool, src []byte) bool {
	attr := parentOfType(expr, "jsx_attribute")
	if attr == nil {
		return false
	}
	for p := attr.Parent(); p != nil; p = p.Parent() {
		if p.Type() == "jsx_self_closing_element" || p.Type() == "jsx_opening_element" {
			return memoized[jsxElementName(p, src)] && unstablePropKind(expr, src) != ""
		}
		if p.Type() == "jsx_element" {
			return false
		}
	}
	return false
}

func unstablePropKind(expr *sitter.Node, src []byte) string {
	value := firstExpressionValue(expr)
	if value == nil {
		return ""
	}
	switch value.Type() {
	case "arrow_function", "function_expression":
		return "inline function"
	case "object":
		return "inline object"
	case "array":
		return "inline array"
	case "new_expression":
		return "new object"
	case "call_expression":
		if isBindCall(value, src) {
			return "bound function"
		}
	case "identifier":
		return localUnstableIdentifierKind(value, src)
	}
	return ""
}

func firstExpressionValue(expr *sitter.Node) *sitter.Node {
	for i := 0; i < int(expr.NamedChildCount()); i++ {
		ch := expr.NamedChild(i)
		if ch.Type() != "comment" {
			return ch
		}
	}
	return nil
}

func localUnstableIdentifierKind(id *sitter.Node, src []byte) string {
	name := nodeText(id, src)
	fn := enclosingFunction(id)
	if fn == nil {
		return ""
	}
	init := localVariableInitializer(fn, name, id.StartByte(), src)
	if init == nil || stableHookInitializer(init, src) {
		return ""
	}
	switch init.Type() {
	case "object":
		return "local object"
	case "array":
		return "local array"
	case "arrow_function", "function_expression":
		return "local function"
	case "new_expression":
		return "local new object"
	case "call_expression":
		if isBindCall(init, src) {
			return "local bound function"
		}
	}
	return ""
}

func localVariableInitializer(scope *sitter.Node, name string, before uint32, src []byte) *sitter.Node {
	var out *sitter.Node
	walkReact(scope, func(node *sitter.Node) {
		if out != nil || node.StartByte() >= before || node.Type() != "variable_declarator" {
			return
		}
		if isInNestedFunction(node, scope) {
			return
		}
		nameNode := node.ChildByFieldName("name")
		if nameNode == nil || nodeText(nameNode, src) != name {
			return
		}
		out = node.ChildByFieldName("value")
	})
	return out
}

func stableHookInitializer(init *sitter.Node, src []byte) bool {
	if init == nil || init.Type() != "call_expression" {
		return false
	}
	fn := init.ChildByFieldName("function")
	name := calleeName(fn, src)
	return name == "useMemo" || name == "useCallback" || name == "useRef"
}

func isBindCall(call *sitter.Node, src []byte) bool {
	fn := call.ChildByFieldName("function")
	return fn != nil && fn.Type() == "member_expression" && fieldNodeText(fn, "property", src) == "bind"
}

func jsxElementName(node *sitter.Node, src []byte) string {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		ch := node.NamedChild(i)
		switch ch.Type() {
		case "identifier", "member_expression":
			return lastIdentifierText(ch, src)
		default:
			return ""
		}
	}
	return ""
}

func jsxAttributeName(attr *sitter.Node, src []byte) string {
	for i := 0; i < int(attr.NamedChildCount()); i++ {
		ch := attr.NamedChild(i)
		if ch.Type() == "property_identifier" || ch.Type() == "identifier" {
			return nodeText(ch, src)
		}
	}
	return ""
}
