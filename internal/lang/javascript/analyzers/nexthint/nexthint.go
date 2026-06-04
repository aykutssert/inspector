package nexthint

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aykutssert/inspector/internal/core"
	jscontext "github.com/aykutssert/inspector/internal/lang/javascript/context"
)

type Analyzer struct{}

func New() *Analyzer { return &Analyzer{} }

var _ core.Analyzer = (*Analyzer)(nil)

func (a *Analyzer) Name() string { return "next-hint" }

func (a *Analyzer) Available() bool { return true }

func (a *Analyzer) Scan(ctx core.ProjectContext) ([]core.Finding, error) {
	files := jsFiles(ctx.Files)
	if len(files) == 0 {
		return nil, nil
	}
	graph := jscontext.Build(ctx.Root, files)
	targets := serverOnlyTargets(graph)
	if len(targets) == 0 {
		return nil, nil
	}

	var findings []core.Finding
	for _, file := range sortedGraphFiles(graph) {
		if !hasUseClientDirective(filepath.Join(ctx.Root, file)) {
			continue
		}
		fp := graph.Files[file]
		seenImports := map[string]bool{}
		for _, im := range fp.Imports {
			if !strings.HasPrefix(im.Source, ".") {
				continue
			}
			target := graph.ResolveImport(file, im.Source)
			if target == "" {
				continue
			}
			serverTarget, chain, ok := reachableServerTarget(graph, target, targets)
			if !ok {
				continue
			}
			key := fmt.Sprintf("%d:%s:%s", im.Line, im.Source, serverTarget.file)
			if seenImports[key] {
				continue
			}
			seenImports[key] = true
			findings = append(findings, boundaryFinding(a.Name(), file, im.Line, im.Source, serverTarget, append([]string{file}, chain...)))
		}
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		return findings[i].Message < findings[j].Message
	})
	return findings, nil
}

func jsFiles(files []string) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		switch strings.ToLower(filepath.Ext(f)) {
		case ".js", ".jsx", ".ts", ".tsx", ".mjs", ".cjs", ".mts", ".cts":
			out = append(out, f)
		}
	}
	sort.Strings(out)
	return out
}

type serverTarget struct {
	file       string
	reason     string
	confidence string
}

func serverOnlyTargets(graph *jscontext.Graph) map[string]serverTarget {
	targets := map[string]serverTarget{}
	for _, file := range sortedGraphFiles(graph) {
		fp := graph.Files[file]
		if reason, ok := explicitServerOnlyFile(file); ok {
			targets[file] = serverTarget{file: file, reason: reason, confidence: core.ConfidenceRule}
			continue
		}
		if source, ok := importsServerOnlyPackage(fp); ok {
			targets[file] = serverTarget{file: file, reason: "imports server-only package " + source, confidence: core.ConfidenceRule}
			continue
		}
	}
	return targets
}

func explicitServerOnlyFile(file string) (string, bool) {
	slash := filepath.ToSlash(strings.ToLower(file))
	base := filepath.Base(slash)
	if strings.Contains(base, ".server.") {
		return "uses .server.* filename", true
	}
	for _, segment := range strings.Split(slash, "/") {
		switch segment {
		case "server", "server-only":
			return "is under a server-only directory", true
		}
	}
	return "", false
}

func importsServerOnlyPackage(fp *jscontext.FileParse) (string, bool) {
	for _, im := range fp.Imports {
		source := normalizePackageSource(im.Source)
		if isServerOnlyPackage(source) {
			return im.Source, true
		}
	}
	return "", false
}

func normalizePackageSource(source string) string {
	source = strings.TrimPrefix(source, "node:")
	if strings.HasPrefix(source, "@prisma/client") {
		return "@prisma/client"
	}
	parts := strings.Split(source, "/")
	if strings.HasPrefix(source, "@") && len(parts) >= 2 {
		return parts[0] + "/" + parts[1]
	}
	if len(parts) > 0 {
		return parts[0]
	}
	return source
}

func isServerOnlyPackage(source string) bool {
	switch source {
	case "server-only",
		"fs", "child_process", "dns", "net", "tls",
		"@prisma/client", "pg", "postgres", "mysql", "mysql2", "mongoose",
		"typeorm", "sequelize", "drizzle-orm", "knex", "better-sqlite3", "sqlite3":
		return true
	}
	return false
}

func reachableServerTarget(graph *jscontext.Graph, start string, targets map[string]serverTarget) (serverTarget, []string, bool) {
	type pathNode struct {
		file string
		path []string
	}
	queue := []pathNode{{file: start, path: []string{start}}}
	seen := map[string]bool{start: true}
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		if target, ok := targets[node.file]; ok {
			return target, node.path, true
		}
		next := append([]string(nil), graph.Imports(node.file)...)
		sort.Strings(next)
		for _, child := range next {
			if seen[child] {
				continue
			}
			seen[child] = true
			path := append(append([]string(nil), node.path...), child)
			queue = append(queue, pathNode{file: child, path: path})
		}
	}
	return serverTarget{}, nil, false
}

func boundaryFinding(analyzer, file string, line int, source string, target serverTarget, chain []string) core.Finding {
	return core.Finding{
		Analyzer:   analyzer,
		RuleID:     "next.local-server-boundary",
		Severity:   core.SeverityWarning,
		Level:      core.SeverityWarning.String(),
		Category:   "bug",
		Confidence: target.confidence,
		File:       file,
		Line:       line,
		Message:    fmt.Sprintf("Client component imports local server-only code through %q; reaches %s (%s). Import chain: %s.", source, target.file, target.reason, strings.Join(chain, " -> ")),
		Fix:        "Keep server-only code in a Server Component, Server Action, route handler, or API layer; pass only serializable data into the client component.",
	}
}

func sortedGraphFiles(graph *jscontext.Graph) []string {
	files := make([]string, 0, len(graph.Files))
	for file := range graph.Files {
		files = append(files, file)
	}
	sort.Strings(files)
	return files
}

func hasUseClientDirective(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	s := directiveScanner{src: data}
	for {
		s.skipWhitespaceAndComments()
		value, ok := s.readStringLiteral()
		if !ok {
			return false
		}
		s.skipHorizontalWhitespace()
		if !s.consumeDirectiveTerminator() {
			return false
		}
		if value == "use client" {
			return true
		}
	}
}

type directiveScanner struct {
	src []byte
	pos int
}

func (s *directiveScanner) skipWhitespaceAndComments() {
	for {
		s.skipHorizontalWhitespace()
		for s.pos < len(s.src) && (s.src[s.pos] == '\n' || s.src[s.pos] == '\r') {
			s.pos++
		}
		if s.skipComment() {
			continue
		}
		return
	}
}

func (s *directiveScanner) skipHorizontalWhitespace() {
	for s.pos < len(s.src) {
		switch s.src[s.pos] {
		case ' ', '\t', '\v', '\f', 0xef, 0xbb, 0xbf:
			s.pos++
		default:
			return
		}
	}
}

func (s *directiveScanner) skipComment() bool {
	if s.pos+1 >= len(s.src) || s.src[s.pos] != '/' {
		return false
	}
	switch s.src[s.pos+1] {
	case '/':
		s.pos += 2
		for s.pos < len(s.src) && s.src[s.pos] != '\n' && s.src[s.pos] != '\r' {
			s.pos++
		}
		return true
	case '*':
		s.pos += 2
		for s.pos+1 < len(s.src) {
			if s.src[s.pos] == '*' && s.src[s.pos+1] == '/' {
				s.pos += 2
				return true
			}
			s.pos++
		}
		s.pos = len(s.src)
		return true
	default:
		return false
	}
}

func (s *directiveScanner) readStringLiteral() (string, bool) {
	if s.pos >= len(s.src) || (s.src[s.pos] != '"' && s.src[s.pos] != '\'') {
		return "", false
	}
	quote := s.src[s.pos]
	s.pos++
	start := s.pos
	for s.pos < len(s.src) {
		ch := s.src[s.pos]
		if ch == '\\' {
			return "", false
		}
		if ch == quote {
			value := string(s.src[start:s.pos])
			s.pos++
			return value, true
		}
		if ch == '\n' || ch == '\r' {
			return "", false
		}
		s.pos++
	}
	return "", false
}

func (s *directiveScanner) consumeDirectiveTerminator() bool {
	if s.pos >= len(s.src) {
		return true
	}
	if s.src[s.pos] == ';' {
		s.pos++
		return true
	}
	return s.src[s.pos] == '\n' || s.src[s.pos] == '\r'
}
