package rnhint

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/typescript/tsx"

	"github.com/aykutssert/scout/internal/core"
)

const (
	maxFileBytes = 1 << 20
	parseTimeout = 5 * time.Second
)

type Analyzer struct{}

func New() *Analyzer { return &Analyzer{} }

var _ core.Analyzer = (*Analyzer)(nil)

func (a *Analyzer) Name() string { return "rn-hint" }

func (a *Analyzer) Available() bool { return true }

func (a *Analyzer) Scan(ctx core.ProjectContext) ([]core.Finding, error) {
	var findings []core.Finding
	for _, rel := range ctx.Files {
		switch strings.ToLower(filepath.Ext(rel)) {
		case ".js", ".jsx", ".tsx", ".mjs", ".cjs":
		default:
			continue
		}
		fs, err := scanFile(filepath.Join(ctx.Root, rel), rel)
		if err != nil {
			continue
		}
		findings = append(findings, fs...)
	}
	return findings, nil
}

func scanFile(abs, rel string) ([]core.Finding, error) {
	if info, err := os.Stat(abs); err == nil && info.Size() > maxFileBytes {
		return nil, nil
	}
	src, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	if len(src) > maxFileBytes {
		return nil, nil
	}

	lang := javascript.GetLanguage()
	if strings.EqualFold(filepath.Ext(abs), ".tsx") {
		lang = tsx.GetLanguage()
	}
	parser := sitter.NewParser()
	defer parser.Close()
	parser.SetLanguage(lang)
	pctx, cancel := context.WithTimeout(context.Background(), parseTimeout)
	defer cancel()
	tree, err := parser.ParseCtx(pctx, nil, src)
	if err != nil {
		return nil, err
	}
	defer tree.Close()
	importedNames, namespaces := reactNativeBindings(tree.RootNode(), src)
	if len(importedNames) == 0 && len(namespaces) == 0 {
		return nil, nil
	}

	var findings []core.Finding
	findings = append(findings, detectImageChildren(tree.RootNode(), src, rel, importedNames, namespaces)...)
	findings = append(findings, detectBareStrings(tree.RootNode(), src, rel, importedNames, namespaces)...)
	findings = append(findings, detectListDataMapped(tree.RootNode(), src, rel, importedNames, namespaces)...)
	findings = append(findings, detectListCallbackPerRow(tree.RootNode(), src, rel, importedNames, namespaces)...)
	findings = append(findings, detectScrollViewMappedList(tree.RootNode(), src, rel, importedNames, namespaces)...)
	findings = append(findings, detectListInlineObject(tree.RootNode(), src, rel, importedNames, namespaces)...)
	findings = append(findings, detectScrollState(tree.RootNode(), src, rel, importedNames, namespaces)...)
	return findings, nil
}

func detectImageChildren(root *sitter.Node, src []byte, file string, importedNames map[string]string, namespaces map[string]bool) []core.Finding {
	var findings []core.Finding
	walk(root, func(node *sitter.Node) {
		if node.Type() != "jsx_element" {
			return
		}
		opening := directChildOfType(node, "jsx_opening_element")
		if opening == nil || !isReactNativeImageTag(jsxTagName(opening, src), importedNames, namespaces) {
			return
		}
		if !hasMeaningfulJSXChild(node, src) {
			return
		}
		findings = append(findings, core.Finding{
			Analyzer:   "rn-hint",
			RuleID:     "rn-no-image-children",
			Severity:   core.SeverityError,
			Level:      core.SeverityError.String(),
			Category:   "bug",
			Confidence: core.ConfidenceRule,
			File:       file,
			Line:       int(opening.StartPoint().Row) + 1,
			Message:    "React Native <Image> does not support children and can fail at runtime.",
			Fix:        "Use <ImageBackground> when content must render over an image, or move the child outside <Image>.",
		})
	})
	return findings
}

func detectBareStrings(root *sitter.Node, src []byte, file string, importedNames map[string]string, namespaces map[string]bool) []core.Finding {
	var findings []core.Finding
	walk(root, func(node *sitter.Node) {
		if node.Type() != "jsx_element" {
			return
		}
		opening := directChildOfType(node, "jsx_opening_element")
		if opening == nil {
			return
		}
		tag := jsxTagName(opening, src)
		if !isNonTextReactNativeComponent(tag, importedNames, namespaces) {
			return
		}

		// Check children of the jsx_element
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			switch child.Type() {
			case "jsx_opening_element", "jsx_closing_element":
				continue
			case "jsx_text":
				text := nodeText(child, src)
				if strings.TrimSpace(text) != "" {
					findings = append(findings, core.Finding{
						Analyzer:   "rn-hint",
						RuleID:     "rn-bare-string-outside-text",
						Severity:   core.SeverityError,
						Level:      core.SeverityError.String(),
						Category:   "bug",
						Confidence: core.ConfidenceRule,
						File:       file,
						Line:       int(child.StartPoint().Row) + 1,
						Message:    "Found raw string child of non-Text React Native component. All text strings must be rendered within a <Text> component.",
						Fix:        "Wrap the string in a <Text> component.",
					})
				}
			case "jsx_expression":
				if child.NamedChildCount() > 0 {
					expr := child.NamedChild(0)
					if expr.Type() == "string" || expr.Type() == "template_string" {
						findings = append(findings, core.Finding{
							Analyzer:   "rn-hint",
							RuleID:     "rn-bare-string-outside-text",
							Severity:   core.SeverityError,
							Level:      core.SeverityError.String(),
							Category:   "bug",
							Confidence: core.ConfidenceRule,
							File:       file,
							Line:       int(child.StartPoint().Row) + 1,
							Message:    "Found raw string literal child of non-Text React Native component. All text strings must be rendered within a <Text> component.",
							Fix:        "Wrap the expression in a <Text> component or remove the curly braces and wrap in <Text>.",
						})
					}
				}
			}
		}
	})
	return findings
}

func reactNativeBindings(root *sitter.Node, src []byte) (map[string]string, map[string]bool) {
	importedNames := map[string]string{}
	namespaces := map[string]bool{}
	walk(root, func(node *sitter.Node) {
		if node.Type() != "import_statement" || unquote(nodeText(node.ChildByFieldName("source"), src)) != "react-native" {
			return
		}
		walk(node, func(child *sitter.Node) {
			switch child.Type() {
			case "import_specifier":
				name := nodeText(child.ChildByFieldName("name"), src)
				local := child.ChildByFieldName("alias")
				localName := name
				if local != nil {
					localName = nodeText(local, src)
				}
				importedNames[localName] = name
			case "namespace_import":
				for i := 0; i < int(child.NamedChildCount()); i++ {
					if local := child.NamedChild(i); local.Type() == "identifier" {
						namespaces[nodeText(local, src)] = true
					}
				}
			}
		})
	})
	return importedNames, namespaces
}

func isReactNativeImageTag(tag string, importedNames map[string]string, namespaces map[string]bool) bool {
	if importedNames[tag] == "Image" {
		return true
	}
	for namespace := range namespaces {
		if tag == namespace+".Image" {
			return true
		}
	}
	return false
}

func isNonTextReactNativeComponent(tag string, importedNames map[string]string, namespaces map[string]bool) bool {
	if importedNames[tag] == "Text" {
		return false
	}
	for namespace := range namespaces {
		if tag == namespace+".Text" {
			return false
		}
	}

	if importedNames[tag] != "" {
		return true
	}
	for namespace := range namespaces {
		if strings.HasPrefix(tag, namespace+".") {
			return true
		}
	}
	return false
}

func jsxTagName(opening *sitter.Node, src []byte) string {
	for i := 0; i < int(opening.NamedChildCount()); i++ {
		child := opening.NamedChild(i)
		switch child.Type() {
		case "identifier", "member_expression", "nested_identifier":
			return nodeText(child, src)
		}
	}
	return ""
}

func hasMeaningfulJSXChild(element *sitter.Node, src []byte) bool {
	for i := 0; i < int(element.NamedChildCount()); i++ {
		child := element.NamedChild(i)
		switch child.Type() {
		case "jsx_opening_element", "jsx_closing_element":
			continue
		case "jsx_text":
			if strings.TrimSpace(nodeText(child, src)) != "" {
				return true
			}
		case "jsx_expression":
			for j := 0; j < int(child.NamedChildCount()); j++ {
				if child.NamedChild(j).Type() != "comment" {
					return true
				}
			}
		default:
			return true
		}
	}
	return false
}

func directChildOfType(node *sitter.Node, typ string) *sitter.Node {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child.Type() == typ {
			return child
		}
	}
	return nil
}

func nodeText(node *sitter.Node, src []byte) string {
	if node == nil {
		return ""
	}
	return node.Content(src)
}

func unquote(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') {
		return value[1 : len(value)-1]
	}
	return value
}

func walk(node *sitter.Node, fn func(*sitter.Node)) {
	if node == nil {
		return
	}
	fn(node)
	for i := 0; i < int(node.NamedChildCount()); i++ {
		walk(node.NamedChild(i), fn)
	}
}

func detectListDataMapped(root *sitter.Node, src []byte, file string, importedNames map[string]string, namespaces map[string]bool) []core.Finding {
	var findings []core.Finding
	walk(root, func(node *sitter.Node) {
		if node.Type() != "jsx_element" && node.Type() != "jsx_self_closing_element" {
			return
		}
		var opening *sitter.Node
		if node.Type() == "jsx_element" {
			opening = directChildOfType(node, "jsx_opening_element")
		} else {
			opening = node
		}
		if opening == nil {
			return
		}
		tag := jsxTagName(opening, src)
		isList := tag == "FlatList" || tag == "FlashList" || importedNames[tag] == "FlatList"
		if !isList {
			for namespace := range namespaces {
				if tag == namespace+".FlatList" {
					isList = true
					break
				}
			}
		}
		if !isList {
			return
		}

		walk(opening, func(attr *sitter.Node) {
			if attr.Type() != "jsx_attribute" {
				return
			}
			nameNode := attr.NamedChild(0)
			if nameNode == nil || nameNode.Content(src) != "data" {
				return
			}
			valNode := attr.NamedChild(1)
			if valNode == nil || valNode.Type() != "jsx_expression" {
				return
			}
			for i := 0; i < int(valNode.NamedChildCount()); i++ {
				child := valNode.NamedChild(i)
				if isInlineMapCall(child, src) {
					findings = append(findings, core.Finding{
						Analyzer:   "rn-hint",
						RuleID:     "rn-list-data-mapped",
						Severity:   core.SeverityWarning,
						Level:      core.SeverityWarning.String(),
						Category:   "performance",
						Confidence: core.ConfidenceRule,
						File:       file,
						Line:       int(child.StartPoint().Row) + 1,
						Message:    "List 'data' prop is populated via inline '.map()' inside render body. This recreates the array reference on every render, causing performance degradation by forcing child cells to redraw.",
						Fix:        "Move the data mapping logic to a useMemo hook or outside the component's render body.",
					})
				}
			}
		})
	})
	return findings
}

func detectListCallbackPerRow(root *sitter.Node, src []byte, file string, importedNames map[string]string, namespaces map[string]bool) []core.Finding {
	var findings []core.Finding
	walk(root, func(node *sitter.Node) {
		if node.Type() != "jsx_element" && node.Type() != "jsx_self_closing_element" {
			return
		}
		var opening *sitter.Node
		if node.Type() == "jsx_element" {
			opening = directChildOfType(node, "jsx_opening_element")
		} else {
			opening = node
		}
		if opening == nil {
			return
		}
		tag := jsxTagName(opening, src)
		isList := tag == "FlatList" || tag == "FlashList" || importedNames[tag] == "FlatList"
		if !isList {
			for namespace := range namespaces {
				if tag == namespace+".FlatList" {
					isList = true
					break
				}
			}
		}
		if !isList {
			return
		}

		walk(opening, func(attr *sitter.Node) {
			if attr.Type() != "jsx_attribute" {
				return
			}
			nameNode := attr.NamedChild(0)
			if nameNode == nil || nameNode.Content(src) != "renderItem" {
				return
			}
			valNode := attr.NamedChild(1)
			if valNode == nil || valNode.Type() != "jsx_expression" {
				return
			}
			for i := 0; i < int(valNode.NamedChildCount()); i++ {
				child := valNode.NamedChild(i)
				if isInlineFunction(child) {
					findings = append(findings, core.Finding{
						Analyzer:   "rn-hint",
						RuleID:     "rn-list-callback-per-row",
						Severity:   core.SeverityWarning,
						Level:      core.SeverityWarning.String(),
						Category:   "performance",
						Confidence: core.ConfidenceRule,
						File:       file,
						Line:       int(child.StartPoint().Row) + 1,
						Message:    "List 'renderItem' callback is defined inline inside the render body. This recreates the callback function on every render, breaking item view recycling optimization.",
						Fix:        "Extract the renderItem function to a separate component, module scope, or wrap it in a useCallback hook.",
					})
				}
			}
		})
	})
	return findings
}

func detectScrollViewMappedList(root *sitter.Node, src []byte, file string, importedNames map[string]string, namespaces map[string]bool) []core.Finding {
	var findings []core.Finding
	walk(root, func(node *sitter.Node) {
		if node.Type() != "jsx_element" {
			return
		}
		opening := directChildOfType(node, "jsx_opening_element")
		if opening == nil {
			return
		}
		tag := jsxTagName(opening, src)
		isScrollView := tag == "ScrollView" || importedNames[tag] == "ScrollView"
		if !isScrollView {
			for namespace := range namespaces {
				if tag == namespace+".ScrollView" {
					isScrollView = true
					break
				}
			}
		}
		if !isScrollView {
			return
		}

		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child.Type() != "jsx_expression" {
				continue
			}
			for j := 0; j < int(child.NamedChildCount()); j++ {
				expr := child.NamedChild(j)
				if isInlineMapCall(expr, src) {
					findings = append(findings, core.Finding{
						Analyzer:   "rn-hint",
						RuleID:     "rn-no-scrollview-mapped-list",
						Severity:   core.SeverityWarning,
						Level:      core.SeverityWarning.String(),
						Category:   "performance",
						Confidence: core.ConfidenceRule,
						File:       file,
						Line:       int(expr.StartPoint().Row) + 1,
						Message:    "Inline '.map()' list rendering detected inside a <ScrollView>. This renders all items at once without virtualization, which can cause severe lag and memory issues for large lists.",
						Fix:        "Replace <ScrollView> with <FlatList> or Shopify's <FlashList> to enable lazy rendering and component recycling.",
					})
				}
			}
		}
	})
	return findings
}

func isInlineMapCall(node *sitter.Node, src []byte) bool {
	if node == nil || node.Type() != "call_expression" {
		return false
	}
	callee := node.ChildByFieldName("function")
	if callee == nil {
		return false
	}
	if callee.Type() == "member_expression" {
		prop := callee.ChildByFieldName("property")
		if prop != nil && prop.Content(src) == "map" {
			return true
		}
	}
	return false
}

func detectListInlineObject(root *sitter.Node, src []byte, file string, importedNames map[string]string, namespaces map[string]bool) []core.Finding {
	var findings []core.Finding
	walk(root, func(node *sitter.Node) {
		if node.Type() != "jsx_element" && node.Type() != "jsx_self_closing_element" {
			return
		}
		var opening *sitter.Node
		if node.Type() == "jsx_element" {
			opening = directChildOfType(node, "jsx_opening_element")
		} else {
			opening = node
		}
		if opening == nil {
			return
		}
		tag := jsxTagName(opening, src)
		isList := tag == "FlatList" || tag == "FlashList" || importedNames[tag] == "FlatList" || importedNames[tag] == "FlashList"
		if !isList {
			for namespace := range namespaces {
				if tag == namespace+".FlatList" || tag == namespace+".FlashList" {
					isList = true
					break
				}
			}
		}
		if !isList {
			return
		}

		walk(opening, func(attr *sitter.Node) {
			if attr.Type() != "jsx_attribute" {
				return
			}
			nameNode := attr.NamedChild(0)
			if nameNode == nil || nameNode.Content(src) != "renderItem" {
				return
			}
			valNode := attr.NamedChild(1)
			if valNode == nil || valNode.Type() != "jsx_expression" {
				return
			}
			for i := 0; i < int(valNode.NamedChildCount()); i++ {
				cb := valNode.NamedChild(i)
				if !isInlineFunction(cb) {
					continue
				}
				findings = append(findings, findInlineObjectInCallback(cb, src, file)...)
			}
		})
	})
	return findings
}

func findInlineObjectInCallback(cb *sitter.Node, src []byte, file string) []core.Finding {
	var findings []core.Finding
	walk(cb, func(n *sitter.Node) {
		if n.Type() != "jsx_element" && n.Type() != "jsx_self_closing_element" {
			return
		}
		var opening *sitter.Node
		if n.Type() == "jsx_element" {
			opening = directChildOfType(n, "jsx_opening_element")
		} else {
			opening = n
		}
		if opening == nil {
			return
		}
		walk(opening, func(attr *sitter.Node) {
			if attr.Type() != "jsx_attribute" {
				return
			}
			valNode := attr.NamedChild(1)
			if valNode == nil || valNode.Type() != "jsx_expression" {
				return
			}
			for i := 0; i < int(valNode.NamedChildCount()); i++ {
				expr := valNode.NamedChild(i)
				if expr.Type() == "object" || expr.Type() == "array" {
					findings = append(findings, core.Finding{
						Analyzer:   "rn-hint",
						RuleID:     "rn-no-inline-object-in-list-item",
						Severity:   core.SeverityWarning,
						Level:      core.SeverityWarning.String(),
						Category:   "performance",
						Confidence: core.ConfidenceHint,
						File:       file,
						Line:       int(attr.StartPoint().Row) + 1,
						Message:    "Inline object/array literal in JSX prop inside list renderItem. This creates a new reference on every render, breaking memo optimization for list items.",
						Fix:        "Extract the object/array to a module-scope constant, StyleSheet.create(), or useMemo.",
					})
				}
			}
		})
	})
	return findings
}

func detectScrollState(root *sitter.Node, src []byte, file string, importedNames map[string]string, namespaces map[string]bool) []core.Finding {
	var findings []core.Finding
	walk(root, func(node *sitter.Node) {
		if node.Type() != "jsx_element" && node.Type() != "jsx_self_closing_element" {
			return
		}
		var opening *sitter.Node
		if node.Type() == "jsx_element" {
			opening = directChildOfType(node, "jsx_opening_element")
		} else {
			opening = node
		}
		if opening == nil {
			return
		}
		tag := jsxTagName(opening, src)
		isScrollable := tag == "ScrollView" || tag == "FlatList" || tag == "FlashList" || tag == "Animated.ScrollView" ||
			importedNames[tag] == "ScrollView" || importedNames[tag] == "FlatList" || importedNames[tag] == "FlashList"
		if !isScrollable {
			for namespace := range namespaces {
				if tag == namespace+".ScrollView" || tag == namespace+".FlatList" || tag == namespace+".FlashList" {
					isScrollable = true
					break
				}
			}
		}
		if !isScrollable {
			return
		}

		walk(opening, func(attr *sitter.Node) {
			if attr.Type() != "jsx_attribute" {
				return
			}
			nameNode := attr.NamedChild(0)
			if nameNode == nil || nameNode.Content(src) != "onScroll" {
				return
			}
			valNode := attr.NamedChild(1)
			if valNode == nil || valNode.Type() != "jsx_expression" {
				return
			}
			for i := 0; i < int(valNode.NamedChildCount()); i++ {
				handler := valNode.NamedChild(i)
				var fnBody *sitter.Node
				if handler.Type() == "arrow_function" || handler.Type() == "function_expression" {
					fnBody = handler.ChildByFieldName("body")
				} else if handler.Type() == "identifier" {
					// Named handler — find its definition
					// For now, only handle inline functions
					continue
				}
				if fnBody == nil {
					continue
				}
				if hasSetterCall(fnBody, src) {
					findings = append(findings, core.Finding{
						Analyzer:   "rn-hint",
						RuleID:     "rn-no-scroll-state",
						Severity:   core.SeverityWarning,
						Level:      core.SeverityWarning.String(),
						Category:   "performance",
						Confidence: core.ConfidenceHint,
						File:       file,
						Line:       int(attr.StartPoint().Row) + 1,
						Message:    "onScroll handler calls a state setter. Scroll events fire at high frequency (60fps) and setState triggers a synchronous render cycle, causing jank.",
						Fix:        "Use Reanimated's useSharedValue + useAnimatedScrollHandler to update scroll position without JS thread re-renders.",
					})
				}
			}
		})
	})
	return findings
}

var setterRe = regexp.MustCompile(`^set[A-Z]`)

func hasSetterCall(body *sitter.Node, src []byte) bool {
	if body == nil {
		return false
	}
	found := false
	walk(body, func(n *sitter.Node) {
		if found {
			return
		}
		if n.Type() != "call_expression" {
			return
		}
		fn := n.ChildByFieldName("function")
		if fn == nil {
			return
		}
		name := nodeText(fn, src)
		if setterRe.MatchString(name) {
			found = true
		}
	})
	return found
}

func isInlineFunction(node *sitter.Node) bool {
	if node == nil {
		return false
	}
	return node.Type() == "arrow_function" || node.Type() == "function_expression"
}
