// Package sveltehint contains deterministic Svelte-specific hints that the
// external eslint-plugin-svelte ruleset does not cover.
package sveltehint

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
	"github.com/smacker/go-tree-sitter/typescript/typescript"

	"github.com/aykutssert/scout/internal/core"
)

const (
	maxFileBytes = 1 << 20 // 1 MiB; skip larger files instead of parsing
	parseTimeout = 5 * time.Second
)

var globalDOMQueryMethods = map[string]bool{
	"getElementById":         true,
	"querySelector":          true,
	"querySelectorAll":       true,
	"getElementsByClassName": true,
	"getElementsByTagName":   true,
}

const documentCallQuery = `
(call_expression
  function: (member_expression
    object: (identifier) @object
    property: (property_identifier) @property)) @call
`

type Analyzer struct{}

func New() *Analyzer { return &Analyzer{} }

var _ core.Analyzer = (*Analyzer)(nil)

func (a *Analyzer) Name() string { return "svelte-hint" }

func (a *Analyzer) Available() bool { return true }

func (a *Analyzer) Scan(ctx core.ProjectContext) ([]core.Finding, error) {
	var targets []string
	for _, f := range ctx.Files {
		if strings.EqualFold(filepath.Ext(f), ".svelte") {
			targets = append(targets, f)
		}
	}
	if len(targets) == 0 {
		return nil, nil
	}
	sort.Strings(targets)

	var findings []core.Finding
	for _, rel := range targets {
		fs, err := scanFile(filepath.Join(ctx.Root, rel), rel)
		if err != nil {
			continue
		}
		findings = append(findings, fs...)
	}
	return findings, nil
}

func scanFile(abs, rel string) ([]core.Finding, error) {
	if info, err := os.Stat(abs); err == nil && info.Size() > maxFileBytes {
		return nil, nil
	}
	src, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	if len(src) > maxFileBytes {
		return nil, nil
	}
	var findings []core.Finding
	for _, block := range scriptBlocks(src) {
		fs, err := scanScriptBlock(block)
		if err != nil {
			continue
		}
		for _, f := range fs {
			f.File = rel
			findings = append(findings, f)
		}
	}
	findings = append(findings, detectEachIndexAsKey(src, rel)...)
	findings = append(findings, detectComponentSplitting(src, rel)...)
	return findings, nil
}

type scriptBlock struct {
	src       []byte
	lineStart int
	ts        bool
}

func scriptBlocks(src []byte) []scriptBlock {
	lower := strings.ToLower(string(src))
	var blocks []scriptBlock
	pos := 0
	for {
		open := strings.Index(lower[pos:], "<script")
		if open == -1 {
			break
		}
		open += pos
		tagEnd := findTagEnd(src, open)
		if tagEnd == -1 {
			break
		}
		tag := string(src[open : tagEnd+1])
		closeRel := strings.Index(lower[tagEnd+1:], "</script>")
		if closeRel == -1 {
			break
		}
		contentStart := tagEnd + 1
		contentEnd := tagEnd + 1 + closeRel
		if !hasScriptSrcAttr(tag) {
			blocks = append(blocks, scriptBlock{
				src:       src[contentStart:contentEnd],
				lineStart: lineForOffset(src, contentStart),
				ts:        isTypeScriptScript(tag),
			})
		}
		pos = contentEnd + len("</script>")
	}
	return blocks
}

func findTagEnd(src []byte, start int) int {
	var quote byte
	for i := start; i < len(src); i++ {
		c := src[i]
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			quote = c
			continue
		}
		if c == '>' {
			return i
		}
	}
	return -1
}

func lineForOffset(src []byte, offset int) int {
	line := 1
	for i := 0; i < offset && i < len(src); i++ {
		if src[i] == '\n' {
			line++
		}
	}
	return line
}

func hasScriptSrcAttr(tag string) bool {
	return hasAttr(tag, "src")
}

func isTypeScriptScript(tag string) bool {
	return attrValueEquals(tag, "lang", "ts") || attrValueEquals(tag, "type", "text/typescript")
}

func hasAttr(tag, name string) bool {
	_, ok := attrValue(tag, name)
	return ok
}

func attrValueEquals(tag, name, want string) bool {
	got, ok := attrValue(tag, name)
	return ok && strings.EqualFold(got, want)
}

func attrValue(tag, name string) (string, bool) {
	lower := strings.ToLower(tag)
	name = strings.ToLower(name)
	for i := 0; i < len(lower); i++ {
		if !attrBoundaryBefore(lower, i) || !strings.HasPrefix(lower[i:], name) {
			continue
		}
		j := i + len(name)
		if j < len(lower) && isAttrNameChar(lower[j]) {
			continue
		}
		for j < len(lower) && isSpace(lower[j]) {
			j++
		}
		if j >= len(lower) || lower[j] != '=' {
			return "", true
		}
		j++
		for j < len(lower) && isSpace(lower[j]) {
			j++
		}
		if j >= len(tag) {
			return "", true
		}
		if tag[j] == '"' || tag[j] == '\'' {
			quote := tag[j]
			j++
			start := j
			for j < len(tag) && tag[j] != quote {
				j++
			}
			return tag[start:j], true
		}
		start := j
		for j < len(tag) && !isSpace(tag[j]) && tag[j] != '>' {
			j++
		}
		return tag[start:j], true
	}
	return "", false
}

func attrBoundaryBefore(s string, i int) bool {
	if i == 0 {
		return true
	}
	return isSpace(s[i-1]) || s[i-1] == '<'
}

func isAttrNameChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == ':'
}

func isSpace(c byte) bool {
	switch c {
	case ' ', '\n', '\r', '\t', '\f':
		return true
	default:
		return false
	}
}

func scanScriptBlock(block scriptBlock) ([]core.Finding, error) {
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
		return nil, err
	}
	defer tree.Close()
	root := tree.RootNode()
	lang := javascript.GetLanguage()
	if block.ts {
		lang = typescript.GetLanguage()
	}

	var findings []core.Finding
	err = runQuery(documentCallQuery, root, lang, func(caps map[string]*sitter.Node) {
		if nodeText(caps["object"], block.src) != "document" {
			return
		}
		method := nodeText(caps["property"], block.src)
		if !globalDOMQueryMethods[method] {
			return
		}
		call := caps["call"]
		findings = append(findings, core.Finding{
			Analyzer:   "svelte-hint",
			RuleID:     "svelte.global-dom-query",
			Severity:   core.SeverityWarning,
			Level:      core.SeverityWarning.String(),
			Category:   "quality",
			Confidence: core.ConfidenceHint,
			Line:       block.lineStart + int(call.StartPoint().Row),
			Message:    "Svelte component queries the global document with document." + method + "(...). This often bypasses Svelte's binding/reactivity model and makes the component harder to reason about.",
			Fix:        "Prefer bind:this for component-owned elements, or model the UI state in Svelte. If this is third-party integration, isolate it inside onMount.",
		})
	})
	return findings, err
}

func runQuery(q string, root *sitter.Node, lang *sitter.Language, fn func(map[string]*sitter.Node)) error {
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

func nodeText(n *sitter.Node, src []byte) string {
	if n == nil {
		return ""
	}
	return string(src[n.StartByte():n.EndByte()])
}
