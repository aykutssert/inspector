package reacthint

import (
	"strings"

	"github.com/aykutssert/scout/internal/core"
	sitter "github.com/smacker/go-tree-sitter"
)

type compPropInfo struct {
	name          string
	fnNode        *sitter.Node
	receivedProps map[string]bool
	passes        []jsxPass
	line          int
}

type jsxPass struct {
	childComp string
	propName  string
}

func detectDeepPropDrilling(root *sitter.Node, lang *sitter.Language, src []byte, file string, externalMemoized map[string]bool) []core.Finding {
	// Find all components in the file
	components := make(map[string]*compPropInfo)
	_ = runMatches(componentQuery, root, lang, func(caps map[string]*sitter.Node) {
		name := nodeText(caps["name"], src)
		fn := caps["fn"]
		if fn == nil || !isComponentName(name) {
			return
		}

		received := collectProps(fn, src, lang)
		passes := collectJSXPasses(fn, src, received)

		components[name] = &compPropInfo{
			name:          name,
			fnNode:        fn,
			receivedProps: received,
			passes:        passes,
			line:          int(fn.StartPoint().Row) + 1,
		}
	})

	// Track which components are drilling nodes for which props
	// drillingNodes[compName][propName] = childName
	drillingNodes := make(map[string]map[string]string)
	for name, info := range components {
		drillingNodes[name] = make(map[string]string)
		for _, pass := range info.passes {
			isUsed := isPropUsedLocally(info.fnNode, pass.propName, src)
			// A prop is drilled if it is received, passed, and not used locally
			if !isUsed {
				drillingNodes[name][pass.propName] = pass.childComp
			}
		}
	}

	var out []core.Finding
	// Trace chains: CompB -(prop)-> CompC -(prop)-> CompD
	// where CompB and CompC are both drilling nodes in the file
	for bName, bProps := range drillingNodes {
		for prop, cName := range bProps {
			cProps, cExists := drillingNodes[cName]
			if cExists {
				if dName, cDrills := cProps[prop]; cDrills {
					bInfo := components[bName]
					msg := "Prop '" + prop + "' is drilled through multiple components (" + bName + " -> " + cName + " -> " + dName + ") without being used locally in intermediate components."

					out = append(out, hint(
						"deep-prop-drilling", "quality", core.SeverityWarning, file, bInfo.line,
						msg,
						"Consider using React Context API or Component Composition to share data instead of prop drilling.",
					))
				}
			}
		}
	}

	return out
}

func firstParameter(fnNode *sitter.Node) *sitter.Node {
	var params *sitter.Node
	for i := 0; i < int(fnNode.ChildCount()); i++ {
		ch := fnNode.Child(i)
		if ch.Type() == "formal_parameters" {
			params = ch
			break
		}
	}
	if params == nil || params.ChildCount() == 0 {
		return nil
	}
	for i := 0; i < int(params.ChildCount()); i++ {
		ch := params.Child(i)
		if ch.Type() != "(" && ch.Type() != ")" && ch.Type() != "," {
			return ch
		}
	}
	return nil
}

func unwrapParameter(node *sitter.Node) *sitter.Node {
	if node == nil {
		return nil
	}
	for node.Type() == "required_parameter" || node.Type() == "optional_parameter" || node.Type() == "assignment_pattern" {
		if node.ChildCount() > 0 {
			node = node.Child(0)
		} else {
			break
		}
	}
	return node
}

func collectParamProps(param *sitter.Node, src []byte, props map[string]bool) {
	if param == nil {
		return
	}
	if param.Type() == "identifier" || param.Type() == "shorthand_property_identifier_pattern" || param.Type() == "shorthand_property_identifier" {
		props[string(param.Content(src))] = true
		return
	}
	if param.Type() == "object_pattern" {
		for i := 0; i < int(param.ChildCount()); i++ {
			child := param.Child(i)
			if child.Type() == "shorthand_property_identifier_pattern" || child.Type() == "shorthand_property_identifier" {
				props[string(child.Content(src))] = true
			} else if child.Type() == "pair_pattern" || child.Type() == "pair" {
				if child.ChildCount() >= 3 {
					collectParamProps(child.Child(2), src, props)
				}
			}
		}
	}
}

func collectProps(fnNode *sitter.Node, src []byte, lang *sitter.Language) map[string]bool {
	props := make(map[string]bool)
	param := unwrapParameter(firstParameter(fnNode))
	if param == nil {
		return props
	}
	if param.Type() == "object_pattern" {
		collectParamProps(param, src, props)
	} else if param.Type() == "identifier" {
		paramName := string(param.Content(src))
		walkReact(fnNode, func(n *sitter.Node) {
			if n.Type() == "member_expression" {
				obj := n.ChildByFieldName("object")
				prop := n.ChildByFieldName("property")
				if obj != nil && prop != nil && obj.Type() == "identifier" && string(obj.Content(src)) == paramName {
					props[string(prop.Content(src))] = true
				}
			} else if n.Type() == "variable_declarator" {
				val := n.ChildByFieldName("value")
				if val != nil && val.Type() == "identifier" && string(val.Content(src)) == paramName {
					namePat := n.ChildByFieldName("name")
					if namePat != nil && namePat.Type() == "object_pattern" {
						collectParamProps(namePat, src, props)
					}
				}
			}
		})
	}
	return props
}

func collectJSXPasses(fnNode *sitter.Node, src []byte, receivedProps map[string]bool) []jsxPass {
	var passes []jsxPass

	walkReact(fnNode, func(n *sitter.Node) {
		var nameNode *sitter.Node
		if n.Type() == "jsx_opening_element" || n.Type() == "jsx_self_closing_element" {
			nameNode = n.ChildByFieldName("name")
		} else {
			return
		}

		if nameNode == nil {
			return
		}
		childName := string(nameNode.Content(src))
		if !isComponentName(childName) {
			return
		}

		for i := 0; i < int(n.ChildCount()); i++ {
			child := n.Child(i)
			if child.Type() == "jsx_attribute" {
				valNode := firstNamedChildOfType(child, "jsx_expression")
				if valNode != nil {
					var idNode *sitter.Node
					walkReact(valNode, func(cx *sitter.Node) {
						if cx.Type() == "identifier" {
							idNode = cx
						}
					})
					if idNode != nil {
						idName := string(idNode.Content(src))
						if receivedProps[idName] {
							passes = append(passes, jsxPass{childComp: childName, propName: idName})
						}
					}
				}
			} else if child.Type() == "jsx_spread_attribute" {
				var exprNode *sitter.Node
				for j := 0; j < int(child.ChildCount()); j++ {
					ch := child.Child(j)
					if ch.Type() != "{" && ch.Type() != "}" && ch.Type() != "..." {
						exprNode = ch
						break
					}
				}
				if exprNode != nil && exprNode.Type() == "identifier" {
					exprName := string(exprNode.Content(src))
					if receivedProps[exprName] {
						passes = append(passes, jsxPass{childComp: childName, propName: exprName})
					}
				}
			}
		}
	})

	return passes
}

func isPropUsedLocally(fnNode *sitter.Node, prop string, src []byte) bool {
	var occurrences []*sitter.Node
	walkReact(fnNode, func(n *sitter.Node) {
		if n.Type() == "identifier" && string(n.Content(src)) == prop {
			occurrences = append(occurrences, n)
		}
	})

	localUses := 0
	for _, occ := range occurrences {
		if isParameterNode(occ, fnNode) {
			continue
		}
		if isDeclarationOfProp(occ, prop, src) {
			continue
		}
		if isJSXPassValue(occ, src) {
			continue
		}
		localUses++
	}

	return localUses > 0
}

func isParameterNode(occ, fnNode *sitter.Node) bool {
	params := firstParameter(fnNode)
	if params == nil {
		return false
	}
	for p := occ.Parent(); p != nil; p = p.Parent() {
		if p == params {
			return true
		}
	}
	return false
}

func isDeclarationOfProp(occ *sitter.Node, prop string, src []byte) bool {
	decl := parentOfType(occ, "variable_declarator")
	if decl == nil {
		return false
	}
	namePat := decl.ChildByFieldName("name")
	if namePat == nil {
		return false
	}
	for p := occ; p != nil && p != decl; p = p.Parent() {
		if p == namePat {
			return true
		}
	}
	return false
}

func isJSXPassValue(occ *sitter.Node, src []byte) bool {
	var jsxAttr *sitter.Node
	for p := occ.Parent(); p != nil; p = p.Parent() {
		t := p.Type()
		if t == "jsx_attribute" || t == "jsx_spread_attribute" {
			jsxAttr = p
			break
		}
		if isFunctionNode(p) || strings.HasSuffix(t, "_statement") || strings.HasSuffix(t, "_declaration") {
			break
		}
	}
	if jsxAttr == nil {
		return false
	}

	el := jsxAttr.Parent()
	if el == nil {
		return false
	}
	nameNode := el.ChildByFieldName("name")
	if nameNode == nil {
		return false
	}
	return isComponentName(string(nameNode.Content(src)))
}
