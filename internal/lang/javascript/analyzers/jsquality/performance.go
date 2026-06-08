package jsquality

import (
	"regexp"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/aykutssert/scout/internal/core"
)

var intlConstructors = map[string]bool{
	"Intl.Collator":           true,
	"Intl.DateTimeFormat":     true,
	"Intl.DisplayNames":       true,
	"Intl.ListFormat":         true,
	"Intl.Locale":             true,
	"Intl.NumberFormat":       true,
	"Intl.PluralRules":        true,
	"Intl.RelativeTimeFormat": true,
	"Intl.Segmenter":          true,
}

var repeatedArrayMethods = map[string]bool{
	"every": true, "filter": true, "flatMap": true, "forEach": true,
	"map": true, "reduce": true, "some": true,
}

func detectPerformancePatterns(root *sitter.Node, _ *sitter.Language, src []byte, file string) []core.Finding {
	if isPerformanceTestFile(file) {
		return nil
	}
	var out []core.Finding
	walkQuality(root, func(node *sitter.Node) {
		switch node.Type() {
		case "new_expression":
			out = append(out, detectHoistableConstruction(node, src, file)...)
		case "call_expression":
			out = append(out, detectRepeatedLookup(node, src, file)...)
			out = append(out, detectCombineIterations(node, src, file)...)
			out = append(out, detectAsyncReduceWithoutAwaitedAcc(node, src, file)...)
			out = append(out, detectCacheStorage(node, src, file)...)
		case "variable_declarator":
			out = append(out, detectModuleScopeStaticValue(node, src, file)...)
			value := node.ChildByFieldName("value")
			if value != nil && (value.Type() == "arrow_function" || value.Type() == "function_expression") {
				out = append(out, detectModuleScopePureFunction(value, src, file)...)
			}
		case "function_declaration":
			out = append(out, detectModuleScopePureFunction(node, src, file)...)
		}
	})
	return out
}

func detectCombineIterations(node *sitter.Node, src []byte, file string) []core.Finding {
	fn := node.ChildByFieldName("function")
	if fn == nil || fn.Type() != "member_expression" {
		return nil
	}
	outerProp := qualityText(fn.ChildByFieldName("property"), src)
	if !repeatedArrayMethods[outerProp] {
		return nil
	}
	object := fn.ChildByFieldName("object")
	if object == nil || object.Type() != "call_expression" {
		return nil
	}
	innerFn := object.ChildByFieldName("function")
	if innerFn == nil || innerFn.Type() != "member_expression" {
		return nil
	}
	innerProp := qualityText(innerFn.ChildByFieldName("property"), src)
	if !repeatedArrayMethods[innerProp] {
		return nil
	}

	line := int(node.StartPoint().Row) + 1
	return []core.Finding{hint(
		"js-combine-iterations", "performance", core.SeverityInfo, file, line,
		"Chained array iterations '."+innerProp+"()."+outerProp+"()' can be combined into a single iteration.",
		"Use a single loop or .reduce() to avoid creating intermediate arrays and iterating multiple times.",
	)}
}

func detectAsyncReduceWithoutAwaitedAcc(node *sitter.Node, src []byte, file string) []core.Finding {
	fn := node.ChildByFieldName("function")
	if fn == nil || fn.Type() != "member_expression" {
		return nil
	}
	prop := qualityText(fn.ChildByFieldName("property"), src)
	if prop != "reduce" {
		return nil
	}
	args := node.ChildByFieldName("arguments")
	if args == nil || args.NamedChildCount() == 0 {
		return nil
	}
	callback := args.NamedChild(0)
	if callback == nil || !isQualityFunction(callback) {
		return nil
	}
	if !isAsyncFunction(callback, src) {
		return nil
	}
	accName := firstParamName(callback, src)
	if accName == "" {
		return nil
	}
	body := callback.ChildByFieldName("body")
	if body == nil {
		return nil
	}

	hasUnawaitedAccess := false
	var unawaitedNode *sitter.Node

	walkQuality(body, func(n *sitter.Node) {
		if hasUnawaitedAccess || n.Type() != "identifier" {
			return
		}
		if qualityText(n, src) == accName {
			if isPropertyOfMemberExpression(n) {
				return
			}
			if isReturnStatementChild(n) {
				return
			}
			if callback.Type() == "arrow_function" && callback.ChildByFieldName("body") == n {
				return
			}
			if !isAwaited(n) {
				hasUnawaitedAccess = true
				unawaitedNode = n
			}
		}
	})

	if hasUnawaitedAccess && unawaitedNode != nil {
		line := int(unawaitedNode.StartPoint().Row) + 1
		return []core.Finding{hint(
			"js-async-reduce-without-awaited-acc", "performance", core.SeverityWarning, file, line,
			"The accumulator '"+accName+"' in this async reduce callback is used without being awaited.",
			"Since the callback is async, the accumulator is a Promise in subsequent iterations. Await it before accessing its properties.",
		)}
	}

	return nil
}

func isAsyncFunction(node *sitter.Node, src []byte) bool {
	if node == nil {
		return false
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		if node.Child(i).Type() == "async" {
			return true
		}
	}
	return strings.HasPrefix(strings.TrimSpace(qualityText(node, src)), "async")
}

func firstParamName(fn *sitter.Node, src []byte) string {
	params := fn.ChildByFieldName("parameters")
	if params == nil {
		return ""
	}
	for i := 0; i < int(params.NamedChildCount()); i++ {
		child := params.NamedChild(i)
		if child.Type() == "identifier" {
			return qualityText(child, src)
		}
		if child.Type() == "required_parameter" {
			pattern := child.ChildByFieldName("pattern")
			if pattern != nil && pattern.Type() == "identifier" {
				return qualityText(pattern, src)
			}
			return qualityText(child, src)
		}
	}
	return ""
}

func isAwaited(node *sitter.Node) bool {
	for p := node.Parent(); p != nil; p = p.Parent() {
		if p.Type() == "await_expression" {
			return true
		}
		if isQualityFunction(p) {
			break
		}
	}
	return false
}

func isPropertyOfMemberExpression(node *sitter.Node) bool {
	parent := node.Parent()
	if parent != nil && parent.Type() == "member_expression" {
		return parent.ChildByFieldName("property") == node
	}
	return false
}

func isReturnStatementChild(node *sitter.Node) bool {
	parent := node.Parent()
	if parent != nil && parent.Type() == "return_statement" {
		return true
	}
	return false
}

func detectCacheStorage(node *sitter.Node, src []byte, file string) []core.Finding {
	if !insideRepeatedExecution(node, src) {
		return nil
	}
	fn := node.ChildByFieldName("function")
	if fn == nil || fn.Type() != "member_expression" {
		return nil
	}
	obj := fn.ChildByFieldName("object")
	prop := fn.ChildByFieldName("property")
	if obj == nil || prop == nil {
		return nil
	}
	objName := qualityText(obj, src)
	propName := qualityText(prop, src)

	if (objName == "localStorage" || objName == "sessionStorage") && propName == "getItem" {
		line := int(node.StartPoint().Row) + 1
		return []core.Finding{hint(
			"js-cache-storage", "performance", core.SeverityInfo, file, line,
			"Reading from "+objName+" inside a loop or repeated execution path.",
			"Store the value in a local variable outside the loop to avoid repeated synchronous disk access.",
		)}
	}
	return nil
}



func isPerformanceTestFile(file string) bool {
	path := "/" + strings.ToLower(strings.ReplaceAll(file, "\\", "/"))
	return strings.Contains(path, "/__tests__/") ||
		strings.Contains(path, "/__testfixtures__/") ||
		strings.Contains(path, "/fixtures/") ||
		strings.Contains(path, ".test.") ||
		strings.Contains(path, ".spec.")
}

func detectHoistableConstruction(node *sitter.Node, src []byte, file string) []core.Finding {
	if !insideRepeatedExecution(node, src) || !staticNewArguments(node, src) {
		return nil
	}
	constructor := node.ChildByFieldName("constructor")
	name := qualityText(constructor, src)
	line := int(node.StartPoint().Row) + 1
	switch {
	case intlConstructors[name]:
		return []core.Finding{hint(
			"js-hoist-intl", "performance", core.SeverityInfo, file, line,
			name+" is rebuilt in a repeated execution path even though its configuration is static.",
			"Create the formatter once at module scope and reuse it.",
		)}
	case name == "RegExp":
		return []core.Finding{hint(
			"js-hoist-regexp", "performance", core.SeverityInfo, file, line,
			"new RegExp(...) uses a static pattern inside a repeated execution path.",
			"Create the RegExp once outside the component or loop and reuse it.",
		)}
	default:
		return nil
	}
}

func detectRepeatedLookup(node *sitter.Node, src []byte, file string) []core.Finding {
	if !insideCollectionIteration(node, src) {
		return nil
	}
	fn := node.ChildByFieldName("function")
	if fn == nil || fn.Type() != "member_expression" {
		return nil
	}
	property := qualityText(fn.ChildByFieldName("property"), src)
	object := fn.ChildByFieldName("object")
	if object == nil || (object.Type() != "identifier" && object.Type() != "member_expression") {
		return nil
	}
	line := int(node.StartPoint().Row) + 1
	switch property {
	case "find":
		if !isKnownArrayExpression(object, node, src) {
			return nil
		}
		return []core.Finding{hint(
			"js-index-maps", "performance", core.SeverityInfo, file, line,
			"Array.find() runs inside a loop or array iteration, which can turn the operation into O(N²) work.",
			"Build a Map keyed by the lookup field before the loop and use Map.get().",
		)}
	case "includes":
		if !isKnownArrayExpression(object, node, src) {
			return nil
		}
		return []core.Finding{hint(
			"js-set-map-lookups", "performance", core.SeverityInfo, file, line,
			"Array.includes() runs inside a loop or array iteration, repeatedly scanning the same collection.",
			"Build a Set before the loop and use Set.has().",
		)}
	default:
		return nil
	}
}

func detectModuleScopeStaticValue(node *sitter.Node, src []byte, file string) []core.Finding {
	fn := nearestQualityFunction(node.Parent())
	if fn == nil || !isComponentName(qualityFunctionName(fn, src)) || nearestQualityFunction(fn.Parent()) != nil {
		return nil
	}
	decl := node.Parent()
	if decl == nil || decl.Type() != "lexical_declaration" || !strings.HasPrefix(strings.TrimSpace(qualityText(decl, src)), "const ") {
		return nil
	}
	value := node.ChildByFieldName("value")
	if value == nil || (value.Type() != "object" && value.Type() != "array" && value.Type() != "regex") || !isStaticExpression(value, src) {
		return nil
	}
	name := qualityText(node.ChildByFieldName("name"), src)
	if name == "" {
		return nil
	}
	return []core.Finding{hint(
		"prefer-module-scope-static-value", "performance", core.SeverityInfo, file,
		int(node.StartPoint().Row)+1,
		name+" is a static value recreated on every component render.",
		"Move the constant to module scope so its allocation and identity are stable.",
	)}
}

func detectModuleScopePureFunction(node *sitter.Node, src []byte, file string) []core.Finding {
	component := nearestQualityFunction(node.Parent())
	if component == nil || !isComponentName(qualityFunctionName(component, src)) {
		return nil
	}
	name := qualityFunctionName(node, src)
	if name == "" || isComponentName(name) || capturesOuterLocal(node, component, src) {
		return nil
	}
	return []core.Finding{hint(
		"prefer-module-scope-pure-function", "performance", core.SeverityInfo, file,
		int(node.StartPoint().Row)+1,
		name+" does not capture component-local values but is recreated on every render.",
		"Move the helper to module scope so the function is allocated once.",
	)}
}

func capturesOuterLocal(inner, outer *sitter.Node, src []byte) bool {
	outerNames := declaredNames(outer, inner, src)
	innerNames := declaredNames(inner, nil, src)
	captured := false
	walkQuality(inner, func(node *sitter.Node) {
		if captured || node.Type() != "identifier" {
			return
		}
		name := qualityText(node, src)
		if outerNames[name] && !innerNames[name] {
			captured = true
		}
	})
	return captured
}

func declaredNames(scope, skip *sitter.Node, src []byte) map[string]bool {
	out := map[string]bool{}
	var walk func(*sitter.Node)
	walk = func(node *sitter.Node) {
		if node == nil || node == skip {
			return
		}
		switch node.Type() {
		case "required_parameter", "optional_parameter", "identifier", "shorthand_property_identifier_pattern":
			if parent := node.Parent(); parent != nil && (parent.Type() == "formal_parameters" || parent.Type() == "required_parameter" || parent.Type() == "optional_parameter" || parent.Type() == "object_pattern" || parent.Type() == "array_pattern") {
				out[qualityText(node, src)] = true
			}
		case "variable_declarator", "function_declaration":
			collectPatternIdentifiers(node.ChildByFieldName("name"), src, out)
		}
		for i := 0; i < int(node.NamedChildCount()); i++ {
			walk(node.NamedChild(i))
		}
	}
	walk(scope)
	return out
}

func collectPatternIdentifiers(node *sitter.Node, src []byte, out map[string]bool) {
	if node == nil {
		return
	}
	if node.Type() == "identifier" || node.Type() == "shorthand_property_identifier_pattern" {
		out[qualityText(node, src)] = true
	}
	for i := 0; i < int(node.NamedChildCount()); i++ {
		collectPatternIdentifiers(node.NamedChild(i), src, out)
	}
}

func insideRepeatedExecution(node *sitter.Node, src []byte) bool {
	for parent := node.Parent(); parent != nil; parent = parent.Parent() {
		switch parent.Type() {
		case "for_statement", "for_in_statement", "while_statement", "do_statement":
			return true
		}
		if isQualityFunction(parent) {
			return isComponentName(qualityFunctionName(parent, src)) || repeatedCallback(parent, src)
		}
	}
	return false
}

func insideCollectionIteration(node *sitter.Node, src []byte) bool {
	for parent := node.Parent(); parent != nil; parent = parent.Parent() {
		switch parent.Type() {
		case "for_statement", "for_in_statement", "while_statement", "do_statement":
			return true
		}
		if isQualityFunction(parent) {
			return repeatedCallback(parent, src)
		}
	}
	return false
}

func isKnownArrayExpression(object, use *sitter.Node, src []byte) bool {
	if object == nil {
		return false
	}
	if object.Type() == "array" {
		return true
	}
	if object.Type() != "identifier" {
		return false
	}
	name := qualityText(object, src)
	scope := nearestQualityFunction(use)
	if scope == nil {
		scope = rootQualityNode(use)
	}
	if typedArrayParameter(scope, name, src) {
		return true
	}
	known := false
	walkQuality(scope, func(node *sitter.Node) {
		if known || node.Type() != "variable_declarator" || node.StartByte() >= use.StartByte() {
			return
		}
		if qualityText(node.ChildByFieldName("name"), src) != name {
			return
		}
		value := node.ChildByFieldName("value")
		known = isArrayProducingExpression(value, src)
	})
	if !known && scope.Parent() != nil {
		root := rootQualityNode(scope)
		walkQuality(root, func(node *sitter.Node) {
			if known || node.Type() != "variable_declarator" || node.StartByte() >= use.StartByte() {
				return
			}
			if qualityText(node.ChildByFieldName("name"), src) == name {
				known = isArrayProducingExpression(node.ChildByFieldName("value"), src)
			}
		})
	}
	return known
}

func typedArrayParameter(scope *sitter.Node, name string, src []byte) bool {
	if scope == nil || name == "" {
		return false
	}
	params := scope.ChildByFieldName("parameters")
	if params == nil {
		return false
	}
	pattern := `\b` + regexp.QuoteMeta(name) + `\s*(?:\??:\s*(?:readonly\s+)?[^,)=]*(?:\[\]|Array\s*<))`
	return regexp.MustCompile(pattern).MatchString(qualityText(params, src))
}

func isArrayProducingExpression(node *sitter.Node, src []byte) bool {
	if node == nil {
		return false
	}
	if node.Type() == "array" {
		return true
	}
	if node.Type() != "call_expression" {
		return false
	}
	fn := node.ChildByFieldName("function")
	if fn == nil || fn.Type() != "member_expression" {
		return false
	}
	object := qualityText(fn.ChildByFieldName("object"), src)
	property := qualityText(fn.ChildByFieldName("property"), src)
	if object == "Array" && property == "from" {
		return true
	}
	if object == "Object" && (property == "keys" || property == "values" || property == "entries") {
		return true
	}
	switch property {
	case "concat", "filter", "flat", "flatMap", "map", "slice", "toReversed", "toSorted", "toSpliced":
		return true
	default:
		return false
	}
}

func rootQualityNode(node *sitter.Node) *sitter.Node {
	for node != nil && node.Parent() != nil {
		node = node.Parent()
	}
	return node
}

func repeatedCallback(fn *sitter.Node, src []byte) bool {
	args := fn.Parent()
	if args == nil || args.Type() != "arguments" {
		return false
	}
	call := args.Parent()
	if call == nil || call.Type() != "call_expression" {
		return false
	}
	callee := call.ChildByFieldName("function")
	return callee != nil && callee.Type() == "member_expression" &&
		repeatedArrayMethods[qualityText(callee.ChildByFieldName("property"), src)]
}

func staticNewArguments(node *sitter.Node, src []byte) bool {
	args := node.ChildByFieldName("arguments")
	if args == nil {
		return true
	}
	for i := 0; i < int(args.NamedChildCount()); i++ {
		if !isStaticExpression(args.NamedChild(i), src) {
			return false
		}
	}
	return true
}

func isStaticExpression(node *sitter.Node, src []byte) bool {
	if node == nil {
		return true
	}
	switch node.Type() {
	case "string", "number", "true", "false", "null", "regex":
		return true
	case "template_string":
		static := true
		walkQuality(node, func(child *sitter.Node) {
			if child.Type() == "template_substitution" {
				static = false
			}
		})
		return static
	case "array", "object", "pair", "parenthesized_expression", "unary_expression":
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if node.Type() == "pair" && child == node.ChildByFieldName("key") {
				continue
			}
			if !isStaticExpression(child, src) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func nearestQualityFunction(node *sitter.Node) *sitter.Node {
	for current := node; current != nil; current = current.Parent() {
		if isQualityFunction(current) {
			return current
		}
	}
	return nil
}

func isQualityFunction(node *sitter.Node) bool {
	if node == nil {
		return false
	}
	switch node.Type() {
	case "function_declaration", "function_expression", "arrow_function", "method_definition":
		return true
	default:
		return false
	}
}

func qualityFunctionName(node *sitter.Node, src []byte) string {
	if node == nil {
		return ""
	}
	if name := node.ChildByFieldName("name"); name != nil {
		return qualityText(name, src)
	}
	parent := node.Parent()
	if parent != nil && parent.Type() == "variable_declarator" {
		return qualityText(parent.ChildByFieldName("name"), src)
	}
	return ""
}

func qualityText(node *sitter.Node, src []byte) string {
	if node == nil {
		return ""
	}
	return node.Content(src)
}

func walkQuality(node *sitter.Node, fn func(*sitter.Node)) {
	if node == nil {
		return
	}
	fn(node)
	for i := 0; i < int(node.NamedChildCount()); i++ {
		walkQuality(node.NamedChild(i), fn)
	}
}
