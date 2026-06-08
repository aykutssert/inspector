package reacthint

// Five effect anti-pattern detectors.
//
// Rules implemented:
//   - no-initialize-state      (#85): useEffect(() => setState(x), []) — use lazy initializer
//   - no-mutable-in-deps       (#86): ref.current or location.pathname in dep array
//   - prefer-use-sync-external-store (#92): store.subscribe in useEffect — use useSyncExternalStore
//   - no-cascading-set-state   (#78): 2+ distinct setState calls in one effect
//   - no-self-updating-effect  (#91): effect dep array contains a state var that the effect updates

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/aykutssert/scout/internal/core"
)

// ─── Shared helpers ───────────────────────────────────────────────────────────

// effectCallback returns the first arg of a useEffect call when it's a
// function (arrow or function expression). Returns nil otherwise.
func effectCallback(call *sitter.Node) *sitter.Node {
	args := call.ChildByFieldName("arguments")
	if args == nil || args.NamedChildCount() < 1 {
		return nil
	}
	cb := args.NamedChild(0)
	if cb.Type() != "arrow_function" && cb.Type() != "function_expression" {
		return nil
	}
	return cb
}

// effectDepsNode returns the dep array node (2nd arg), or nil if absent.
func effectDepsNode(call *sitter.Node) *sitter.Node {
	args := call.ChildByFieldName("arguments")
	if args == nil || args.NamedChildCount() < 2 {
		return nil
	}
	return args.NamedChild(1)
}

// effectDepTexts returns the source text of each item in the dep array.
func effectDepTexts(call *sitter.Node, src []byte) []string {
	deps := effectDepsNode(call)
	if deps == nil || deps.Type() != "array" {
		return nil
	}
	var out []string
	for i := 0; i < int(deps.NamedChildCount()); i++ {
		out = append(out, nodeText(deps.NamedChild(i), src))
	}
	return out
}

// isEmptyDepsArray reports whether the useEffect call has [] as its dep array.
func isEmptyDepsArray(call *sitter.Node) bool {
	deps := effectDepsNode(call)
	return deps != nil && deps.Type() == "array" && deps.NamedChildCount() == 0
}

// isUseEffectCall reports whether node is a useEffect(...) call expression.
func isUseEffectCall(node *sitter.Node, src []byte) bool {
	if node.Type() != "call_expression" {
		return false
	}
	return calleeName(node.ChildByFieldName("function"), src) == "useEffect"
}

// ─── stateDecl ────────────────────────────────────────────────────────────────

type stateDecl struct {
	valueVar  string // e.g. "count"
	setterVar string // e.g. "setCount"
}

// stateDestructureQuery matches: const [state, setter] = useState(...)
const stateDestructureQuery = `
(variable_declarator
  name: (array_pattern
    (identifier) @state
    (identifier) @setter)
  value: (call_expression
    function: [(identifier)(member_expression)] @fn)) @decl
`

// collectStateDecls returns all useState destructurings that are directly
// inside fn (not in nested function bodies).
func collectStateDecls(fn *sitter.Node, lang *sitter.Language, src []byte) []stateDecl {
	var decls []stateDecl
	_ = runMatches(stateDestructureQuery, fn, lang, func(caps map[string]*sitter.Node) {
		if calleeName(caps["fn"], src) != "useState" {
			return
		}
		decl := caps["decl"]
		if decl == nil || isInNestedFunction(decl, fn) {
			return
		}
		decls = append(decls, stateDecl{
			valueVar:  nodeText(caps["state"], src),
			setterVar: nodeText(caps["setter"], src),
		})
	})
	return decls
}

// ─── no-initialize-state (#85) ────────────────────────────────────────────────

// detectInitializeState flags:
//
//	useEffect(() => { setState(x) }, [])
//
// where the mount-only effect only sets state. This adds an extra render cycle.
// The fix is a lazy initializer: useState(() => computeInitial()).
func detectInitializeState(root *sitter.Node, _ *sitter.Language, src []byte, file string) []core.Finding {
	if isEffectPerformanceTestFile(file) {
		return nil
	}
	var out []core.Finding
	walkReact(root, func(node *sitter.Node) {
		if !isUseEffectCall(node, src) || !isEmptyDepsArray(node) {
			return
		}
		cb := effectCallback(node)
		if cb == nil || !callbackOnlyCallsSetters(cb, src) {
			return
		}
		out = append(out, hint(
			"no-initialize-state", "quality", core.SeverityInfo, file,
			int(node.StartPoint().Row)+1,
			"useEffect with [] only calls a state setter — this initializes state on mount "+
				"and causes an extra render cycle. Use a lazy initializer instead: useState(() => initialValue()).",
			"Replace useEffect(() => setState(x), []) with useState(() => x).",
		))
	})
	return out
}

// callbackOnlyCallsSetters returns true when every top-level statement in cb
// is a setState call and there are no other side effects.
func callbackOnlyCallsSetters(cb *sitter.Node, src []byte) bool {
	body := cb.ChildByFieldName("body")
	if body == nil {
		return false
	}
	// Concise arrow: () => setState(x)
	if body.Type() == "call_expression" {
		return setterRe.MatchString(nodeText(body.ChildByFieldName("function"), src))
	}
	if body.Type() != "statement_block" {
		return false
	}
	stmtCount := 0
	for i := 0; i < int(body.NamedChildCount()); i++ {
		stmt := body.NamedChild(i)
		if stmt.Type() == "empty_statement" {
			continue
		}
		stmtCount++
		if stmt.Type() != "expression_statement" {
			return false // return stmt, await, etc.
		}
		expr := stmt.NamedChild(0)
		if expr == nil || expr.Type() != "call_expression" {
			return false
		}
		if !setterRe.MatchString(nodeText(expr.ChildByFieldName("function"), src)) {
			return false
		}
	}
	return stmtCount > 0
}

// ─── no-mutable-in-deps (#86) ─────────────────────────────────────────────────

// detectMutableInDeps flags non-reactive values in a useEffect dep array.
// ref.current and location.pathname change without triggering re-renders, so
// the effect will silently not re-run when they change.
func detectMutableInDeps(root *sitter.Node, _ *sitter.Language, src []byte, file string) []core.Finding {
	if isEffectPerformanceTestFile(file) {
		return nil
	}
	var out []core.Finding
	walkReact(root, func(node *sitter.Node) {
		if !isUseEffectCall(node, src) {
			return
		}
		for _, dep := range effectDepTexts(node, src) {
			reason := mutableDepReason(dep)
			if reason == "" {
				continue
			}
			out = append(out, hint(
				"no-mutable-in-deps", "bug", core.SeverityWarning, file,
				int(node.StartPoint().Row)+1,
				"useEffect dependency "+dep+" is not reactive: "+reason+". "+
					"The effect will not re-run when this value changes.",
				"Remove "+dep+" from the dep array; read it inside the effect body via a ref instead.",
			))
		}
	})
	return out
}

var mutableDepPatterns = []struct {
	suffix string
	reason string
}{
	{".current", "ref.current mutates without triggering re-renders"},
	{"location.pathname", "location is not a reactive value in React"},
	{"location.search", "location is not a reactive value in React"},
	{"location.hash", "location is not a reactive value in React"},
	{"location.href", "location is not a reactive value in React"},
	{"window.location.pathname", "window.location is not reactive"},
	{"window.location.href", "window.location is not reactive"},
	{"router.pathname", "router.pathname is a mutable property, not reactive state"},
}

func mutableDepReason(dep string) string {
	for _, p := range mutableDepPatterns {
		if dep == p.suffix || strings.HasSuffix(dep, p.suffix) {
			return p.reason
		}
	}
	return ""
}

// ─── prefer-use-sync-external-store (#92) ────────────────────────────────────

// detectPreferUseSyncExternalStore flags the manual subscribe+cleanup pattern:
//
//	useEffect(() => {
//	    const unsub = store.subscribe(...)
//	    return () => unsub()
//	}, [])
//
// useSyncExternalStore is concurrent-safe; the manual pattern can miss updates
// during concurrent renders in React 18+ (store tearing).
func detectPreferUseSyncExternalStore(root *sitter.Node, _ *sitter.Language, src []byte, file string) []core.Finding {
	if isEffectPerformanceTestFile(file) {
		return nil
	}
	var out []core.Finding
	walkReact(root, func(node *sitter.Node) {
		if !isUseEffectCall(node, src) {
			return
		}
		cb := effectCallback(node)
		if cb == nil || !hasSubscribeCall(cb, src) || !effectHasCleanupReturn(cb) {
			return
		}
		out = append(out, hint(
			"prefer-use-sync-external-store", "quality", core.SeverityInfo, file,
			int(node.StartPoint().Row)+1,
			"useEffect subscribes to an external store with a cleanup function. "+
				"Use useSyncExternalStore instead — it is concurrent-safe and prevents store tearing in React 18+.",
			"Replace with useSyncExternalStore(store.subscribe, store.getSnapshot).",
		))
	})
	return out
}

// hasSubscribeCall reports whether cb's top-level body calls *.subscribe(...).
func hasSubscribeCall(cb *sitter.Node, src []byte) bool {
	found := false
	walkReact(cb, func(node *sitter.Node) {
		if found || node.Type() != "call_expression" || isInNestedFunction(node, cb) {
			return
		}
		if calleeName(node.ChildByFieldName("function"), src) == "subscribe" {
			found = true
		}
	})
	return found
}

// ─── no-cascading-set-state (#78) ─────────────────────────────────────────────

// detectCascadingSetState flags useEffect bodies that call two or more distinct
// state setters. Each setter triggers its own render; related updates should
// use useReducer or React 18 automatic batching.
func detectCascadingSetState(root *sitter.Node, _ *sitter.Language, src []byte, file string) []core.Finding {
	if isEffectPerformanceTestFile(file) {
		return nil
	}
	var out []core.Finding
	walkReact(root, func(node *sitter.Node) {
		if !isUseEffectCall(node, src) {
			return
		}
		cb := effectCallback(node)
		if cb == nil {
			return
		}
		unique := uniqueStrings(topLevelSetterNames(cb, src))
		if len(unique) < 2 {
			return
		}
		out = append(out, hint(
			"no-cascading-set-state", "quality", core.SeverityInfo, file,
			int(node.StartPoint().Row)+1,
			"useEffect calls "+strings.Join(unique, ", ")+" — each setter schedules a separate render. "+
				"Cascading setState in one effect causes unnecessary re-renders.",
			"Consolidate with useReducer dispatch, or split into separate effects that each own one concern.",
		))
	})
	return out
}

// topLevelSetterNames collects the names of state setters called directly
// inside cb (not inside nested functions).
func topLevelSetterNames(cb *sitter.Node, src []byte) []string {
	var names []string
	walkReact(cb, func(node *sitter.Node) {
		if node.Type() != "call_expression" || isInNestedFunction(node, cb) {
			return
		}
		fn := node.ChildByFieldName("function")
		name := nodeText(fn, src)
		if setterRe.MatchString(name) {
			names = append(names, name)
		}
	})
	return names
}

func uniqueStrings(ss []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// ─── no-self-updating-effect (#91) ────────────────────────────────────────────

// detectSelfUpdatingEffect flags useEffect hooks where a state variable is in
// the dep array AND that same state is updated inside the effect. This creates
// an infinite render loop.
//
//	const [count, setCount] = useState(0)
//	useEffect(() => { setCount(count + 1) }, [count]) // infinite loop
func detectSelfUpdatingEffect(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding {
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
		// Scope: immediate enclosing function (component/hook body).
		scope := enclosingFunction(node)
		if scope == nil {
			return
		}
		stateDecls := collectStateDecls(scope, lang, src)
		for _, sd := range stateDecls {
			if !sliceContains(deps, sd.valueVar) {
				continue
			}
			if !callsNamedSetter(cb, src, sd.setterVar) {
				continue
			}
			out = append(out, hint(
				"no-self-updating-effect", "bug", core.SeverityWarning, file,
				int(node.StartPoint().Row)+1,
				"useEffect depends on "+sd.valueVar+" and calls "+sd.setterVar+" inside — "+
					"this creates an infinite update loop: the effect updates state, which triggers the effect again.",
				"Use the functional update form ("+sd.setterVar+"(prev => ...)) and remove "+sd.valueVar+" from deps, "+
					"or restructure the logic to break the cycle.",
			))
		}
	})
	return out
}

// callsNamedSetter reports whether cb calls a specific setter function.
func callsNamedSetter(cb *sitter.Node, src []byte, setterName string) bool {
	found := false
	walkReact(cb, func(node *sitter.Node) {
		if found || node.Type() != "call_expression" {
			return
		}
		if nodeText(node.ChildByFieldName("function"), src) == setterName {
			found = true
		}
	})
	return found
}

func sliceContains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
