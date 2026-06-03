package javascript

import (
	"path/filepath"
	"strings"

	"github.com/aykutssert/inspector/internal/core"
)

type Adapter struct {
	rulesDir string
}

func New(rulesDir string) *Adapter {
	return &Adapter{rulesDir: rulesDir}
}

var _ core.LanguageAdapter = (*Adapter)(nil)

func (a *Adapter) Language() string { return "javascript" }

var extensions = map[string]bool{
	".js": true, ".jsx": true, ".ts": true, ".tsx": true,
	".mjs": true, ".cjs": true, ".mts": true, ".cts": true,
}

func (a *Adapter) Matches(path string) bool {
	return extensions[strings.ToLower(filepath.Ext(path))]
}

func (a *Adapter) Rules() []string {
	if a.rulesDir == "" {
		return nil
	}
	return []string{filepath.Join(a.rulesDir, "javascript")}
}
