package jsquality

import (
	sitter "github.com/smacker/go-tree-sitter"

	"github.com/aykutssert/scout/internal/core"
)

var domReadProps = map[string]bool{
	"offsetHeight":          true,
	"offsetWidth":           true,
	"clientHeight":          true,
	"clientWidth":           true,
	"scrollTop":             true,
	"scrollLeft":            true,
	"scrollHeight":          true,
	"scrollWidth":           true,
	"getBoundingClientRect": true,
}

var domWriteProps = map[string]bool{
	"style":       true,
	"className":   true,
	"textContent": true,
	"innerHTML":   true,
	"innerText":   true,
}

var domWriteClassListMethods = map[string]bool{
	"add":    true,
	"remove": true,
	"toggle": true,
}

func detectBatchDOMCss(root *sitter.Node, _ *sitter.Language, src []byte, file string) []core.Finding {
	if isPerformanceTestFile(file) {
		return nil
	}
	var out []core.Finding
	walkQuality(root, func(n *sitter.Node) {
		switch n.Type() {
		case "for_statement", "for_in_statement", "while_statement", "do_statement":
			body := n.ChildByFieldName("body")
			if body == nil {
				return
			}
			var stmts []*sitter.Node
			if body.Type() == "statement_block" {
				for i := 0; i < int(body.NamedChildCount()); i++ {
					stmts = append(stmts, body.NamedChild(i))
				}
			} else {
				stmts = append(stmts, body)
			}
			for i := 1; i < len(stmts); i++ {
				if hasDOMWrite(stmts[i-1], src) && hasDOMRead(stmts[i], src) {
					line := int(n.StartPoint().Row) + 1
					out = append(out, core.Finding{
						Analyzer:   "js-quality",
						RuleID:     "js-batch-dom-css",
						Severity:   core.SeverityInfo,
						Level:      core.SeverityInfo.String(),
						Category:   "performance",
						Confidence: core.ConfidenceHint,
						File:       file,
						Line:       line,
						Message:    "DOM write followed by DOM read in the same loop — forces synchronous layout. Batch DOM reads and writes separately.",
						Fix:        "Group all DOM reads first, then apply all DOM writes. This avoids layout thrashing (forced reflow).",
					})
					break
				}
			}
		}
	})
	return out
}

func hasDOMWrite(node *sitter.Node, src []byte) bool {
	if node == nil {
		return false
	}
	// Assignment to .style.*, .className, .textContent, .innerHTML, .innerText
	// Recursively checks the left chain so `el.style.height = x` is caught
	if node.Type() == "assignment_expression" {
		if isDOMWriteTarget(node.ChildByFieldName("left"), src) {
			return true
		}
	}
	// .classList.add/remove/toggle calls
	if node.Type() == "call_expression" {
		fn := node.ChildByFieldName("function")
		if fn != nil && fn.Type() == "member_expression" {
			obj := fn.ChildByFieldName("object")
			prop := fn.ChildByFieldName("property")
			if obj != nil && prop != nil && obj.Type() == "member_expression" {
				classListProp := obj.ChildByFieldName("property")
				if classListProp != nil && classListProp.Content(src) == "classList" && domWriteClassListMethods[prop.Content(src)] {
					return true
				}
			}
		}
	}
	for i := 0; i < int(node.NamedChildCount()); i++ {
		if hasDOMWrite(node.NamedChild(i), src) {
			return true
		}
	}
	return false
}

// isDOMWriteTarget checks if a member_expression chain ends in a DOM-write property
// like `.style`, `.className`, `.textContent`. For `el.style.height`, property is `height`
// but the chain contains `style` — so we check recursively through the object chain.
func isDOMWriteTarget(node *sitter.Node, src []byte) bool {
	if node == nil || node.Type() != "member_expression" {
		return false
	}
	prop := node.ChildByFieldName("property")
	if prop != nil && domWriteProps[prop.Content(src)] {
		return true
	}
	return isDOMWriteTarget(node.ChildByFieldName("object"), src)
}

func hasDOMRead(node *sitter.Node, src []byte) bool {
	if node == nil {
		return false
	}
	if node.Type() == "member_expression" {
		prop := node.ChildByFieldName("property")
		if prop != nil && domReadProps[prop.Content(src)] {
			return true
		}
	}
	if node.Type() == "call_expression" {
		fn := node.ChildByFieldName("function")
		if fn != nil && fn.Type() == "identifier" && fn.Content(src) == "getComputedStyle" {
			return true
		}
	}
	for i := 0; i < int(node.NamedChildCount()); i++ {
		if hasDOMRead(node.NamedChild(i), src) {
			return true
		}
	}
	return false
}
