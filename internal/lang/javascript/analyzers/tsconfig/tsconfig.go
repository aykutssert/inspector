package tsconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aykutssert/scout/internal/core"
)

type Analyzer struct{}

func New() *Analyzer { return &Analyzer{} }

var _ core.Analyzer = (*Analyzer)(nil)

func (a *Analyzer) Name() string { return "tsconfig" }

func (a *Analyzer) Available() bool { return true }

type tsConfig struct {
	Extends         any `json:"extends"`
	CompilerOptions struct {
		Strict *bool `json:"strict"`
	} `json:"compilerOptions"`
}

func (a *Analyzer) Scan(ctx core.ProjectContext) ([]core.Finding, error) {
	if !hasTSFiles(ctx.Files) {
		return nil, nil
	}

	dirs := tsProjectDirs(ctx)
	if len(dirs) == 0 {
		return nil, nil
	}

	var findings []core.Finding
	for _, dir := range dirs {
		fs, err := a.scanProject(ctx.Root, dir)
		if err != nil {
			return nil, err
		}
		findings = append(findings, fs...)
	}
	return findings, nil
}

func (a *Analyzer) scanProject(root, dir string) ([]core.Finding, error) {
	configPath := filepath.Join(dir, "tsconfig.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	cleaned := stripJSONComments(data)

	var cfg tsConfig
	if err := json.Unmarshal(cleaned, &cfg); err != nil {
		// If it is malformed JSON, tsc will catch it anyway, or we can ignore it to be low noise.
		return nil, nil
	}

	// A config that extends another inherits compilerOptions we cannot see
	// without resolving the base, so flagging "strict missing" here would be a
	// false positive on the common monorepo pattern (per-package tsconfig that
	// extends a strict root). Skip extending configs.
	if cfg.Extends != nil {
		return nil, nil
	}

	relPath, err := filepath.Rel(root, configPath)
	if err != nil {
		relPath = configPath
	}

	// Only flag strict. noUncheckedIndexedAccess / exactOptionalPropertyTypes are
	// opt-in flags that are off by default, so a "disabled" check fires on almost
	// every project — a low-actionable nag about a preference, not a defect.
	if cfg.CompilerOptions.Strict == nil || !*cfg.CompilerOptions.Strict {
		return []core.Finding{{
			Analyzer:   "tsconfig",
			RuleID:     "tsconfig-strict-disabled",
			Severity:   core.SeverityWarning,
			Level:      core.SeverityWarning.String(),
			Category:   "quality",
			Confidence: core.ConfidenceRule,
			File:       relPath,
			Line:       1,
			Message:    "tsconfig.json has 'strict' compiler option disabled or missing. Enabling strict type checking enforces safer type contracts and catches bugs at compile time.",
			Fix:        "Set 'compilerOptions.strict' to true in tsconfig.json.",
		}}, nil
	}

	return nil, nil
}

func stripJSONComments(data []byte) []byte {
	var out []byte
	inString := false
	inLineComment := false
	inBlockComment := false

	for i := 0; i < len(data); i++ {
		ch := data[i]

		if inLineComment {
			if ch == '\n' {
				inLineComment = false
				out = append(out, ch)
			}
			continue
		}
		if inBlockComment {
			if ch == '*' && i+1 < len(data) && data[i+1] == '/' {
				inBlockComment = false
				i++
			}
			continue
		}
		if inString {
			if ch == '"' {
				// Check for escaped quote
				escaped := false
				for j := len(out) - 1; j >= 0; j-- {
					if out[j] == '\\' {
						escaped = !escaped
					} else {
						break
					}
				}
				if !escaped {
					inString = false
				}
			}
			out = append(out, ch)
			continue
		}

		if ch == '"' {
			inString = true
			out = append(out, ch)
			continue
		}

		if ch == '/' && i+1 < len(data) {
			next := data[i+1]
			if next == '/' {
				inLineComment = true
				i++
				continue
			} else if next == '*' {
				inBlockComment = true
				i++
				continue
			}
		}

		out = append(out, ch)
	}
	return out
}

func hasTSFiles(files []string) bool {
	for _, f := range files {
		switch strings.ToLower(filepath.Ext(f)) {
		case ".ts", ".tsx", ".mts", ".cts":
			return true
		}
	}
	return false
}

func tsProjectDirs(ctx core.ProjectContext) []string {
	seen := map[string]bool{}
	for _, f := range ctx.Files {
		switch strings.ToLower(filepath.Ext(f)) {
		case ".ts", ".tsx", ".mts", ".cts":
		default:
			continue
		}
		dir := filepath.Dir(f)
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(ctx.Root, dir)
		}
		if d := nearestTSConfigDir(dir, ctx.Root); d != "" {
			seen[d] = true
		}
	}
	out := make([]string, 0, len(seen))
	for d := range seen {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

func nearestTSConfigDir(dir, root string) string {
	for {
		if _, err := os.Stat(filepath.Join(dir, "tsconfig.json")); err == nil {
			return dir
		}
		if dir == root || !strings.HasPrefix(dir, root) {
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
