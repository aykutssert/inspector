package reacthint

import (
	"regexp"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/aykutssert/scout/internal/core"
)

type effectInfo struct {
	call    *sitter.Node
	cb      *sitter.Node
	deps    []string
	setters []setterCall
}

type setterCall struct {
	name string
	arg  string
	node *sitter.Node
}

var loadingStateRe = regexp.MustCompile(`(?i)(^|[_.])(is)?(loading|pending|submitting|saving|sending|working|busy)$`)
var triggerStateNameRe = regexp.MustCompile(`(?i)(trigger|submit|click|request|open|close|confirm|ready|sent|run)$`)

func detectChainStateUpdates(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding {
	if isEffectPerformanceTestFile(file) {
		return nil
	}
	var out []core.Finding
	_ = runMatches(componentQuery, root, lang, func(caps map[string]*sitter.Node) {
		fn := funcNode(caps["fn"])
		name := nodeText(caps["name"], src)
		if fn == nil || !isComponentOrHook(name) {
			return
		}
		effects := collectEffectInfos(fn, src)
		if len(effects) < 2 {
			return
		}
		stateBySetter := stateBySetterMap(collectStateDecls(fn, lang, src))
		for i, dst := range effects {
			if len(dst.setters) == 0 {
				continue
			}
			for j, srcEff := range effects {
				if i == j {
					continue
				}
				for _, sc := range srcEff.setters {
					stateVar := stateBySetter[sc.name]
					if stateVar != "" && sliceContains(dst.deps, stateVar) {
						out = append(out, hint(
							"no-chain-state-updates", "quality", core.SeverityInfo, file,
							int(dst.call.StartPoint().Row)+1,
							"This useEffect updates state from another state that is itself produced by a different useEffect. Chaining state updates across effects adds extra render passes and makes the update flow harder to follow.",
							"Collapse the flow into one event handler, reducer, or one derived computation instead of effect-to-effect state hops.",
						))
						return
					}
				}
			}
		}
	})
	return out
}

func detectEffectChain(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding {
	if isEffectPerformanceTestFile(file) {
		return nil
	}
	var out []core.Finding
	_ = runMatches(componentQuery, root, lang, func(caps map[string]*sitter.Node) {
		fn := funcNode(caps["fn"])
		name := nodeText(caps["name"], src)
		if fn == nil || !isComponentOrHook(name) {
			return
		}
		effects := collectEffectInfos(fn, src)
		if len(effects) < 3 {
			return
		}
		stateBySetter := stateBySetterMap(collectStateDecls(fn, lang, src))
		for mid := range effects {
			if len(effects[mid].setters) == 0 {
				continue
			}
			hasIncoming := false
			hasOutgoing := false
			for i := range effects {
				if i == mid {
					continue
				}
				if effectFeedsEffect(effects[i], effects[mid], stateBySetter) {
					hasIncoming = true
				}
				if effectFeedsEffect(effects[mid], effects[i], stateBySetter) {
					hasOutgoing = true
				}
			}
			if hasIncoming && hasOutgoing {
				out = append(out, hint(
					"no-effect-chain", "quality", core.SeverityInfo, file,
					int(effects[mid].call.StartPoint().Row)+1,
					"This useEffect sits in a multi-step effect chain: one effect produces state that triggers another effect, which then triggers another. Effect chains create hidden control flow and extra renders.",
					"Replace the chain with one event path, a reducer, or a directly derived value.",
				))
				return
			}
		}
	})
	return out
}

func detectEffectEventHandler(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding {
	if isEffectPerformanceTestFile(file) {
		return nil
	}
	var out []core.Finding
	_ = runMatches(componentQuery, root, lang, func(caps map[string]*sitter.Node) {
		fn := funcNode(caps["fn"])
		name := nodeText(caps["name"], src)
		if fn == nil || !isComponentOrHook(name) {
			return
		}
		triggerStates := eventTriggeredStates(fn, lang, src)
		if len(triggerStates) == 0 {
			return
		}
		callbacks := callbackBindingMap(fn.ChildByFieldName("parameters"), src)
		for _, eff := range collectEffectInfos(fn, src) {
			stateVar := primaryTriggeredDep(eff.deps, triggerStates)
			if stateVar == "" || len(eff.setters) > 0 || effectHasCleanupReturn(eff.cb) {
				continue
			}
			if !effectCallsPropCallback(eff.cb, src, callbacks) {
				continue
			}
			out = append(out, hint(
				"no-effect-event-handler", "quality", core.SeverityInfo, file,
				int(eff.call.StartPoint().Row)+1,
				"This useEffect is only reacting to state that was set from an event handler, and then calls a callback prop. That event logic belongs in the handler, not in a follow-up effect.",
				"Call the prop callback directly from the event handler instead of routing through state + useEffect.",
			))
		}
	})
	return out
}

func detectNoEventHandler(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding {
	if isEffectPerformanceTestFile(file) {
		return nil
	}
	var out []core.Finding
	_ = runMatches(componentQuery, root, lang, func(caps map[string]*sitter.Node) {
		fn := funcNode(caps["fn"])
		name := nodeText(caps["name"], src)
		if fn == nil || !isComponentOrHook(name) {
			return
		}
		triggerStates := eventTriggeredStates(fn, lang, src)
		if len(triggerStates) == 0 {
			return
		}
		for _, eff := range collectEffectInfos(fn, src) {
			stateVar := primaryTriggeredDep(eff.deps, triggerStates)
			if stateVar == "" || len(eff.setters) > 0 || effectHasCleanupReturn(eff.cb) {
				continue
			}
			if !effectHasNonSetterCall(eff.cb, src) || effectCallsPropCallback(eff.cb, src, nil) {
				continue
			}
			out = append(out, hint(
				"no-event-handler", "quality", core.SeverityInfo, file,
				int(eff.call.StartPoint().Row)+1,
				"This useEffect exists only to run side effects after an event handler flips local state. That splits one event flow across render and effect phases.",
				"Run the side effect directly in the event handler, or wrap the async branch in one handler function.",
			))
		}
	})
	return out
}

func detectEventTriggerState(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding {
	if isEffectPerformanceTestFile(file) {
		return nil
	}
	var out []core.Finding
	_ = runMatches(componentQuery, root, lang, func(caps map[string]*sitter.Node) {
		fn := funcNode(caps["fn"])
		name := nodeText(caps["name"], src)
		if fn == nil || !isComponentOrHook(name) {
			return
		}
		triggerStates := eventTriggeredStates(fn, lang, src)
		if len(triggerStates) == 0 {
			return
		}
		inits := stateInitMap(fn, lang, src)
		for _, eff := range collectEffectInfos(fn, src) {
			stateVar := primaryTriggeredDep(eff.deps, triggerStates)
			if stateVar == "" || len(eff.setters) > 0 {
				continue
			}
			if (!isTriggerLikeInit(inits[stateVar]) && !triggerStateNameRe.MatchString(stateVar)) || !strings.Contains(nodeText(eff.cb, src), stateVar) {
				continue
			}
			out = append(out, hint(
				"no-event-trigger-state", "quality", core.SeverityInfo, file,
				int(eff.call.StartPoint().Row)+1,
				"This state value is used as a trigger to kick off a useEffect after an event. Trigger-only state adds an extra render just to schedule work.",
				"Remove the trigger state and run the logic from the event handler itself.",
			))
		}
	})
	return out
}

func detectPassLiveStateToParent(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding {
	if isEffectPerformanceTestFile(file) {
		return nil
	}
	var out []core.Finding
	_ = runMatches(componentQuery, root, lang, func(caps map[string]*sitter.Node) {
		fn := funcNode(caps["fn"])
		name := nodeText(caps["name"], src)
		if fn == nil || !isComponentOrHook(name) {
			return
		}
		callbacks := callbackBindingMap(fn.ChildByFieldName("parameters"), src)
		if len(callbacks) == 0 {
			return
		}
		stateVars := stateVarSet(collectStateDecls(fn, lang, src))
		for _, eff := range collectEffectInfos(fn, src) {
			if call := firstEffectPropCallbackCall(eff.cb, src, callbacks, stateVars, true); call != nil {
				out = append(out, hint(
					"no-pass-live-state-to-parent", "design", core.SeverityInfo, file,
					int(call.StartPoint().Row)+1,
					"This useEffect pushes live local state up to the parent through a callback prop. That synchronizes parent/child state through render-followed effects.",
					"Lift the shared state up, or notify the parent from the event that changed the state instead of from useEffect.",
				))
			}
		}
	})
	return out
}

func detectPropCallbackInEffect(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding {
	if isEffectPerformanceTestFile(file) {
		return nil
	}
	var out []core.Finding
	_ = runMatches(componentQuery, root, lang, func(caps map[string]*sitter.Node) {
		fn := funcNode(caps["fn"])
		name := nodeText(caps["name"], src)
		if fn == nil || !isComponentOrHook(name) {
			return
		}
		callbacks := callbackBindingMap(fn.ChildByFieldName("parameters"), src)
		if len(callbacks) == 0 {
			return
		}
		stateVars := stateVarSet(collectStateDecls(fn, lang, src))
		for _, eff := range collectEffectInfos(fn, src) {
			if call := firstEffectPropCallbackCall(eff.cb, src, callbacks, stateVars, false); call != nil {
				out = append(out, hint(
					"no-prop-callback-in-effect", "design", core.SeverityInfo, file,
					int(call.StartPoint().Row)+1,
					"This useEffect calls a callback prop. Effect-driven parent callbacks usually indicate control flow split across render and commit phases.",
					"Prefer calling the callback from the event handler or lifting the coordination into shared state/context.",
				))
			}
		}
	})
	return out
}

func detectResetAllStateOnPropChange(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding {
	if isEffectPerformanceTestFile(file) {
		return nil
	}
	var out []core.Finding
	_ = runMatches(componentQuery, root, lang, func(caps map[string]*sitter.Node) {
		fn := funcNode(caps["fn"])
		name := nodeText(caps["name"], src)
		if fn == nil || !isComponentOrHook(name) {
			return
		}
		stateVars := stateVarSet(collectStateDecls(fn, lang, src))
		for _, eff := range collectEffectInfos(fn, src) {
			if len(eff.deps) == 0 || len(eff.setters) < 2 || !callbackOnlyCallsSetters(eff.cb, src) {
				continue
			}
			propDriven := true
			for _, dep := range eff.deps {
				if stateVars[dep] {
					propDriven = false
					break
				}
			}
			if !propDriven || !allResetLike(eff.setters) {
				continue
			}
			out = append(out, hint(
				"no-reset-all-state-on-prop-change", "quality", core.SeverityInfo, file,
				int(eff.call.StartPoint().Row)+1,
				"This useEffect resets multiple local state values when a prop changes. Resetting a whole component tree through effects is brittle and causes extra renders.",
				"If the prop change should recreate the component state, give the child a key derived from that prop instead of manually resetting each state slice.",
			))
		}
	})
	return out
}

func detectHoistJSX(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding {
	var out []core.Finding
	_ = runMatches(componentQuery, root, lang, func(caps map[string]*sitter.Node) {
		fn := funcNode(caps["fn"])
		name := nodeText(caps["name"], src)
		if fn == nil || !isComponentName(name) {
			return
		}
		body := fn.ChildByFieldName("body")
		if body == nil || body.Type() != "statement_block" {
			return
		}
		for i := 0; i < int(body.NamedChildCount()); i++ {
			stmt := body.NamedChild(i)
			if stmt.Type() != "lexical_declaration" && stmt.Type() != "variable_declaration" {
				continue
			}
			for j := 0; j < int(stmt.NamedChildCount()); j++ {
				decl := stmt.NamedChild(j)
				if decl.Type() != "variable_declarator" {
					continue
				}
				nameNode := decl.ChildByFieldName("name")
				val := decl.ChildByFieldName("value")
				if nameNode == nil || val == nil || !isStaticJSXNode(val) {
					continue
				}
				varName := nodeText(nameNode, src)
				if !isReferencedInReturn(body, varName, src) {
					continue
				}
				out = append(out, hint(
					"rendering-hoist-jsx", "performance", core.SeverityInfo, file,
					int(decl.StartPoint().Row)+1,
					"Static JSX is created inside the component on every render. This JSX does not depend on props or state and can be hoisted once at module scope.",
					"Move the static JSX constant outside the component so React reuses the same element tree.",
				))
			}
		}
	})
	return out
}

func detectUseTransitionLoading(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding {
	var out []core.Finding
	_ = runMatches(componentQuery, root, lang, func(caps map[string]*sitter.Node) {
		fn := funcNode(caps["fn"])
		name := nodeText(caps["name"], src)
		if fn == nil || !isComponentName(name) {
			return
		}
		decls := collectStateDecls(fn, lang, src)
		if len(decls) == 0 {
			return
		}
		stateBySetter := stateBySetterMap(decls)
		handlers := collectEventHandlers(fn, lang, src)
		for _, handler := range handlers {
			if !functionLooksAsync(handler, src) || !hasAwaitExpression(handler) {
				continue
			}
			calls := topLevelSetterCalls(handler, src)
			for _, sd := range decls {
				if !loadingStateRe.MatchString(sd.valueVar) || stateBySetter[sd.setterVar] == "" {
					continue
				}
				if hasSetterToggle(calls, sd.setterVar) {
					out = append(out, hint(
						"rendering-usetransition-loading", "performance", core.SeverityInfo, file,
						int(handler.StartPoint().Row)+1,
						"Async event handler toggles a loading/pending state before and after await. For transition-style UI updates, useTransition avoids urgent re-renders and keeps interaction responsive.",
						"Consider const [isPending, startTransition] = useTransition() and wrap the non-urgent update in startTransition(...).",
					))
					return
				}
			}
		}
	})
	return out
}

func detectMemoBeforeEarlyReturn(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding {
	var out []core.Finding
	_ = runMatches(componentQuery, root, lang, func(caps map[string]*sitter.Node) {
		fn := funcNode(caps["fn"])
		name := nodeText(caps["name"], src)
		if fn == nil || !isComponentName(name) {
			return
		}
		body := fn.ChildByFieldName("body")
		if body == nil || body.Type() != "statement_block" {
			return
		}
		var memoDecls []*sitter.Node
		for i := 0; i < int(body.NamedChildCount()); i++ {
			stmt := body.NamedChild(i)
			if isUseMemoDecl(stmt, src) {
				memoDecls = append(memoDecls, stmt)
				continue
			}
			if len(memoDecls) == 0 || stmt.Type() != "if_statement" || !hasEarlyReturn(stmt) {
				continue
			}
			for _, decl := range memoDecls {
				out = append(out, hint(
					"rerender-memo-before-early-return", "performance", core.SeverityInfo, file,
					int(decl.StartPoint().Row)+1,
					"useMemo runs before an early-return guard. When the guard exits the component, the memoized computation still runs even though nothing is rendered.",
					"Move the early return above the useMemo call, or split the guarded branch into a child component.",
				))
			}
			return
		}
	})
	return out
}

func collectEffectInfos(scope *sitter.Node, src []byte) []effectInfo {
	var out []effectInfo
	walkReact(scope, func(node *sitter.Node) {
		if !isUseEffectCall(node, src) || isInNestedFunction(node, scope) {
			return
		}
		cb := effectCallback(node)
		if cb == nil {
			return
		}
		out = append(out, effectInfo{
			call:    node,
			cb:      cb,
			deps:    effectDepTexts(node, src),
			setters: topLevelSetterCalls(cb, src),
		})
	})
	return out
}

func effectFeedsEffect(srcEff, dstEff effectInfo, stateBySetter map[string]string) bool {
	if len(dstEff.setters) == 0 {
		return false
	}
	for _, sc := range srcEff.setters {
		if stateVar := stateBySetter[sc.name]; stateVar != "" && sliceContains(dstEff.deps, stateVar) {
			return true
		}
	}
	return false
}

func topLevelSetterCalls(fn *sitter.Node, src []byte) []setterCall {
	var out []setterCall
	walkReact(fn, func(node *sitter.Node) {
		if node.Type() != "call_expression" || isInNestedFunction(node, fn) {
			return
		}
		name := nodeText(node.ChildByFieldName("function"), src)
		if !setterRe.MatchString(name) {
			return
		}
		arg := ""
		if args := node.ChildByFieldName("arguments"); args != nil && args.NamedChildCount() > 0 {
			arg = nodeText(args.NamedChild(0), src)
		}
		out = append(out, setterCall{name: name, arg: strings.TrimSpace(arg), node: node})
	})
	return out
}

func stateBySetterMap(decls []stateDecl) map[string]string {
	out := make(map[string]string, len(decls))
	for _, sd := range decls {
		out[sd.setterVar] = sd.valueVar
	}
	return out
}

func stateVarSet(decls []stateDecl) map[string]bool {
	out := make(map[string]bool, len(decls))
	for _, sd := range decls {
		out[sd.valueVar] = true
	}
	return out
}

func callbackBindingMap(params *sitter.Node, src []byte) map[string]string {
	out := map[string]string{}
	if params == nil {
		return out
	}
	pattern := firstDescendant(params, "object_pattern")
	if pattern == nil {
		return out
	}
	for _, pb := range callbackBindings(pattern, src) {
		out[pb.binding] = pb.name
	}
	return out
}

func firstEffectPropCallbackCall(cb *sitter.Node, src []byte, callbacks map[string]string, stateVars map[string]bool, requireStateArg bool) *sitter.Node {
	var found *sitter.Node
	walkReact(cb, func(node *sitter.Node) {
		if found != nil || node.Type() != "call_expression" || isInNestedFunction(node, cb) {
			return
		}
		callee := nodeText(node.ChildByFieldName("function"), src)
		if callbacks[callee] == "" {
			return
		}
		if !requireStateArg {
			found = node
			return
		}
		args := node.ChildByFieldName("arguments")
		if args == nil {
			return
		}
		for i := 0; i < int(args.NamedChildCount()); i++ {
			if stateVars[nodeText(args.NamedChild(i), src)] {
				found = node
				return
			}
		}
	})
	return found
}

func effectCallsPropCallback(cb *sitter.Node, src []byte, callbacks map[string]string) bool {
	if callbacks == nil {
		callbacks = map[string]string{}
	}
	return firstEffectPropCallbackCall(cb, src, callbacks, nil, false) != nil
}

func effectHasNonSetterCall(cb *sitter.Node, src []byte) bool {
	found := false
	walkReact(cb, func(node *sitter.Node) {
		if found || node.Type() != "call_expression" || isInNestedFunction(node, cb) {
			return
		}
		name := calleeName(node.ChildByFieldName("function"), src)
		if setterRe.MatchString(name) || name == "useEffect" || name == "useLayoutEffect" {
			return
		}
		found = true
	})
	return found
}

func primaryTriggeredDep(deps []string, triggers map[string]bool) string {
	var stateVar string
	for _, dep := range deps {
		if triggers[dep] {
			if stateVar != "" {
				return ""
			}
			stateVar = dep
			continue
		}
		if strings.HasPrefix(dep, "on") || strings.HasPrefix(dep, "handle") {
			continue
		}
		return ""
	}
	return stateVar
}

func eventTriggeredStates(scope *sitter.Node, lang *sitter.Language, src []byte) map[string]bool {
	stateBySetter := stateBySetterMap(collectStateDecls(scope, lang, src))
	named := collectNamedFunctions(scope, lang, src)
	out := map[string]bool{}
	walkReact(scope, func(node *sitter.Node) {
		if node.Type() != "jsx_attribute" {
			return
		}
		name := jsxAttributeName(node, src)
		if !strings.HasPrefix(name, "on") {
			return
		}
		expr := firstNamedChildOfType(node, "jsx_expression")
		if expr == nil || expr.NamedChildCount() == 0 {
			return
		}
		val := expr.NamedChild(0)
		if val == nil {
			return
		}
		var target *sitter.Node
		if isFunctionNode(val) {
			target = val
		} else if val.Type() == "identifier" {
			target = named[nodeText(val, src)]
		}
		if target == nil {
			return
		}
		for _, sc := range topLevelSetterCalls(target, src) {
			if stateVar := stateBySetter[sc.name]; stateVar != "" {
				out[stateVar] = true
			}
		}
	})
	return out
}

func collectNamedFunctions(scope *sitter.Node, lang *sitter.Language, src []byte) map[string]*sitter.Node {
	out := map[string]*sitter.Node{}
	_ = runMatches(componentQuery, scope, lang, func(caps map[string]*sitter.Node) {
		fn := funcNode(caps["fn"])
		name := nodeText(caps["name"], src)
		if fn == nil || name == "" || isInNestedFunction(fn, scope) {
			return
		}
		out[name] = fn
	})
	return out
}

func stateInitMap(scope *sitter.Node, lang *sitter.Language, src []byte) map[string]string {
	out := map[string]string{}
	_ = runMatches(stateInitQuery, scope, lang, func(caps map[string]*sitter.Node) {
		if calleeName(caps["fn"], src) != "useState" {
			return
		}
		decl := caps["decl"]
		if decl == nil || isInNestedFunction(decl, scope) {
			return
		}
		out[nodeText(caps["state"], src)] = strings.TrimSpace(nodeText(caps["init"], src))
	})
	return out
}

const stateInitQuery = `
(variable_declarator
  name: (array_pattern
    (identifier) @state
    (identifier) @setter)
  value: (call_expression
    function: [(identifier)(member_expression)] @fn
    arguments: (arguments . (_) @init .?))) @decl
`

func isTriggerLikeInit(init string) bool {
	switch init {
	case "false", "null", "undefined", "0", `""`, `''`:
		return true
	default:
		return false
	}
}

func allResetLike(calls []setterCall) bool {
	for _, sc := range calls {
		switch sc.arg {
		case "false", "true", "null", "undefined", "0", `""`, `''`, "[]", "{}":
		default:
			return false
		}
	}
	return len(calls) > 0
}

func isStaticJSXNode(node *sitter.Node) bool {
	if node == nil {
		return false
	}
	switch node.Type() {
	case "jsx_element", "jsx_self_closing_element", "jsx_fragment":
	default:
		return false
	}
	static := true
	walkReact(node, func(n *sitter.Node) {
		if n.Type() == "jsx_expression" {
			static = false
		}
	})
	return static
}

func isReferencedInReturn(body *sitter.Node, name string, src []byte) bool {
	found := false
	walkReact(body, func(node *sitter.Node) {
		if found || node.Type() != "return_statement" {
			return
		}
		if node.NamedChildCount() == 0 {
			return
		}
		walkReact(node.NamedChild(0), func(n *sitter.Node) {
			if n.Type() == "identifier" && nodeText(n, src) == name {
				found = true
			}
		})
	})
	return found
}

func collectEventHandlers(scope *sitter.Node, lang *sitter.Language, src []byte) []*sitter.Node {
	named := collectNamedFunctions(scope, lang, src)
	var out []*sitter.Node
	walkReact(scope, func(node *sitter.Node) {
		if node.Type() != "jsx_attribute" || !strings.HasPrefix(jsxAttributeName(node, src), "on") {
			return
		}
		expr := firstNamedChildOfType(node, "jsx_expression")
		if expr == nil || expr.NamedChildCount() == 0 {
			return
		}
		val := expr.NamedChild(0)
		if val == nil {
			return
		}
		if isFunctionNode(val) {
			out = append(out, val)
			return
		}
		if val.Type() == "identifier" {
			if fn := named[nodeText(val, src)]; fn != nil {
				out = append(out, fn)
			}
		}
	})
	return out
}

func functionLooksAsync(fn *sitter.Node, src []byte) bool {
	return strings.HasPrefix(strings.TrimSpace(nodeText(fn, src)), "async")
}

func hasAwaitExpression(fn *sitter.Node) bool {
	found := false
	walkReact(fn, func(node *sitter.Node) {
		if !found && node.Type() == "await_expression" {
			found = true
		}
	})
	return found
}

func hasSetterToggle(calls []setterCall, setter string) bool {
	hasTrue := false
	hasFalse := false
	for _, sc := range calls {
		if sc.name != setter {
			continue
		}
		switch sc.arg {
		case "true":
			hasTrue = true
		case "false":
			hasFalse = true
		}
	}
	return hasTrue && hasFalse
}

func isUseMemoDecl(stmt *sitter.Node, src []byte) bool {
	if stmt.Type() != "lexical_declaration" && stmt.Type() != "variable_declaration" {
		return false
	}
	for i := 0; i < int(stmt.NamedChildCount()); i++ {
		decl := stmt.NamedChild(i)
		if decl.Type() != "variable_declarator" {
			continue
		}
		val := decl.ChildByFieldName("value")
		if val != nil && val.Type() == "call_expression" && calleeName(val.ChildByFieldName("function"), src) == "useMemo" {
			return true
		}
	}
	return false
}

func hasEarlyReturn(ifNode *sitter.Node) bool {
	for _, field := range []string{"consequence", "alternative"} {
		branch := ifNode.ChildByFieldName(field)
		if branch == nil {
			continue
		}
		if branch.Type() == "return_statement" {
			return true
		}
		if branch.Type() == "statement_block" && branch.NamedChildCount() > 0 {
			if branch.NamedChild(0).Type() == "return_statement" {
				return true
			}
		}
	}
	return false
}
