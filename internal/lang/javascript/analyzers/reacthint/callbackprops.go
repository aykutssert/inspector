package reacthint

import (
	"regexp"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/aykutssert/inspector/internal/core"
)

// callbackPropRe matches prop names that carry callback semantics by
// convention: event handlers (onClick, onSubmit) and bound handlers
// (handleSave). These props are functions; the component is expected to invoke
// them or forward them to a child, not render them as values.
var callbackPropRe = regexp.MustCompile(`^(on[A-Z]|handle[A-Z])`)

// refKind classifies how a callback prop reference is used.
type refKind int

const (
	// refForwarded: called, passed as an argument/handler, aliased, or otherwise
	// handed off. The callback contract is satisfied; never flag.
	refForwarded refKind = iota
	// refValueMisuse: used where a function makes no sense — rendered as a JSX
	// child, interpolated into a template, or an arithmetic operand. Strong bug.
	refValueMisuse
	// refNeutral: ambiguous (truthiness guard, comparison, ternary condition).
	// Could be a deliberate derived value, so it neither proves nor disproves
	// the contract.
	refNeutral
)

// propBinding records a callback prop: its declared name (the contract) and the
// local identifier it binds to (what the body references).
type propBinding struct {
	name    string
	binding string
}

// detectCallbackPropContract flags a callback-semantic prop (onX/handleX) that a
// component uses only as a value — rendered, interpolated, or in arithmetic —
// and never calls or forwards. That breaks the prop's implicit function
// contract: a handler that is read instead of invoked is almost always a bug
// (forgotten call, or the wrong prop). Forwarding the prop to a child
// (onClick={onClick}) or passing it to a function counts as honoring the
// contract, so legitimate pass-through stays quiet.
func detectCallbackPropContract(root *sitter.Node, lang *sitter.Language, src []byte, file string, _ map[string]bool) []core.Finding {
	var out []core.Finding
	_ = runMatches(componentQuery, root, lang, func(caps map[string]*sitter.Node) {
		name := nodeText(caps["name"], src)
		fn := funcNode(caps["fn"])
		if fn == nil || !isComponentName(name) {
			return
		}
		params := fn.ChildByFieldName("parameters")
		if params == nil {
			return
		}
		pattern := firstDescendant(params, "object_pattern")
		if pattern == nil {
			return
		}
		for _, pb := range callbackBindings(pattern, src) {
			if misuse := firstValueMisuse(fn, params, pb.binding, src); misuse != nil {
				out = append(out, hint(
					"callback-prop-not-invoked", "bug", core.SeverityWarning, file,
					int(misuse.StartPoint().Row)+1,
					pb.name+" is a callback prop but is used as a value here and is never called or passed as a handler in "+name+".",
					"Invoke it ("+pb.name+"(...)) or forward it to a child (e.g. onClick={"+pb.name+"}); don't render or interpolate the function itself.",
				))
			}
		}
	})
	return out
}

// funcNode unwraps a component capture to its function node. componentQuery
// captures either the function directly or the variable_declarator holding an
// arrow/function expression.
func funcNode(fn *sitter.Node) *sitter.Node {
	if fn == nil {
		return nil
	}
	if fn.Type() == "variable_declarator" {
		return fn.ChildByFieldName("value")
	}
	return fn
}

// callbackBindings returns the callback-named props destructured in pattern,
// pairing each declared prop name with the local identifier it binds.
func callbackBindings(pattern *sitter.Node, src []byte) []propBinding {
	var out []propBinding
	for i := 0; i < int(pattern.NamedChildCount()); i++ {
		ch := pattern.NamedChild(i)
		switch ch.Type() {
		case "shorthand_property_identifier_pattern":
			name := nodeText(ch, src)
			if callbackPropRe.MatchString(name) {
				out = append(out, propBinding{name: name, binding: name})
			}
		case "object_assignment_pattern":
			// { onClick = noop }: left holds the shorthand pattern.
			left := ch.ChildByFieldName("left")
			if left != nil && left.Type() == "shorthand_property_identifier_pattern" {
				name := nodeText(left, src)
				if callbackPropRe.MatchString(name) {
					out = append(out, propBinding{name: name, binding: name})
				}
			}
		case "pair_pattern":
			// { onClick: handler }: contract is the key, body uses the value.
			key := nodeText(ch.ChildByFieldName("key"), src)
			val := ch.ChildByFieldName("value")
			if val != nil && val.Type() == "identifier" && callbackPropRe.MatchString(key) {
				out = append(out, propBinding{name: key, binding: nodeText(val, src)})
			}
		}
	}
	return out
}

// firstValueMisuse returns the first reference to binding inside scope that
// misuses it as a value, but only if no reference forwards or invokes it. A
// single forwarding/invoking use anywhere means the contract is honored and the
// prop is not flagged. References inside the declaration (params) are ignored.
func firstValueMisuse(scope, params *sitter.Node, binding string, src []byte) *sitter.Node {
	var misuse *sitter.Node
	forwarded := false
	walkReact(scope, func(n *sitter.Node) {
		if n.Type() != "identifier" || nodeText(n, src) != binding {
			return
		}
		if isWithin(n, params) {
			return
		}
		switch classifyRef(n, src) {
		case refForwarded:
			forwarded = true
		case refValueMisuse:
			if misuse == nil {
				misuse = n
			}
		}
	})
	if forwarded {
		return nil
	}
	return misuse
}

func firstDescendant(node *sitter.Node, typ string) *sitter.Node {
	var found *sitter.Node
	walkReact(node, func(n *sitter.Node) {
		if found == nil && n.Type() == typ {
			found = n
		}
	})
	return found
}

func isWithin(node, scope *sitter.Node) bool {
	if scope == nil {
		return false
	}
	for p := node; p != nil; p = p.Parent() {
		if p == scope {
			return true
		}
	}
	return false
}

// classifyRef decides how a single callback-prop reference is used.
func classifyRef(id *sitter.Node, src []byte) refKind {
	p := id.Parent()
	if p == nil {
		return refNeutral
	}
	switch p.Type() {
	case "call_expression":
		// onClick() — the callee — or an unusual direct child; either way a call.
		return refForwarded
	case "arguments":
		return refForwarded // foo(onClick): forwarded into another call
	case "member_expression":
		return refForwarded // onClick.call / onClick.bind / aliasing
	case "variable_declarator", "assignment_expression", "pair", "spread_element",
		"return_statement", "array", "augmented_assignment_expression":
		return refForwarded // aliased, stored, or handed off
	case "jsx_expression":
		gp := p.Parent()
		if gp != nil && gp.Type() == "jsx_attribute" {
			return refForwarded // onClick={onClick}: forwarded to a child
		}
		if soleExpression(p, id) {
			return refValueMisuse // {onClick} rendered as a JSX child
		}
		return refNeutral
	case "template_substitution":
		return refValueMisuse // `${onClick}` interpolated
	case "binary_expression":
		if isArithmeticBinary(p, src) {
			return refValueMisuse // onClick + x: arithmetic/string use
		}
		return refNeutral // comparisons, && / || guards
	default:
		return refNeutral
	}
}

// soleExpression reports whether container's only named child is id, i.e. the
// reference stands alone (not wrapped in a guard, ternary, or call).
func soleExpression(container, id *sitter.Node) bool {
	return container.NamedChildCount() == 1 && container.NamedChild(0) == id
}

// isArithmeticBinary reports whether a binary_expression uses an arithmetic or
// string-concatenation operator, which is never valid on a function.
func isArithmeticBinary(node *sitter.Node, src []byte) bool {
	left := node.ChildByFieldName("left")
	right := node.ChildByFieldName("right")
	if left == nil || right == nil {
		return false
	}
	op := strings.TrimSpace(string(src[left.EndByte():right.StartByte()]))
	switch op {
	case "+", "-", "*", "/", "%", "**":
		return true
	default:
		return false
	}
}
