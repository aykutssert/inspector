package reacthint

import (
	"strconv"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/aykutssert/inspector/internal/core"
)

const (
	godComponentVeryLargeLines = 160
	godComponentBusyLines      = 100
	godComponentHookThreshold  = 8
	godComponentPropThreshold  = 12
	godComponentBusyHooks      = 5
	godComponentBusyProps      = 8
)

// detectGodComponent flags React components whose size or public prop/hook
// surface has crossed a maintainability threshold. This is intentionally a
// hint: large components can be valid, but they are expensive context for an
// agent and usually hide extractable UI or custom hooks.
func detectGodComponent(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding {
	var out []core.Finding
	_ = runMatches(componentQuery, root, lang, func(caps map[string]*sitter.Node) {
		name := nodeText(caps["name"], src)
		fn := caps["fn"]
		if fn == nil || !isComponentName(name) {
			return
		}
		lines := int(fn.EndPoint().Row-fn.StartPoint().Row) + 1
		hooks := countReactHooks(fn, lang, src)
		props := countComponentProps(fn, src)
		if !isGodComponent(lines, hooks, props) {
			return
		}
		out = append(out, hint(
			"god-component", "quality", core.SeverityInfo, file,
			int(fn.StartPoint().Row)+1,
			name+" is large/complex ("+strconv.Itoa(lines)+" lines, "+strconv.Itoa(hooks)+" hooks, "+strconv.Itoa(props)+" props); this is hard to review and weak context for agents.",
			"Split unrelated UI into child components and move stateful behavior into focused custom hooks.",
		))
	})
	return out
}

func isGodComponent(lines, hooks, props int) bool {
	if lines >= godComponentVeryLargeLines {
		return true
	}
	if hooks >= godComponentHookThreshold || props >= godComponentPropThreshold {
		return true
	}
	return lines >= godComponentBusyLines && (hooks >= godComponentBusyHooks || props >= godComponentBusyProps)
}

// isComponentOrHook matches React naming: a component is capitalized, a hook is
// prefixed with "use". Avoids flagging plain helpers that happen to call hooks.
func isComponentOrHook(name string) bool {
	if name == "" {
		return false
	}
	if name[0] >= 'A' && name[0] <= 'Z' {
		return true
	}
	return strings.HasPrefix(name, "use")
}

func isComponentName(name string) bool {
	return name != "" && name[0] >= 'A' && name[0] <= 'Z'
}

func countUseState(node *sitter.Node, lang *sitter.Language, src []byte) int {
	n := 0
	_ = runQuery(setterCallQuery, node, lang, func(_ string, id *sitter.Node) {
		if id.Content(src) == "useState" {
			n++
		}
	})
	return n
}

func countReactHooks(node *sitter.Node, lang *sitter.Language, src []byte) int {
	n := 0
	_ = runQuery(setterCallQuery, node, lang, func(_ string, id *sitter.Node) {
		if isHookName(id.Content(src)) {
			n++
		}
	})
	return n
}

func isHookName(name string) bool {
	return len(name) > 3 && strings.HasPrefix(name, "use") && name[3] >= 'A' && name[3] <= 'Z'
}

func countComponentProps(node *sitter.Node, src []byte) int {
	text := node.Content(src)
	open := strings.Index(text, "{")
	if open == -1 {
		return 0
	}
	close := matchingBrace(text, open)
	if close == -1 || !braceLooksLikeParamDestructure(text, close) {
		return 0
	}
	return countTopLevelCommaItems(text[open+1 : close])
}

func matchingBrace(text string, open int) int {
	depth := 0
	for i := open; i < len(text); i++ {
		switch text[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func braceLooksLikeParamDestructure(text string, close int) bool {
	rest := strings.TrimSpace(text[close+1:])
	if strings.HasPrefix(rest, ")") || strings.HasPrefix(rest, "=>") || strings.HasPrefix(rest, ",") {
		return true
	}
	if !strings.HasPrefix(rest, ":") {
		return false
	}
	closeParen := strings.Index(rest, ")")
	body := strings.Index(rest, "{")
	return closeParen != -1 && (body == -1 || closeParen < body)
}

func countTopLevelCommaItems(text string) int {
	count, depth, hasToken := 0, 0, false
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '{', '[', '(':
			depth++
			hasToken = true
		case '}', ']', ')':
			if depth > 0 {
				depth--
			}
			hasToken = true
		case ',':
			if depth == 0 {
				if hasToken {
					count++
				}
				hasToken = false
				continue
			}
			hasToken = true
		case ' ', '\n', '\r', '\t':
		default:
			hasToken = true
		}
	}
	if hasToken {
		count++
	}
	return count
}
