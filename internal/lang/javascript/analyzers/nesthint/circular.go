package nesthint

import (
	"sort"

	"github.com/aykutssert/scout/internal/core"
)

func (p *project) detectCircularModuleDep() []core.Finding {
	var out []core.Finding
	type color int
	const (
		white color = iota
		gray
		black
	)
	colors := make(map[string]color, len(p.Modules))
	keys := make([]string, 0, len(p.Modules))
	for k := range p.Modules {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	stack := make([]string, 0, len(p.Modules))
	for _, k := range keys {
		if colors[k] != white {
			continue
		}
		stack = append(stack, k)
		for len(stack) > 0 {
			top := stack[len(stack)-1]
			switch colors[top] {
			case white:
				colors[top] = gray
				m := p.Modules[top]
				if m == nil || m.UnknownImports {
					colors[top] = black
					stack = stack[:len(stack)-1]
					continue
				}
				importKeys := make([]string, 0, len(m.Imports))
				for imp := range m.Imports {
					importKeys = append(importKeys, imp)
				}
				sort.Strings(importKeys)
				hasUnvisited := false
				for _, imp := range importKeys {
					if colors[imp] == gray {
						// Skip if this import uses forwardRef (NestJS tolerates it)
						if m.ForwardRefs[imp] {
							continue
						}
						out = append(out, core.Finding{
							Analyzer:   "nest-hint",
							RuleID:     "nestjs.circular-module-dep",
							Severity:   core.SeverityWarning,
							Level:      core.SeverityWarning.String(),
							Category:   "design",
							Confidence: core.ConfidenceRule,
							File:       m.File,
							Line:       m.Line,
							Message:    m.Ref.Name + " imports " + shortModuleName(imp) + " creating a circular dependency cycle.",
							Fix:        "Use forwardRef(() => " + shortModuleName(imp) + ") in one of the modules, or extract shared logic into a shared module imported by both.",
						})
						continue
					}
					if colors[imp] == white && p.Modules[imp] != nil {
						stack = append(stack, imp)
						hasUnvisited = true
					}
				}
				if !hasUnvisited {
					colors[top] = black
					stack = stack[:len(stack)-1]
				}
			case gray:
				colors[top] = black
				stack = stack[:len(stack)-1]
			case black:
				stack = stack[:len(stack)-1]
			}
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

func shortModuleName(key string) string {
	for i := len(key) - 1; i >= 0; i-- {
		if key[i] == '#' {
			return key[i+1:]
		}
	}
	return key
}
