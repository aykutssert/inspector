package lang

import (
	"github.com/aykutssert/scout/internal/core"
)

type Registry struct {
	adapters []core.LanguageAdapter
}

func NewRegistry(adapters ...core.LanguageAdapter) *Registry {
	return &Registry{adapters: adapters}
}

// Detect returns the set of internal language names present in files.
// Uses enry-based detection (extension fast path + content-based fallback).
func (r *Registry) Detect(files []string, root string) []string {
	seen := map[string]bool{}
	groups := DetectLanguages(files, root)
	for lang := range groups {
		seen[lang] = true
	}
	// Also check adapter-based matching for languages enry may not detect
	// (e.g. custom DSLs or languages without enry mapping).
	for _, a := range r.adapters {
		for _, f := range files {
			if a.Matches(f) {
				if !seen[a.Language()] {
					seen[a.Language()] = true
				}
				break
			}
		}
	}
	out := make([]string, 0, len(seen))
	for lang := range seen {
		out = append(out, lang)
	}
	return out
}

// GroupByLanguage groups files by their detected internal language using
// enry-based detection. Returns a map of language → matching files.
func (r *Registry) GroupByLanguage(files []string, root string) map[string][]string {
	groups := DetectLanguages(files, root)
	// Extend with adapter-based grouping for languages enry may miss.
	for _, a := range r.adapters {
		lang := a.Language()
		if _, ok := groups[lang]; ok {
			continue
		}
		for _, f := range files {
			if a.Matches(f) {
				groups[lang] = append(groups[lang], f)
			}
		}
	}
	return groups
}
