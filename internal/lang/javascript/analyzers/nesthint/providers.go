package nesthint

import (
	"sort"

	"github.com/aykutssert/scout/internal/core"
)

func (p *project) findMissingProviders() []core.Finding {
	owner := p.classOwners()
	globalProviders := p.globalExportedProviders()
	var out []core.Finding
	classKeys := make([]string, 0, len(p.Classes))
	for k := range p.Classes {
		classKeys = append(classKeys, k)
	}
	sort.Strings(classKeys)
	for _, classKey := range classKeys {
		c := p.Classes[classKey]
		if len(c.Constructor) == 0 {
			continue
		}
		mod := owner[classKey]
		if mod == nil {
			continue
		}
		for _, dep := range c.Constructor {
			if p.providerAvailable(mod, dep.Ref.key(), globalProviders, map[string]bool{}) {
				continue
			}
			if mod.UnknownProvider || mod.UnknownImports {
				continue
			}
			out = append(out, core.Finding{
				Analyzer:   "nest-hint",
				RuleID:     "nestjs.provider-not-registered",
				Severity:   core.SeverityWarning,
				Level:      core.SeverityWarning.String(),
				Category:   "bug",
				Confidence: core.ConfidenceRule,
				File:       c.Ref.File,
				Line:       dep.Line,
				Message:    dep.Name + " is injected into " + c.Ref.Name + " but is not provided by " + mod.Ref.Name + " or any imported module that exports it.",
				Fix:        "Add " + dep.Name + " to the module's providers, or import a module that exports " + dep.Name + ".",
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

func (p *project) classOwners() map[string]*moduleInfo {
	out := map[string]*moduleInfo{}
	keys := make([]string, 0, len(p.Modules))
	for k := range p.Modules {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		m := p.Modules[k]
		for c := range m.Controllers {
			out[c] = m
		}
		for c := range m.Providers {
			out[c] = m
		}
	}
	return out
}

func (p *project) globalExportedProviders() map[string]bool {
	out := map[string]bool{}
	for _, m := range p.Modules {
		if !m.IsGlobal {
			continue
		}
		for provider := range m.Providers {
			if m.Exports[provider] {
				out[provider] = true
			}
		}
	}
	return out
}

func (p *project) providerAvailable(m *moduleInfo, dep string, global map[string]bool, seen map[string]bool) bool {
	if dep == "" {
		return true
	}
	if m.Providers[dep] || global[dep] {
		return true
	}
	if seen[m.Ref.key()] {
		return false
	}
	seen[m.Ref.key()] = true
	for imported := range m.Imports {
		im := p.Modules[imported]
		if im == nil {
			continue
		}
		if im.Exports[dep] && im.Providers[dep] {
			return true
		}
		if im.UnknownExports || im.UnknownProvider {
			return true
		}
		for exported := range im.Exports {
			exportedModule := p.Modules[exported]
			if exportedModule != nil && p.exportedProviderAvailable(exportedModule, dep, seen) {
				return true
			}
		}
	}
	return false
}

func (p *project) exportedProviderAvailable(m *moduleInfo, dep string, seen map[string]bool) bool {
	if seen[m.Ref.key()] {
		return false
	}
	seen[m.Ref.key()] = true
	if m.Exports[dep] && m.Providers[dep] {
		return true
	}
	if m.UnknownExports || m.UnknownProvider {
		return true
	}
	for exported := range m.Exports {
		if child := p.Modules[exported]; child != nil && p.exportedProviderAvailable(child, dep, seen) {
			return true
		}
	}
	return false
}
