package nesthint

import (
	"sort"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/aykutssert/scout/internal/core"
)

type mutableVar struct {
	Name string
	Line int
}

func collectMutableVar(fi *fileInfo, n *sitter.Node, src []byte) {
	var kind string
	if n.Type() == "lexical_declaration" {
		kind = text(n.ChildByFieldName("kind"), src)
	} else {
		kind = "var"
	}
	if kind != "let" && kind != "var" {
		return
	}
	if !isModuleLevelLet(n) {
		return
	}
	for i := 0; i < int(n.NamedChildCount()); i++ {
		decl := n.NamedChild(i)
		if decl.Type() != "variable_declarator" {
			continue
		}
		nameNode := decl.ChildByFieldName("name")
		if nameNode == nil {
			continue
		}
		fi.MutableVars = append(fi.MutableVars, mutableVar{
			Name: text(nameNode, src),
			Line: line(decl),
		})
	}
}

func isModuleLevelLet(n *sitter.Node) bool {
	for p := n.Parent(); p != nil; p = p.Parent() {
		switch p.Type() {
		case "program":
			return true
		case "function_declaration", "arrow_function", "function_expression", "method_definition", "class_declaration", "for_statement", "for_in_statement", "for_of_statement":
			return false
		}
	}
	return false
}

func (p *project) detectServerMutableState() []core.Finding {
	var out []core.Finding
	fileKeys := make([]string, 0, len(p.Files))
	for k := range p.Files {
		fileKeys = append(fileKeys, k)
	}
	sort.Strings(fileKeys)
	for _, rel := range fileKeys {
		fi := p.Files[rel]
		sort.Slice(fi.MutableVars, func(i, j int) bool {
			return fi.MutableVars[i].Line < fi.MutableVars[j].Line
		})
		for _, mv := range fi.MutableVars {
			out = append(out, core.Finding{
				Analyzer:   "nest-hint",
				RuleID:     "nestjs.server-mutable-module-state",
				Severity:   core.SeverityWarning,
				Level:      core.SeverityWarning.String(),
				Category:   "bug",
				Confidence: core.ConfidenceHint,
				File:       rel,
				Line:       mv.Line,
				Message:    "Module-level " + mv.Name + " is declared with let/var. This mutable state persists across requests and can leak data between users.",
				Fix:        "Use const for module-level bindings. If per-request state is needed, use a request-scoped provider or a closure that captures the state per invocation.",
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out
}
