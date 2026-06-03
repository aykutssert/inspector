package packs

import "github.com/aykutssert/inspector/internal/core"

func containsLanguage(ctx core.ProjectContext, lang string) bool {
	for _, got := range ctx.Languages {
		if got == lang {
			return true
		}
	}
	return false
}
