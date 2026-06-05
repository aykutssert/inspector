package archhint

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aykutssert/scout/internal/architecture/boundary"
	"github.com/aykutssert/scout/internal/core"
	jscontext "github.com/aykutssert/scout/internal/lang/javascript/context"
)

type Analyzer struct{}

func New() *Analyzer { return &Analyzer{} }

var _ core.Analyzer = (*Analyzer)(nil)

func (a *Analyzer) Name() string { return "arch-hint" }

func (a *Analyzer) Available() bool { return true }

func (a *Analyzer) Scan(ctx core.ProjectContext) ([]core.Finding, error) {
	files := jsFiles(ctx.Files)
	if len(files) == 0 {
		return nil, nil
	}
	graph := jscontext.Build(ctx.Root, files)
	classifier := newClassifier(ctx.Root, graph)
	local := boundary.Analyze(boundaryGraph(graph), classifier.classify, layeredRules())

	var findings []core.Finding
	for _, v := range local {
		findings = append(findings, findingFromViolation(a.Name(), v))
	}
	findings = append(findings, directPackageBoundaryFindings(a.Name(), graph, classifier)...)
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		return findings[i].Message < findings[j].Message
	})
	return dedupeFindings(findings), nil
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

func boundaryGraph(graph *jscontext.Graph) boundary.Graph {
	files := sortedGraphFiles(graph)
	edges := map[string][]boundary.Edge{}
	for _, file := range files {
		fp := graph.Files[file]
		if fp == nil {
			continue
		}
		for _, im := range fp.Imports {
			target := graph.ResolveImport(file, im.Source)
			if target == "" {
				continue
			}
			edges[file] = append(edges[file], boundary.Edge{From: file, To: target, Source: im.Source, Line: im.Line})
		}
	}
	return boundary.Graph{Files: files, Edges: edges}
}

func layeredRules() []boundary.Rule {
	return []boundary.Rule{
		{
			ID:          "architecture.layered-boundary",
			SourceKind:  "controller",
			TargetKinds: []string{"repository", "orm-client", "data-model"},
			PassThrough: isTransparentBarrel,
		},
		{
			ID:          "architecture.layered-boundary",
			SourceKind:  "handler",
			TargetKinds: []string{"repository", "orm-client", "data-model"},
			PassThrough: isTransparentBarrel,
		},
	}
}

func isTransparentBarrel(file string) bool {
	base := strings.ToLower(filepath.Base(file))
	switch base {
	case "index.js", "index.jsx", "index.ts", "index.tsx", "index.mjs", "index.cjs", "index.mts", "index.cts":
		return true
	default:
		return false
	}
}

type classifier struct {
	root  string
	graph *jscontext.Graph
}

func newClassifier(root string, graph *jscontext.Graph) *classifier {
	return &classifier{root: root, graph: graph}
}

func (c *classifier) classify(file string) []boundary.Tag {
	var tags []boundary.Tag
	tags = append(tags, sourceTags(c.root, file)...)
	if fp := c.graph.Files[file]; fp != nil {
		tags = append(tags, targetTags(file, fp)...)
	}
	return tags
}

func sourceTags(root, file string) []boundary.Tag {
	lower := filepath.ToSlash(strings.ToLower(file))
	base := strings.ToLower(filepath.Base(file))
	var tags []boundary.Tag
	if strings.Contains(base, ".controller.") || hasPathSegment(lower, "controllers") || fileContains(root, file, "@Controller") {
		tags = append(tags, boundary.Tag{Kind: "controller", Reason: "controller layer file", Confidence: core.ConfidenceRule})
	}
	if strings.Contains(base, ".handler.") || strings.Contains(base, ".route.") || hasPathSegment(lower, "handlers") || hasPathSegment(lower, "routes") {
		tags = append(tags, boundary.Tag{Kind: "handler", Reason: "route/handler layer file", Confidence: core.ConfidenceRule})
	}
	return tags
}

func targetTags(file string, fp *jscontext.FileParse) []boundary.Tag {
	lower := filepath.ToSlash(strings.ToLower(file))
	base := strings.ToLower(filepath.Base(file))
	var tags []boundary.Tag
	if strings.Contains(base, ".repository.") || base == "repository.ts" || base == "repository.js" || hasPathSegment(lower, "repositories") {
		tags = append(tags, boundary.Tag{Kind: "repository", Reason: "repository layer module", Confidence: core.ConfidenceRule})
	}
	for _, im := range fp.Imports {
		source := normalizePackageSource(im.Source)
		if isORMPackage(source) {
			tags = append(tags, boundary.Tag{Kind: "orm-client", Reason: "imports ORM/database package " + im.Source, Confidence: core.ConfidenceRule})
			if source == "mongoose" || source == "@nestjs/mongoose" {
				tags = append(tags, boundary.Tag{Kind: "data-model", Reason: "imports MongoDB model package " + im.Source, Confidence: core.ConfidenceRule})
			}
			break
		}
	}
	return tags
}

func directPackageBoundaryFindings(analyzer string, graph *jscontext.Graph, classifier *classifier) []core.Finding {
	var out []core.Finding
	for _, file := range sortedGraphFiles(graph) {
		if !hasSourceLayer(classifier.classify(file)) {
			continue
		}
		fp := graph.Files[file]
		if fp == nil {
			continue
		}
		for _, im := range fp.Imports {
			source := normalizePackageSource(im.Source)
			if !isORMPackage(source) {
				continue
			}
			out = append(out, core.Finding{
				Analyzer:   analyzer,
				RuleID:     "architecture.layered-boundary",
				Severity:   core.SeverityWarning,
				Level:      core.SeverityWarning.String(),
				Category:   "quality",
				Confidence: core.ConfidenceRule,
				File:       file,
				Line:       im.Line,
				Message:    fmt.Sprintf("Controller/handler imports ORM/database package %q directly. Request entrypoints should depend on service/use-case code, not data-access clients.", im.Source),
				Fix:        "Move database access behind a service/use-case layer and inject or call that layer from the controller/handler.",
			})
		}
	}
	return out
}

func hasSourceLayer(tags []boundary.Tag) bool {
	for _, tag := range tags {
		if tag.Kind == "controller" || tag.Kind == "handler" {
			return true
		}
	}
	return false
}

func findingFromViolation(analyzer string, v boundary.Violation) core.Finding {
	msg := fmt.Sprintf("Controller/handler imports data-access layer through %q; reaches %s (%s). Import chain: %s.", v.ImportSource, v.TargetFile, v.TargetReason, strings.Join(v.Chain, " -> "))
	return core.Finding{
		Analyzer:   analyzer,
		RuleID:     v.RuleID,
		Severity:   core.SeverityWarning,
		Level:      core.SeverityWarning.String(),
		Category:   "quality",
		Confidence: v.Confidence,
		File:       v.SourceFile,
		Line:       v.Line,
		Message:    msg,
		Fix:        "Keep controllers/handlers thin: call a service/use-case layer, and keep repositories, ORM clients, and database models behind that layer.",
	}
}

func hasPathSegment(path, segment string) bool {
	for _, part := range strings.Split(path, "/") {
		if part == segment {
			return true
		}
	}
	return false
}

func fileContains(root, file, needle string) bool {
	data, err := os.ReadFile(filepath.Join(root, file))
	if err != nil {
		return false
	}
	return strings.Contains(string(data), needle)
}

func normalizePackageSource(source string) string {
	source = strings.TrimPrefix(source, "node:")
	if strings.HasPrefix(source, "@prisma/client") {
		return "@prisma/client"
	}
	if strings.HasPrefix(source, "@nestjs/mongoose") {
		return "@nestjs/mongoose"
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

func isORMPackage(source string) bool {
	switch source {
	case "@prisma/client", "typeorm", "sequelize", "drizzle-orm", "knex",
		"mongoose", "@nestjs/mongoose", "pg", "postgres", "mysql", "mysql2",
		"better-sqlite3", "sqlite3":
		return true
	default:
		return false
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

func dedupeFindings(in []core.Finding) []core.Finding {
	seen := map[string]bool{}
	out := make([]core.Finding, 0, len(in))
	for _, f := range in {
		key := fmt.Sprintf("%s:%s:%d:%s", f.RuleID, f.File, f.Line, f.Message)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, f)
	}
	return out
}
