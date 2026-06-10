package lang

import (
	"os"
	"path/filepath"
	"strings"

	enry "github.com/go-enry/go-enry/v2"
)

// internalLanguage maps from enry-reported language to our internal language name.
var internalLanguage = map[string]string{
	"JavaScript": "javascript",
	"TypeScript": "javascript",
	"JSX":        "javascript",
	"TSX":        "javascript",
	"Svelte":     "svelte",
}

// DetectLanguage returns the internal language name for a file, or "" if
// unsupported. Uses file extension as fast path then falls back to enry
// content-based detection for ambiguous or unknown extensions.
func DetectLanguage(path, root string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".js", ".jsx", ".mjs", ".cjs":
		return "javascript"
	case ".ts", ".tsx", ".mts", ".cts":
		return "javascript"
	case ".svelte":
		return "svelte"
	}
	rel := filepath.Join(root, path)
	content, err := os.ReadFile(rel)
	if err != nil {
		return ""
	}
	lang := enry.GetLanguage(rel, content)
	if mapped, ok := internalLanguage[lang]; ok {
		return mapped
	}
	return ""
}

// DetectLanguages groups files by detected internal language. Unsupported
// files (no matching language) are omitted from the result.
func DetectLanguages(files []string, root string) map[string][]string {
	groups := map[string][]string{}
	for _, f := range files {
		lang := DetectLanguage(f, root)
		if lang == "" {
			continue
		}
		groups[lang] = append(groups[lang], f)
	}
	return groups
}
