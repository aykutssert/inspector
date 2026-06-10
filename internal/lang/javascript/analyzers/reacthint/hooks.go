package reacthint

import (
	"regexp"
	"strconv"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/aykutssert/scout/internal/core"
)

const derivedStateQuery = `
(call_expression
  function: (identifier) @fn
  arguments: (arguments [(member_expression) (call_expression)] @arg)) @call
`

// detectDerivedState flags useState seeded from a prop/object field or another
// computed value (useState(props.x), useState(compute(x))). Such state won't
// update when its source changes — a frequent React bug. Literal initializers
// (useState(0)) and lazy initializers (useState(() => ...)) are not matched, so
// the legitimate patterns stay quiet.
func detectDerivedState(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding {
	var out []core.Finding
	_ = runMatches(derivedStateQuery, root, lang, func(caps map[string]*sitter.Node) {
		if nodeText(caps["fn"], src) != "useState" {
			return
		}
		call := caps["call"]
		if call == nil {
			return
		}
		out = append(out, hint(
			"no-derived-state", "bug", core.SeverityWarning, file,
			int(call.StartPoint().Row)+1,
			"useState is seeded from a prop or computed value; the state won't update when that source changes.",
			"Compute the value during render (or useMemo), or sync it deliberately when the source changes.",
		))
	})
	return out
}

const effectArrowQuery = `
(call_expression
  function: (identifier) @fn
  arguments: (arguments . (arrow_function) @cb .)) @call
`

const effectFuncQuery = `
(call_expression
  function: (identifier) @fn
  arguments: (arguments . (function_expression) @cb .)) @call
`

const setterCallQuery = `(call_expression function: (identifier) @id)`

var setterRe = regexp.MustCompile(`^set[A-Z]`)

// detectSetStateInEffectNoDeps flags useEffect(fn) with no dependency array
// whose body calls a setX setter. That runs after every render and re-triggers
// itself — a classic infinite-loop smell. The anchors in the query (single
// argument) ensure the dependency array is absent.
func detectSetStateInEffectNoDeps(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding {
	var out []core.Finding
	check := func(caps map[string]*sitter.Node) {
		if nodeText(caps["fn"], src) != "useEffect" {
			return
		}
		cb, call := caps["cb"], caps["call"]
		if cb == nil || call == nil || !callsSetter(cb, lang, src) {
			return
		}
		out = append(out, hint(
			"setstate-in-effect-without-deps", "bug", core.SeverityWarning, file,
			int(call.StartPoint().Row)+1,
			"useEffect has no dependency array and calls a state setter; it runs after every render and can loop.",
			"Add a dependency array, or move the state update out of the effect.",
		))
	}
	_ = runMatches(effectArrowQuery, root, lang, check)
	_ = runMatches(effectFuncQuery, root, lang, check)
	return out
}

const componentQuery = `
(function_declaration name: (identifier) @name) @fn
(variable_declarator name: (identifier) @name value: (arrow_function)) @fn
(variable_declarator name: (identifier) @name value: (function_expression)) @fn
`

// useReducerThreshold is the number of useState hooks in one component/hook
// above which we suggest consolidating with useReducer.
const useReducerThreshold = 4

// detectPreferUseReducer flags a component or hook that declares many useState
// hooks; that state is usually related and easier to manage with useReducer.
func detectPreferUseReducer(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding {
	var out []core.Finding
	_ = runMatches(componentQuery, root, lang, func(caps map[string]*sitter.Node) {
		name := nodeText(caps["name"], src)
		fn := caps["fn"]
		if fn == nil || !isComponentOrHook(name) {
			return
		}
		if countUseState(fn, lang, src) < useReducerThreshold {
			return
		}
		out = append(out, hint(
			"prefer-use-reducer", "quality", core.SeverityInfo, file,
			int(fn.StartPoint().Row)+1,
			name+" declares "+strconv.Itoa(countUseState(fn, lang, src))+" useState hooks; related state is often clearer with useReducer.",
			"Consolidate related state into a single useReducer.",
		))
	})
	return out
}
func isComponentFunction(node *sitter.Node, src []byte) bool {
	if node == nil {
		return false
	}
	var name string
	if n := node.ChildByFieldName("name"); n != nil {
		name = n.Content(src)
	} else if parent := node.Parent(); parent != nil && parent.Type() == "variable_declarator" {
		if n := parent.ChildByFieldName("name"); n != nil {
			name = n.Content(src)
		}
	}
	return isComponentName(name) || isHookName(name)
}

func detectRerenderDeferReadsHook(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding {
	var out []core.Finding
	var searchParamsVar string
	walkReact(root, func(n *sitter.Node) {
		if n.Type() != "call_expression" {
			return
		}
		callee := n.ChildByFieldName("function")
		if callee == nil || nodeText(callee, src) != "useSearchParams" {
			return
		}
		parent := n.Parent()
		if parent == nil || parent.Type() != "variable_declarator" {
			return
		}
		arr := parent.ChildByFieldName("name")
		if arr == nil || arr.Type() != "array_pattern" || arr.NamedChildCount() < 1 {
			return
		}
		searchParamsVar = arr.NamedChild(0).Content(src)
	})
	if searchParamsVar == "" {
		return nil
	}
	walkReact(root, func(n *sitter.Node) {
		if n.Type() != "call_expression" {
			return
		}
		fn := n.ChildByFieldName("function")
		if fn == nil || fn.Type() != "member_expression" {
			return
		}
		obj := fn.ChildByFieldName("object")
		prop := fn.ChildByFieldName("property")
		if obj == nil || prop == nil || obj.Type() != "identifier" || nodeText(obj, src) != searchParamsVar || nodeText(prop, src) != "get" {
			return
		}
		enclosing := enclosingFunction(n)
		if enclosing == nil || !isComponentFunction(enclosing, src) {
			return
		}
		if !inRenderBody(n, enclosing) {
			return
		}
		out = append(out, hint(
			"rerender-defer-reads-hook", "performance", core.SeverityInfo, file,
			int(n.StartPoint().Row)+1,
			"URL search param read in render body triggers re-render when params change. Defer to event handlers to avoid unnecessary re-renders.",
			"Move searchParams.get() into event handlers or use useMemo to isolate the re-render scope.",
		))
	})
	return out
}

func inRenderBody(node, fn *sitter.Node) bool {
	for p := node.Parent(); p != nil && p != fn; p = p.Parent() {
		switch p.Type() {
		case "arrow_function", "function_expression", "function_declaration":
			return false
		}
	}
	return true
}

func detectRerenderDerivedStateFromHook(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding {
	setters := map[string]bool{}
	walkReact(root, func(n *sitter.Node) {
		if n.Type() != "call_expression" {
			return
		}
		callee := n.ChildByFieldName("function")
		if callee == nil || nodeText(callee, src) != "useState" {
			return
		}
		parent := n.Parent()
		if parent == nil || parent.Type() != "variable_declarator" {
			return
		}
		arr := parent.ChildByFieldName("name")
		if arr == nil || arr.Type() != "array_pattern" || arr.NamedChildCount() < 2 {
			return
		}
		setters[arr.NamedChild(1).Content(src)] = true
	})
	if len(setters) == 0 {
		return nil
	}
	highFreqEvents := map[string]bool{"scroll": true, "resize": true, "mousemove": true, "touchmove": true, "pointermove": true, "wheel": true}
	var out []core.Finding
	walkReact(root, func(n *sitter.Node) {
		if n.Type() != "call_expression" {
			return
		}
		callee := n.ChildByFieldName("function")
		if callee == nil || callee.Type() != "member_expression" {
			return
		}
		prop := callee.ChildByFieldName("property")
		if prop == nil || nodeText(prop, src) != "addEventListener" {
			return
		}
		args := n.ChildByFieldName("arguments")
		if args == nil || args.NamedChildCount() < 2 {
			return
		}
		eventName := args.NamedChild(0)
		if eventName == nil || eventName.Type() != "string" || !highFreqEvents[strings.Trim(eventName.Content(src), "'\"")] {
			return
		}
		handler := args.NamedChild(1)
		var handlerBody *sitter.Node
		if handler.Type() == "arrow_function" || handler.Type() == "function_expression" {
			handlerBody = handler.ChildByFieldName("body")
		} else if handler.Type() == "identifier" {
			handlerBody = findFunctionBody(handler, root, src)
		}
		if handlerBody == nil {
			return
		}
		hasDirectSetter := false
		walkReact(handlerBody, func(m *sitter.Node) {
			if hasDirectSetter || m.Type() != "call_expression" {
				return
			}
			hFn := m.ChildByFieldName("function")
			if hFn == nil || hFn.Type() != "identifier" || !setters[nodeText(hFn, src)] {
				return
			}
			hasDirectSetter = true
		})
		if !hasDirectSetter {
			return
		}
		if !insideComponent(n, src) {
			return
		}
		out = append(out, hint(
			"rerender-derived-state-from-hook", "performance", core.SeverityInfo, file,
			int(n.StartPoint().Row)+1,
			"useState setter called directly from a high-frequency event listener (scroll/resize/mousemove) without debounce or throttle. This causes excessive re-renders.",
			"Add debounce/throttle to the event handler, or use a ref + requestAnimationFrame to batch updates.",
		))
	})
	return out
}

func insideComponent(n *sitter.Node, src []byte) bool {
	for p := n.Parent(); p != nil; p = p.Parent() {
		switch p.Type() {
		case "function_declaration", "method_definition":
			if n := p.ChildByFieldName("name"); n != nil && (isComponentName(n.Content(src)) || isHookName(n.Content(src))) {
				return true
			}
		case "function_expression", "arrow_function":
			parent := p.Parent()
			if parent != nil && parent.Type() == "variable_declarator" {
				if nName := parent.ChildByFieldName("name"); nName != nil && (isComponentName(nName.Content(src)) || isHookName(nName.Content(src))) {
					return true
				}
			}
		}
	}
	return false
}

func findFunctionBody(idNode, root *sitter.Node, src []byte) *sitter.Node {
	name := idNode.Content(src)
	var body *sitter.Node
	walkReact(root, func(n *sitter.Node) {
		if body != nil {
			return
		}
		if n.Type() != "lexical_declaration" && n.Type() != "variable_declaration" {
			return
		}
		for i := 0; i < int(n.NamedChildCount()); i++ {
			decl := n.NamedChild(i)
			if decl.Type() != "variable_declarator" {
				continue
			}
			nName := decl.ChildByFieldName("name")
			if nName == nil || nodeText(nName, src) != name {
				continue
			}
			val := decl.ChildByFieldName("value")
			if val == nil || (val.Type() != "arrow_function" && val.Type() != "function_expression") {
				continue
			}
			body = val.ChildByFieldName("body")
			return
		}
	})
	return body
}

func detectNoUseMemoSimpleExpression(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding {
	var out []core.Finding
	walkReact(root, func(node *sitter.Node) {
		if node.Type() != "call_expression" {
			return
		}
		callee := node.ChildByFieldName("function")
		if callee == nil || nodeText(callee, src) != "useMemo" {
			return
		}
		args := node.ChildByFieldName("arguments")
		if args == nil || args.NamedChildCount() < 2 {
			return
		}
		cb := args.NamedChild(0)
		if cb.Type() != "arrow_function" && cb.Type() != "function_expression" {
			return
		}
		cbBody := cb.ChildByFieldName("body")
		if cbBody == nil {
			return
		}
		if cbBody.Type() == "statement_block" {
			return
		}
		if isSimpleExpression(cbBody, src) {
			deps := args.NamedChild(1)
			depNames := collectDepNames(deps, src)
			freeVars := collectFreeVars(cbBody, src)
			if len(freeVars) > 0 && allVarsInDeps(freeVars, depNames) {
				out = append(out, hint(
					"no-usememo-simple-expression", "performance", core.SeverityInfo, file,
					int(node.StartPoint().Row)+1,
					"useMemo wraps a trivial expression that only uses its dependency variables. The expression is cheap to recompute and memoization adds overhead.",
					"Remove useMemo and assign the expression directly: const result = ...",
				))
			}
		}
	})
	return out
}

func isSimpleExpression(node *sitter.Node, src []byte) bool {
	if node == nil {
		return false
	}
	switch node.Type() {
	case "binary_expression", "unary_expression", "member_expression", "identifier",
		"number", "string", "true", "false", "null", "undefined",
		"template_string", "parenthesized_expression", "subscript_expression",
		"ternary_expression":
		return true
	case "call_expression":
		return false
	default:
		return false
	}
}

func collectDepNames(depsNode *sitter.Node, src []byte) map[string]bool {
	out := map[string]bool{}
	if depsNode.Type() != "array" {
		return out
	}
	for i := 0; i < int(depsNode.NamedChildCount()); i++ {
		el := depsNode.NamedChild(i)
		if el != nil && el.Type() == "identifier" {
			out[el.Content(src)] = true
		}
	}
	return out
}

func collectFreeVars(node *sitter.Node, src []byte) map[string]bool {
	out := map[string]bool{}
	walkReact(node, func(n *sitter.Node) {
		if n.Type() != "identifier" {
			return
		}
		// Skip property names in member expressions (obj.prop → skip "prop").
		// In subscript expressions both object and index are value positions
		// (obj[idx] → both are free vars); a string-literal index isn't an
		// identifier so it's skipped naturally.
		parent := n.Parent()
		if parent != nil && parent.Type() == "member_expression" && parent.ChildByFieldName("property") == n {
			return
		}
		out[n.Content(src)] = true
	})
	return out
}

func allVarsInDeps(vars map[string]bool, deps map[string]bool) bool {
	for v := range vars {
		if !deps[v] {
			return false
		}
	}
	return len(vars) > 0
}

func callsSetter(node *sitter.Node, lang *sitter.Language, src []byte) bool {
	found := false
	_ = runQuery(setterCallQuery, node, lang, func(_ string, id *sitter.Node) {
		if setterRe.MatchString(id.Content(src)) {
			found = true
		}
	})
	return found
}

func detectUseEffectFetchSuggestQuery(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding {
	var out []core.Finding
	walkReact(root, func(node *sitter.Node) {
		if node.Type() != "call_expression" {
			return
		}
		fnNode := node.ChildByFieldName("function")
		if fnNode == nil || nodeText(fnNode, src) != "useEffect" {
			return
		}
		argsNode := node.ChildByFieldName("arguments")
		if argsNode == nil || argsNode.NamedChildCount() == 0 {
			return
		}
		cb := argsNode.NamedChild(0)
		if cb == nil {
			return
		}

		// Check if callback body calls fetch or axios
		hasFetch := false
		walkReact(cb, func(n *sitter.Node) {
			if hasFetch || n.Type() != "call_expression" {
				return
			}
			callee := n.ChildByFieldName("function")
			if callee == nil {
				return
			}
			calleeText := nodeText(callee, src)
			if calleeText == "fetch" || calleeText == "axios" || strings.HasPrefix(calleeText, "axios.") {
				hasFetch = true
			}
		})

		if hasFetch {
			out = append(out, hint(
				"react-useeffect-fetch-suggest-query", "quality", core.SeverityInfo, file,
				int(node.StartPoint().Row)+1,
				"useEffect is performing basic client-side data fetching. Consider using TanStack Query (useQuery) for robust state caching, caching strategies, automatic retries, and clean deduplication.",
				"Migrate this fetch to a TanStack Query hook.",
			))
		}
	})
	return out
}
