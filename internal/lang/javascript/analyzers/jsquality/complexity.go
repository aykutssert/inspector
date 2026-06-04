package jsquality

import (
	"strconv"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/aykutssert/inspector/internal/architecture/complexity"
	"github.com/aykutssert/inspector/internal/core"
)

const classQuery = `(class_declaration name: (_) @name) @class`

const functionQuery = `
(function_declaration name: (_) @name) @fn
(variable_declarator name: (_) @name value: (arrow_function)) @fn
(variable_declarator name: (_) @name value: (function_expression)) @fn
`

func detectComplexity(root *sitter.Node, lang *sitter.Language, src []byte, file string) []core.Finding {
	var entities []complexity.Entity
	type details struct {
		name   string
		lines  int
		inputs int
		deps   int
	}
	entDetails := make(map[int]details)

	_ = runMatchesComplexity(classQuery, root, lang, func(caps map[string]*sitter.Node) {
		classNode := caps["class"]
		nameNode := caps["name"]
		if classNode == nil || nameNode == nil {
			return
		}
		name := nameNode.Content(src)
		lines := countNodeCodeLines(classNode, src)
		deps := countConstructorParams(classNode, src)
		inputs := countClassMethods(classNode, src)
		startLine := int(classNode.StartPoint().Row) + 1

		entities = append(entities, complexity.Entity{
			Name:      name,
			Type:      "class",
			LineCount: lines,
			Inputs:    inputs,
			Deps:      deps,
			StartLine: startLine,
		})
		entDetails[startLine] = details{name: name, lines: lines, inputs: inputs, deps: deps}
	})

	_ = runMatchesComplexity(functionQuery, root, lang, func(caps map[string]*sitter.Node) {
		fnNode := caps["fn"]
		nameNode := caps["name"]
		if fnNode == nil || nameNode == nil {
			return
		}
		name := nameNode.Content(src)
		if isComponentName(name) {
			return
		}

		lines := countNodeCodeLines(fnNode, src)
		inputs := countFunctionParams(fnNode)
		deps := countExternalCalls(fnNode)
		startLine := int(fnNode.StartPoint().Row) + 1

		entities = append(entities, complexity.Entity{
			Name:      name,
			Type:      "function",
			LineCount: lines,
			Inputs:    inputs,
			Deps:      deps,
			StartLine: startLine,
		})
		entDetails[startLine] = details{name: name, lines: lines, inputs: inputs, deps: deps}
	})

	if len(entities) == 0 {
		return nil
	}

	rules := []complexity.Rule{
		{
			ID:        "god-class",
			Type:      "class",
			MaxLines:  200,
			MaxInputs: 10,
			MaxDeps:   8,
			Message:   "Class is too complex; split it or extract concerns into smaller helper modules.",
		},
		{
			ID:        "large-function",
			Type:      "function",
			MaxLines:  100,
			MaxInputs: 5,
			MaxDeps:   6,
			Message:   "Function is too complex; split it into smaller utility functions.",
		},
	}

	violations := complexity.Analyze(file, entities, rules)

	var out []core.Finding
	for _, v := range violations {
		d := entDetails[v.Line]
		entityTypeDisplay := "Class"
		ruleID := "god-class"
		severity := core.SeverityInfo
		fix := "Extract class dependencies or refactor methods to reduce public surface."

		if v.RuleID == "large-function" {
			entityTypeDisplay = "Function"
			ruleID = "large-function"
			fix = "Extract parts of the function to smaller, single-responsibility helper functions."
		}

		out = append(out, core.Finding{
			Analyzer:   "js-quality",
			RuleID:     ruleID,
			Severity:   severity,
			Level:      severity.String(),
			Category:   "quality",
			Confidence: core.ConfidenceHint,
			Line:       v.Line,
			File:       file,
			Message:    d.name + " is a complex " + entityTypeDisplay + " (" + strconv.Itoa(d.lines) + " lines, " + strconv.Itoa(d.inputs) + " inputs, " + strconv.Itoa(d.deps) + " dependencies); this is hard to maintain and review.",
			Fix:        fix,
		})
	}

	return out
}

func findClassBody(classNode *sitter.Node) *sitter.Node {
	for i := 0; i < int(classNode.NamedChildCount()); i++ {
		ch := classNode.NamedChild(i)
		if ch.Type() == "class_body" {
			return ch
		}
	}
	return nil
}

func findFormalParameters(fnNode *sitter.Node) *sitter.Node {
	params := fnNode.ChildByFieldName("parameters")
	if params != nil {
		return params
	}
	for i := 0; i < int(fnNode.NamedChildCount()); i++ {
		ch := fnNode.NamedChild(i)
		if ch.Type() == "formal_parameters" {
			return ch
		}
	}
	return nil
}

func countConstructorParams(classNode *sitter.Node, src []byte) int {
	body := findClassBody(classNode)
	if body == nil {
		return 0
	}
	for i := 0; i < int(body.NamedChildCount()); i++ {
		ch := body.NamedChild(i)
		if ch.Type() == "method_definition" {
			nameNode := ch.ChildByFieldName("name")
			if nameNode != nil && nameNode.Content(src) == "constructor" {
				params := findFormalParameters(ch)
				if params != nil {
					return int(params.NamedChildCount())
				}
			}
		}
	}
	return 0
}

func countClassMethods(classNode *sitter.Node, src []byte) int {
	body := findClassBody(classNode)
	if body == nil {
		return 0
	}
	count := 0
	for i := 0; i < int(body.NamedChildCount()); i++ {
		ch := body.NamedChild(i)
		if ch.Type() == "method_definition" {
			nameNode := ch.ChildByFieldName("name")
			if nameNode != nil && nameNode.Content(src) != "constructor" {
				count++
			}
		}
	}
	return count
}

func countFunctionParams(fnNode *sitter.Node) int {
	params := findFormalParameters(fnNode)
	if params != nil {
		return int(params.NamedChildCount())
	}
	return 0
}

func countExternalCalls(fnNode *sitter.Node) int {
	count := 0
	walkComplexity(fnNode, func(n *sitter.Node) {
		if n.Type() == "call_expression" {
			count++
		}
	})
	return count
}

func walkComplexity(n *sitter.Node, fn func(*sitter.Node)) {
	if n == nil {
		return
	}
	fn(n)
	for i := 0; i < int(n.NamedChildCount()); i++ {
		walkComplexity(n.NamedChild(i), fn)
	}
}

func isComponentName(name string) bool {
	return name != "" && name[0] >= 'A' && name[0] <= 'Z'
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

func runMatchesComplexity(q string, root *sitter.Node, lang *sitter.Language, fn func(caps map[string]*sitter.Node)) error {
	query, err := sitter.NewQuery([]byte(q), lang)
	if err != nil {
		return err
	}
	defer query.Close()
	qc := sitter.NewQueryCursor()
	defer qc.Close()
	qc.Exec(query, root)
	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}
		caps := map[string]*sitter.Node{}
		for _, c := range m.Captures {
			caps[query.CaptureNameForId(c.Index)] = c.Node
		}
		fn(caps)
	}
	return nil
}
