package nesthint

import (
	"sort"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/aykutssert/scout/internal/core"
)

func (p *project) detectRequestScopedOveruse() []core.Finding {
	var out []core.Finding
	keys := make([]string, 0, len(p.Classes))
	for k := range p.Classes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		c := p.Classes[k]
		if c.InjectableScope != "REQUEST" {
			continue
		}
		out = append(out, core.Finding{
			Analyzer:   "nest-hint",
			RuleID:     "nestjs.request-scoped-overuse",
			Severity:   core.SeverityWarning,
			Level:      core.SeverityWarning.String(),
			Category:   "performance",
			Confidence: core.ConfidenceHint,
			File:       c.Ref.File,
			Line:       c.Line,
			Message:    c.Ref.Name + " is request-scoped (@Injectable({ scope: Scope.REQUEST })). A new instance is created per request, increasing memory pressure and GC.",
			Fix:        "Use Scope.DEFAULT (singleton) unless the provider genuinely needs per-request state. Extract request-specific logic into a smaller provider and keep the main logic singleton.",
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out
}

func extractInjectableScope(classNode *sitter.Node, src []byte) string {
	dec := findClassDecorator(classNode, "Injectable", src)
	if dec == nil {
		return ""
	}
	call := firstChildOfType(dec, "call_expression")
	if call == nil {
		return ""
	}
	args := firstChildOfType(call, "arguments")
	if args == nil {
		return ""
	}
	obj := firstChildOfType(args, "object")
	if obj == nil {
		return ""
	}
	pair := objectPair(obj, "scope", src)
	if pair == nil {
		return ""
	}
	val := pair.ChildByFieldName("value")
	if val == nil {
		return ""
	}
	if val.Type() == "member_expression" {
		if prop := val.ChildByFieldName("property"); prop != nil {
			return text(prop, src)
		}
	}
	return ""
}
