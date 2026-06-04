package reacthint

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/aykutssert/inspector/internal/core"
)

// NOTE: the framework-agnostic repeated-magic-literal detector moved to the
// jsquality analyzer so it runs on every JS/TS project, not only React ones
// (this pack is gated on a React/Next signal). reacthint keeps only the
// React-shaped JSX hints below.

func sortFindingsByLine(findings []core.Finding) {
	for i := 1; i < len(findings); i++ {
		for j := i; j > 0 && findings[j].Line < findings[j-1].Line; j-- {
			findings[j], findings[j-1] = findings[j-1], findings[j]
		}
	}
}

const jsxTextQuery = `(jsx_text) @t`

// detectEmDashInJSX flags an em dash (—) in rendered JSX text. It is a common
// tell of AI-generated copy and an inconsistency in most codebases. A hint, not
// a defect: the author may have wanted it.
func detectEmDashInJSX(root *sitter.Node, lang *sitter.Language, src []byte, file string, _ map[string]bool) []core.Finding {
	var out []core.Finding
	seen := map[int]bool{}
	_ = runQuery(jsxTextQuery, root, lang, func(_ string, node *sitter.Node) {
		if !strings.Contains(node.Content(src), "—") {
			return
		}
		line := int(node.StartPoint().Row) + 1
		if seen[line] {
			return
		}
		seen[line] = true
		out = append(out, hint(
			"em-dash-in-jsx-text", "quality", core.SeverityInfo, file, line,
			"Em dash (—) in rendered text; often an AI-generated copy tell and a typographic inconsistency.",
			"Use a regular hyphen or rewrite the sentence if it was not intentional.",
		))
	})
	return out
}
