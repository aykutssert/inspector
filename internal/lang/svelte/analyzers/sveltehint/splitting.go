package sveltehint

import (
	"context"
	"path/filepath"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/typescript/typescript"

	"github.com/aykutssert/scout/internal/architecture/splitting"
	"github.com/aykutssert/scout/internal/core"
)

const componentQuery = `
(function_declaration name: (identifier) @name) @fn
(variable_declarator name: (identifier) @name value: (arrow_function)) @fn
(variable_declarator name: (identifier) @name value: (function_expression)) @fn
`

func isComponentOrHook(name string) bool {
	if name == "" {
		return false
	}
	if name[0] >= 'A' && name[0] <= 'Z' {
		return true
	}
	return strings.HasPrefix(name, "use")
}

func detectComponentSplitting(src []byte, rel string) []core.Finding {
	name := strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))
	fileLines := countSvelteFileCodeLines(src)

	comps := []splitting.Component{
		{
			Name:       name,
			LineCount:  fileLines,
			IsExported: true,
			StartLine:  1,
		},
	}

	for _, block := range scriptBlocks(src) {
		scriptComps := scanSvelteScriptComponents(block)
		comps = append(comps, scriptComps...)
	}

	snippetComps := parseSvelteSnippets(src)
	comps = append(comps, snippetComps...)

	rule := splitting.Rule{
		ID:                    "svelte.component-splitting",
		MaxFileLines:          300,
		MaxComponentLines:     150,
		MaxExportedLargeComps: 1,
		Message:               "Svelte file contains multiple large exported components; split them into individual files to reduce agent context size and improve maintainability.",
	}

	fileMetrics := splitting.FileMetrics{
		FilePath:   rel,
		TotalLines: fileLines,
		Components: comps,
	}

	violations := splitting.Analyze([]splitting.FileMetrics{fileMetrics}, []splitting.Rule{rule})

	var out []core.Finding
	for _, v := range violations {
		out = append(out, core.Finding{
			Analyzer:   "svelte-hint",
			RuleID:     v.RuleID,
			Severity:   core.SeverityWarning,
			Level:      core.SeverityWarning.String(),
			Category:   "quality",
			Confidence: core.ConfidenceHint,
			Line:       v.Line,
			File:       v.File,
			Message:    v.Message,
			Fix:        "Split large exported components or Svelte templates into separate files.",
		})
	}
	return out
}

func scanSvelteScriptComponents(block scriptBlock) []splitting.Component {
	parser := sitter.NewParser()
	defer parser.Close()
	if block.ts {
		parser.SetLanguage(typescript.GetLanguage())
	} else {
		parser.SetLanguage(javascript.GetLanguage())
	}
	pctx, cancel := context.WithTimeout(context.Background(), parseTimeout)
	defer cancel()
	tree, err := parser.ParseCtx(pctx, nil, block.src)
	if err != nil {
		return nil
	}
	defer tree.Close()
	root := tree.RootNode()
	lang := javascript.GetLanguage()
	if block.ts {
		lang = typescript.GetLanguage()
	}

	var comps []splitting.Component
	exportedNames := collectExportedNames(root, lang, block.src)

	_ = runQuery(componentQuery, root, lang, func(caps map[string]*sitter.Node) {
		name := nodeText(caps["name"], block.src)
		fn := caps["fn"]
		if fn == nil || !isComponentOrHook(name) {
			return
		}

		lineCount := countNodeCodeLines(fn, block.src)
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
			StartLine:  block.lineStart + int(fn.StartPoint().Row),
		})
	})

	return comps
}

func parseSvelteSnippets(src []byte) []splitting.Component {
	var comps []splitting.Component
	allLines := strings.Split(string(src), "\n")

	type snippetStart struct {
		name string
		line int
	}
	var stack []snippetStart

	for i, l := range allLines {
		line := strings.TrimSpace(l)

		if idx := strings.Index(line, "{#snippet "); idx != -1 {
			rest := line[idx+len("{#snippet "):]
			nameEnd := strings.IndexAny(rest, "( \t}")
			if nameEnd != -1 {
				name := strings.TrimSpace(rest[:nameEnd])
				stack = append(stack, snippetStart{name: name, line: i + 1})
			}
		}

		if strings.Contains(line, "{/snippet}") && len(stack) > 0 {
			start := stack[len(stack)-1]
			stack = stack[:len(stack)-1]

			snippetLines := i + 1 - start.line + 1
			comps = append(comps, splitting.Component{
				Name:       start.name,
				LineCount:  snippetLines,
				IsExported: false,
				StartLine:  start.line,
			})
		}
	}
	return comps
}

func collectExportedNames(root *sitter.Node, lang *sitter.Language, src []byte) map[string]bool {
	exported := make(map[string]bool)
	q := `
	(export_specifier name: (identifier) @name)
	(export_statement value: (identifier) @name)
	(export_statement (identifier) @name)
	`
	_ = runQuery(q, root, lang, func(caps map[string]*sitter.Node) {
		if node, ok := caps["name"]; ok && node != nil {
			exported[nodeText(node, src)] = true
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

func countSvelteFileCodeLines(src []byte) int {
	allLines := strings.Split(string(src), "\n")
	count := 0
	inCommentBlock := false
	for _, l := range allLines {
		line := strings.TrimSpace(l)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "<!--") && strings.HasSuffix(line, "-->") {
			continue
		}
		if strings.HasPrefix(line, "<!--") {
			inCommentBlock = true
			continue
		}
		if inCommentBlock {
			if strings.HasSuffix(line, "-->") {
				inCommentBlock = false
			}
			continue
		}
		if strings.HasPrefix(line, "//") || strings.HasPrefix(line, "/*") || strings.HasPrefix(line, "*") {
			continue
		}
		count++
	}
	return count
}
