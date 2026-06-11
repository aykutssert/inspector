package taintflow

import (
	sitter "github.com/smacker/go-tree-sitter"
)

// taintInfo records why a variable is considered tainted: the source
// expression it was assigned from (e.g. "req.body.role") and the line where
// that assignment happened, so findings can show the full chain.
type taintInfo struct {
	source string
	line   int
}

// sourceRoots are identifiers whose request-derived properties are treated
// as attacker-controlled. "ctx" covers Koa/tRPC-style context objects.
var sourceRoots = map[string]bool{
	"req":     true,
	"request": true,
	"ctx":     true,
}

// sourceProps are the properties on a source root that carry
// attacker-controlled data.
var sourceProps = map[string]bool{
	"body":    true,
	"query":   true,
	"params":  true,
	"headers": true,
	"cookies": true,
}

// sourceExprDesc reports whether node is (or is a property access rooted in)
// a request-derived source expression, e.g. `req.body`, `req.body.role`,
// `ctx.query.id`. It returns the full expression text for use in messages.
func sourceExprDesc(node *sitter.Node, src []byte) (string, bool) {
	if node == nil || node.Type() != "member_expression" {
		return "", false
	}
	obj := node.ChildByFieldName("object")
	prop := node.ChildByFieldName("property")
	if obj == nil || prop == nil {
		return "", false
	}
	if obj.Type() == "identifier" && sourceRoots[obj.Content(src)] && sourceProps[prop.Content(src)] {
		return node.Content(src), true
	}
	if _, ok := sourceExprDesc(obj, src); ok {
		return node.Content(src), true
	}
	return "", false
}

// taintedRoot reports whether node's value is derived from a tainted
// variable: the identifier itself, a property access on it
// (`tainted.field`), a spread of it (`{ ...tainted }`, `[...tainted]`), a
// string built from it (`"cmd " + tainted`, `` `cmd ${tainted}` ``), or a
// parenthesized form of any of these. It returns the name of the tainted
// variable found.
//
// Object literals that merely *reference* a tainted field by name
// (`{ email }`, `{ email: filter.email }`) are deliberately not matched:
// picking a single named field out of a tainted source is the recommended
// fix, not the vulnerability, even though the field's value is still
// attacker-controlled (a separate, much noisier class of finding).
func taintedRoot(node *sitter.Node, src []byte, tainted map[string]taintInfo) (string, bool) {
	if node == nil {
		return "", false
	}
	switch node.Type() {
	case "identifier":
		name := node.Content(src)
		if _, ok := tainted[name]; ok {
			return name, true
		}
	case "member_expression":
		return taintedRoot(node.ChildByFieldName("object"), src, tainted)
	case "parenthesized_expression":
		if node.NamedChildCount() > 0 {
			return taintedRoot(node.NamedChild(0), src, tainted)
		}
	case "binary_expression":
		if name, ok := taintedRoot(node.ChildByFieldName("left"), src, tainted); ok {
			return name, true
		}
		return taintedRoot(node.ChildByFieldName("right"), src, tainted)
	case "template_string":
		for i := 0; i < int(node.NamedChildCount()); i++ {
			ch := node.NamedChild(i)
			if ch.Type() == "template_substitution" && ch.NamedChildCount() > 0 {
				if name, ok := taintedRoot(ch.NamedChild(0), src, tainted); ok {
					return name, true
				}
			}
		}
	case "object", "array":
		for i := 0; i < int(node.NamedChildCount()); i++ {
			ch := node.NamedChild(i)
			if ch.Type() == "spread_element" && ch.NamedChildCount() > 0 {
				if name, ok := taintedRoot(ch.NamedChild(0), src, tainted); ok {
					return name, true
				}
			}
		}
	}
	return "", false
}

// collectDeclarator marks the names bound by `const/let/var <pattern> = <value>`
// as tainted when value is a source expression or already-tainted variable.
func collectDeclarator(node *sitter.Node, src []byte, tainted map[string]taintInfo) {
	name := node.ChildByFieldName("name")
	value := node.ChildByFieldName("value")
	if name == nil || value == nil {
		return
	}
	info, ok := taintInfoFor(value, src, tainted)
	if !ok {
		return
	}
	bindNames(name, src, info, tainted)
}

// collectAssignment marks `x = <value>` as tainted the same way, for plain
// identifier targets (re-binding an existing variable).
func collectAssignment(node *sitter.Node, src []byte, tainted map[string]taintInfo) {
	left := node.ChildByFieldName("left")
	value := node.ChildByFieldName("right")
	if left == nil || value == nil || left.Type() != "identifier" {
		return
	}
	info, ok := taintInfoFor(value, src, tainted)
	if !ok {
		return
	}
	tainted[left.Content(src)] = info
}

// taintInfoFor builds the taintInfo for a value expression, either because
// it is itself a source expression or because it derives from an
// already-tainted variable.
func taintInfoFor(value *sitter.Node, src []byte, tainted map[string]taintInfo) (taintInfo, bool) {
	if desc, ok := sourceExprDesc(value, src); ok {
		return taintInfo{source: desc, line: int(value.StartPoint().Row) + 1}, true
	}
	if root, ok := taintedRoot(value, src, tainted); ok {
		return tainted[root], true
	}
	return taintInfo{}, false
}

// bindNames marks every identifier bound by a declarator's name pattern
// (plain identifier, object pattern, array pattern, with one level of
// destructuring and rest elements).
func bindNames(name *sitter.Node, src []byte, info taintInfo, tainted map[string]taintInfo) {
	switch name.Type() {
	case "identifier":
		tainted[name.Content(src)] = info
	case "object_pattern":
		for i := 0; i < int(name.NamedChildCount()); i++ {
			ch := name.NamedChild(i)
			switch ch.Type() {
			case "shorthand_property_identifier_pattern":
				tainted[ch.Content(src)] = info
			case "pair_pattern":
				if v := ch.ChildByFieldName("value"); v != nil && v.Type() == "identifier" {
					tainted[v.Content(src)] = info
				}
			case "rest_pattern":
				if ch.NamedChildCount() > 0 && ch.NamedChild(0).Type() == "identifier" {
					tainted[ch.NamedChild(0).Content(src)] = info
				}
			}
		}
	case "array_pattern":
		for i := 0; i < int(name.NamedChildCount()); i++ {
			ch := name.NamedChild(i)
			if ch.Type() == "identifier" {
				tainted[ch.Content(src)] = info
			}
		}
	}
}
