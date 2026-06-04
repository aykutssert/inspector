package sveltehint

import (
	"strings"

	"github.com/aykutssert/inspector/internal/core"
)

// detectEachIndexAsKey flags `{#each items as item, i (i)}` — keying a keyed
// each by the loop index. eslint-plugin-svelte's valid-each-key accepts the
// index (it is an each-block variable), so this gap is ours. Keying by index
// defeats the point of a keyed each: when the list reorders, Svelte remounts the
// nodes and loses component state, which is usually a bug.
//
// This scans the Svelte template directly (not a <script> block), because the
// each construct lives in the markup, which has no tree-sitter grammar here.
func detectEachIndexAsKey(src []byte, rel string) []core.Finding {
	var out []core.Finding
	text := string(src)
	for i := 0; i+len("{#each") <= len(text); i++ {
		if !strings.HasPrefix(text[i:], "{#each") {
			continue
		}
		header, end := braceHeader(text, i)
		if end == -1 {
			continue
		}
		if idx, key, ok := eachIndexAndKey(header); ok && idx != "" && key == idx {
			out = append(out, core.Finding{
				Analyzer:   "svelte-hint",
				RuleID:     "svelte.each-index-as-key",
				Severity:   core.SeverityWarning,
				Level:      core.SeverityWarning.String(),
				Category:   "quality",
				Confidence: core.ConfidenceHint,
				File:       rel,
				Line:       lineForOffset(src, i),
				Message:    "Svelte {#each} keys by the loop index. When the list reorders, Svelte remounts the nodes and loses their component state, defeating the keyed each.",
				Fix:        "Key by a stable unique id from the item data, e.g. {#each items as item (item.id)}.",
			})
		}
		i = end
	}
	return out
}

// braceHeader returns the content inside the brace block that opens at start
// (which must point at '{'), and the offset of its closing '}'. Quotes and
// nested braces are respected so a key expression containing them is handled.
func braceHeader(text string, start int) (string, int) {
	depth := 0
	var quote byte
	for j := start; j < len(text); j++ {
		c := text[j]
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '"', '\'', '`':
			quote = c
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return text[start+1 : j], j
			}
		}
	}
	return "", -1
}

// eachIndexAndKey parses an each header like "#each EXPR as ITEM, IDX (KEY)"
// and returns the index variable, the key expression, and whether a keyed each
// with an index was found.
func eachIndexAndKey(header string) (index, key string, ok bool) {
	h := strings.TrimSpace(header)
	h = strings.TrimPrefix(h, "#each")
	asAt := topLevelIndex(h, " as ")
	if asAt == -1 {
		return "", "", false
	}
	rest := strings.TrimSpace(h[asAt+len(" as "):])

	open := lastTopLevelOpenParen(rest)
	if open == -1 || !strings.HasSuffix(rest, ")") {
		return "", "", false // no key expression
	}
	key = strings.TrimSpace(rest[open+1 : len(rest)-1])
	bindings := strings.TrimSpace(rest[:open])

	parts := splitTopLevelComma(bindings)
	if len(parts) < 2 {
		return "", "", false // no index variable
	}
	index = strings.TrimSpace(parts[len(parts)-1])
	return index, key, true
}

// topLevelIndex finds sub in s ignoring matches inside quotes or brackets.
func topLevelIndex(s, sub string) int {
	depth := 0
	var quote byte
	for i := 0; i+len(sub) <= len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '"', '\'', '`':
			quote = c
		case '(', '{', '[':
			depth++
		case ')', '}', ']':
			if depth > 0 {
				depth--
			}
		}
		if depth == 0 && quote == 0 && strings.HasPrefix(s[i:], sub) {
			return i
		}
	}
	return -1
}

// lastTopLevelOpenParen returns the index of the '(' that opens the final
// top-level parenthesized group, or -1 if there is none.
func lastTopLevelOpenParen(s string) int {
	depth := 0
	var quote byte
	open := -1
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '"', '\'', '`':
			quote = c
		case '(':
			if depth == 0 {
				open = i
			}
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case '{', '[':
			depth++
		case '}', ']':
			if depth > 0 {
				depth--
			}
		}
	}
	return open
}

// splitTopLevelComma splits on commas that are not inside quotes or brackets, so
// a destructured item pattern ({a, b}) stays a single binding.
func splitTopLevelComma(s string) []string {
	var parts []string
	depth := 0
	var quote byte
	start := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '"', '\'', '`':
			quote = c
		case '(', '{', '[':
			depth++
		case ')', '}', ']':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])
	return parts
}
