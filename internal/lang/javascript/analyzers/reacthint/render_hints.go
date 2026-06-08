package reacthint

// Render-phase and event-handler hint detectors.
//
// Rules implemented:
//   - rendering-hydration-no-flicker (#49): useState(false) + setX(true) in [] effect
//   - rerender-transitions-scroll    (#53): setState directly in onScroll handler
//   - advanced-event-handler-refs    (#76): addEventListener with handler in dep array

import (
	sitter "github.com/smacker/go-tree-sitter"

	"github.com/aykutssert/scout/internal/core"
)

// ─── rendering-hydration-no-flicker (#49) ─────────────────────────────────────

// detectHydrationNoFlicker flags the "mounted" boolean state pattern that
// causes a visible flash on first render:
//
//	const [mounted, setMounted] = useState(false)
//	useEffect(() => { setMounted(true) }, [])
//
// Components that render null or a spinner until "mounted" show a blank or
// incorrect frame on hydration. Use useSyncExternalStore for client-only values
// or suppressHydrationWarning for unavoidable mismatches.
func detectHydrationNoFlicker(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding {
	if isEffectPerformanceTestFile(file) {
		return nil
	}
	var out []core.Finding
	walkReact(root, func(node *sitter.Node) {
		if !isUseEffectCall(node, src) || !isEmptyDepsArray(node) {
			return
		}
		cb := effectCallback(node)
		if cb == nil {
			return
		}
		setterName, argText := singleSetterCall(cb, src)
		if setterName == "" || argText != "true" {
			return
		}
		// Confirm a corresponding useState(false) exists in the same scope.
		scope := enclosingFunction(node)
		if scope == nil {
			return
		}
		stateDecls := collectStateDecls(scope, lang, src)
		for _, sd := range stateDecls {
			if sd.setterVar != setterName {
				continue
			}
			out = append(out, hint(
				"rendering-hydration-no-flicker", "performance", core.SeverityInfo, file,
				int(node.StartPoint().Row)+1,
				"useEffect sets "+setterName+"(true) on mount — components that gate rendering on this "+
					"'mounted' flag produce a blank frame or spinner on every load.",
				"Use useSyncExternalStore for client-only values, or suppressHydrationWarning for "+
					"unavoidable server/client mismatches.",
			))
			break
		}
	})
	return out
}

// singleSetterCall returns the setter name and first argument text when cb's
// body is exactly one setState call (concise arrow or single-statement block).
// Returns ("", "") if the shape doesn't match.
func singleSetterCall(cb *sitter.Node, src []byte) (name, arg string) {
	body := cb.ChildByFieldName("body")
	if body == nil {
		return
	}
	var callExpr *sitter.Node
	switch body.Type() {
	case "call_expression":
		callExpr = body
	case "statement_block":
		if body.NamedChildCount() != 1 {
			return
		}
		stmt := body.NamedChild(0)
		if stmt.Type() != "expression_statement" {
			return
		}
		expr := stmt.NamedChild(0)
		if expr == nil || expr.Type() != "call_expression" {
			return
		}
		callExpr = expr
	default:
		return
	}
	fnName := nodeText(callExpr.ChildByFieldName("function"), src)
	if !setterRe.MatchString(fnName) {
		return
	}
	args := callExpr.ChildByFieldName("arguments")
	if args == nil || args.NamedChildCount() != 1 {
		return
	}
	return fnName, nodeText(args.NamedChild(0), src)
}

// ─── rerender-transitions-scroll (#53) ────────────────────────────────────────

// detectTransitionsScroll flags React state setters called directly inside an
// onScroll JSX handler. Scroll events fire at 60-120 fps; each setState call
// schedules a full React re-render, causing jank or dropped frames.
//
//	<div onScroll={e => setScrollY(e.currentTarget.scrollTop)} />
func detectTransitionsScroll(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding {
	if isEffectPerformanceTestFile(file) {
		return nil
	}
	var out []core.Finding
	walkReact(root, func(node *sitter.Node) {
		if node.Type() != "jsx_attribute" {
			return
		}
		if jsxAttributeName(node, src) != "onScroll" {
			return
		}
		// JSX attribute value is wrapped in a jsx_expression node: onScroll={...}
		expr := firstNamedChildOfType(node, "jsx_expression")
		if expr == nil || expr.NamedChildCount() == 0 {
			return
		}
		val := expr.NamedChild(0)
		if val == nil || !isFunctionNode(val) {
			return
		}
		if !callsSetter(val, lang, src) {
			return
		}
		out = append(out, hint(
			"rerender-transitions-scroll", "performance", core.SeverityWarning, file,
			int(node.StartPoint().Row)+1,
			"onScroll handler calls a state setter directly — scroll fires at 60-120 fps, "+
				"scheduling a React re-render on every event.",
			"Wrap with startTransition(() => setState(...)), batch via requestAnimationFrame, "+
				"or (React Native) store in a Reanimated shared value.",
		))
	})
	return out
}

// ─── advanced-event-handler-refs (#76) ────────────────────────────────────────

// detectEventHandlerRefs flags addEventListener calls inside useEffect where the
// handler name appears in the dependency array. When the handler is recreated on
// each render (not wrapped in useCallback or useRef), the effect re-runs on every
// render to remove and re-add the listener — an unnecessary churn.
//
//	useEffect(() => {
//	    window.addEventListener('keydown', handleKey)
//	    return () => window.removeEventListener('keydown', handleKey)
//	}, [handleKey])  // re-subscribes whenever handleKey reference changes
func detectEventHandlerRefs(root *sitter.Node, _ *sitter.Language, src []byte, file string) []core.Finding {
	if isEffectPerformanceTestFile(file) {
		return nil
	}
	var out []core.Finding
	walkReact(root, func(node *sitter.Node) {
		if !isUseEffectCall(node, src) {
			return
		}
		deps := effectDepTexts(node, src)
		if len(deps) == 0 {
			return
		}
		cb := effectCallback(node)
		if cb == nil {
			return
		}
		walkReact(cb, func(inner *sitter.Node) {
			if inner.Type() != "call_expression" {
				return
			}
			if calleeName(inner.ChildByFieldName("function"), src) != "addEventListener" {
				return
			}
			args := inner.ChildByFieldName("arguments")
			if args == nil || args.NamedChildCount() < 2 {
				return
			}
			handlerNode := args.NamedChild(1)
			// Skip inline anonymous functions — those are a separate anti-pattern.
			if isFunctionNode(handlerNode) {
				return
			}
			handlerName := nodeText(handlerNode, src)
			if !sliceContains(deps, handlerName) {
				return
			}
			out = append(out, hint(
				"advanced-event-handler-refs", "performance", core.SeverityInfo, file,
				int(node.StartPoint().Row)+1,
				"addEventListener handler "+handlerName+" is in the dep array — if "+handlerName+
					" is recreated each render, the listener is removed and re-added on every render.",
				"Store the handler in a ref: const ref = useRef("+handlerName+"); and use a stable "+
					"wrapper inside the effect instead of listing the handler in deps.",
			))
		})
	})
	return out
}
