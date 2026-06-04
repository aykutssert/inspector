package reacthint

import (
	"regexp"
	"strconv"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/aykutssert/inspector/internal/core"
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
func callsSetter(node *sitter.Node, lang *sitter.Language, src []byte) bool {
	found := false
	_ = runQuery(setterCallQuery, node, lang, func(_ string, id *sitter.Node) {
		if setterRe.MatchString(id.Content(src)) {
			found = true
		}
	})
	return found
}
