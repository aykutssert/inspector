package reacthint

import (
	"strconv"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/aykutssert/scout/internal/architecture/complexity"
	"github.com/aykutssert/scout/internal/architecture/splitting"
	"github.com/aykutssert/scout/internal/core"
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
	var entities []complexity.Entity
	type details struct {
		lines int
		hooks int
		props int
	}
	entDetails := make(map[string]details)

	_ = runMatches(componentQuery, root, lang, func(caps map[string]*sitter.Node) {
		name := nodeText(caps["name"], src)
		fn := caps["fn"]
		if fn == nil || !isComponentName(name) {
			return
		}
		lines := int(fn.EndPoint().Row-fn.StartPoint().Row) + 1
		hooks := countReactHooks(fn, lang, src)
		props := countComponentProps(fn, src)

		entities = append(entities, complexity.Entity{
			Name:      name,
			Type:      "component",
			LineCount: lines,
			Inputs:    props,
			Deps:      hooks,
			StartLine: int(fn.StartPoint().Row) + 1,
		})
		entDetails[name] = details{lines: lines, hooks: hooks, props: props}
	})

	if len(entities) == 0 {
		return nil
	}

	rule := complexity.Rule{
		ID:        "god-component",
		Type:      "component",
		MaxLines:  godComponentVeryLargeLines,
		MaxInputs: godComponentPropThreshold,
		MaxDeps:   godComponentHookThreshold,
	}

	violations := complexity.Analyze(file, entities, []complexity.Rule{rule})

	var out []core.Finding
	for _, v := range violations {
		// Recover details for nice messaging
		d := entDetails[v.RuleID] // Wait, v.RuleID is the Rule.ID ("god-component"), we need the entity name. Let's find it.
		// Let's iterate entities to find the name matching v.Line
		var name string
		for _, ent := range entities {
			if ent.StartLine == v.Line {
				name = ent.Name
				d = entDetails[name]
				break
			}
		}

		out = append(out, hint(
			"god-component", "quality", core.SeverityInfo, file,
			v.Line,
			name+" is large/complex ("+strconv.Itoa(d.lines)+" lines, "+strconv.Itoa(d.hooks)+" hooks, "+strconv.Itoa(d.props)+" props); this is hard to review and weak context for agents.",
			"Split unrelated UI into child components and move stateful behavior into focused custom hooks.",
		))
	}
	return out
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

func detectComponentSplitting(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding {
	var comps []splitting.Component
	exportedNames := collectExportedNames(root, lang, src)

	_ = runMatches(componentQuery, root, lang, func(caps map[string]*sitter.Node) {
		name := nodeText(caps["name"], src)
		fn := caps["fn"]
		if fn == nil || !isComponentName(name) {
			return
		}

		lineCount := countNodeCodeLines(fn, src)

		isExported := exportedNames[name]
		if !isExported {
			for curr := fn; curr != nil; curr = curr.Parent() {
				if curr.Type() == "export_statement" {
					isExported = true
					break
				}
			}
		}

		comps = append(comps, splitting.Component{
			Name:       name,
			LineCount:  lineCount,
			IsExported: isExported,
			StartLine:  int(fn.StartPoint().Row) + 1,
		})
	})

	if len(comps) == 0 {
		return nil
	}

	rule := splitting.Rule{
		ID:                    "component-splitting",
		MaxFileLines:          300,
		MaxComponentLines:     150,
		MaxExportedLargeComps: 1,
		Message:               "File contains multiple large exported components; split them into individual files to reduce agent context size and improve maintainability.",
	}

	fileMetrics := splitting.FileMetrics{
		FilePath:   file,
		TotalLines: countFileCodeLines(src),
		Components: comps,
	}

	violations := splitting.Analyze([]splitting.FileMetrics{fileMetrics}, []splitting.Rule{rule})

	var out []core.Finding
	for _, v := range violations {
		out = append(out, hint(
			v.RuleID, "quality", core.SeverityInfo, v.File, v.Line,
			v.Message,
			"Split large exported components into separate files.",
		))
	}

	return out
}

func collectExportedNames(root *sitter.Node, lang *sitter.Language, src []byte) map[string]bool {
	exported := make(map[string]bool)
	q := `
	(export_specifier name: (identifier) @name)
	(export_statement value: (identifier) @name)
	(export_statement (identifier) @name)
	`
	_ = runQuery(q, root, lang, func(name string, node *sitter.Node) {
		if node != nil {
			exported[node.Content(src)] = true
		}
	})
	return exported
}

func countNodeCodeLines(node *sitter.Node, src []byte) int {
	startRow := int(node.StartPoint().Row)
	endRow := int(node.EndPoint().Row)
	allLines := strings.Split(string(src), "\n")
	count := 0
	for i := startRow; i <= endRow && i < len(allLines); i++ {
		line := strings.TrimSpace(allLines[i])
		if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "/*") || strings.HasPrefix(line, "*") {
			continue
		}
		count++
	}
	return count
}

func countFileCodeLines(src []byte) int {
	allLines := strings.Split(string(src), "\n")
	count := 0
	for _, l := range allLines {
		line := strings.TrimSpace(l)
		if line == "" || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "/*") || strings.HasPrefix(line, "*") {
			continue
		}
		count++
	}
	return count
}
