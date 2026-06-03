package svelte

import (
	"path/filepath"
	"strings"

	"github.com/aykutssert/inspector/internal/core"
)

type Adapter struct{}

func New() *Adapter { return &Adapter{} }

var _ core.LanguageAdapter = (*Adapter)(nil)

func (a *Adapter) Language() string { return "svelte" }

func (a *Adapter) Matches(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".svelte")
}

// Rules returns no semgrep rule dir: semgrep has no Svelte language, so a YAML
// pack would never match. Svelte rules are configured in the eslint flat config
// the svelte-lint analyzer wraps (_toolchains/svelte/eslint.config.mjs).
func (a *Adapter) Rules() []string { return nil }
