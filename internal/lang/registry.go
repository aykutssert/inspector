package lang

import "github.com/aykutssert/inspector/internal/core"

type Registry struct {
	adapters []core.LanguageAdapter
}

func NewRegistry(adapters ...core.LanguageAdapter) *Registry {
	return &Registry{adapters: adapters}
}

func (r *Registry) Detect(files []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, a := range r.adapters {
		for _, f := range files {
			if a.Matches(f) {
				if !seen[a.Language()] {
					seen[a.Language()] = true
					out = append(out, a.Language())
				}
				break
			}
		}
	}
	return out
}
