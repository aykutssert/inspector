package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aykutssert/inspector/internal/analyzers/gitlog"
	"github.com/aykutssert/inspector/internal/analyzers/osv"
	"github.com/aykutssert/inspector/internal/analyzers/oxlint"
	"github.com/aykutssert/inspector/internal/analyzers/reacthint"
	"github.com/aykutssert/inspector/internal/analyzers/semgrep"
	"github.com/aykutssert/inspector/internal/codegraph"
	"github.com/aykutssert/inspector/internal/core"
	"github.com/aykutssert/inspector/internal/lang"
	"github.com/aykutssert/inspector/internal/lang/javascript"
	"github.com/aykutssert/inspector/internal/report"
	"github.com/aykutssert/inspector/internal/scan"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "scan":
		runScan(os.Args[2:])
	case "context":
		runContext(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "inspector — deterministic code security/quality scanner")
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  inspector scan [--diff] [path]")
	fmt.Fprintln(os.Stderr, "  inspector context [--root dir] <file | file:symbol | symbol>")
}

func runScan(args []string) {
	fs := flag.NewFlagSet("scan", flag.ExitOnError)
	diff := fs.Bool("diff", false, "scan only locally changed files")
	rulesDir := fs.String("rules", "rules", "directory holding YAML rule packs")
	_ = fs.Parse(args)

	root := "."
	if fs.NArg() > 0 {
		root = fs.Arg(0)
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	// add new languages here
	jsAdapter := javascript.New(*rulesDir)
	adapters := []core.LanguageAdapter{jsAdapter}
	registry := lang.NewRegistry(adapters...)

	// Each adapter declares where its user-authored rule packs live; semgrep
	// loads them on top of the registry packs. Resolve to absolute so semgrep
	// (which runs with its cwd at the scan root) finds them. Missing dirs are
	// ignored by the analyzer.
	var customRuleDirs []string
	for _, ad := range adapters {
		for _, d := range ad.Rules() {
			if abs, err := filepath.Abs(d); err == nil {
				customRuleDirs = append(customRuleDirs, abs)
			}
		}
	}

	files, err := scan.Discover(absRoot, *diff, adapters)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error discovering files:", err)
		os.Exit(1)
	}

	var changed []string
	if *diff {
		if changed, err = scan.Changed(absRoot); err != nil {
			fmt.Fprintln(os.Stderr, "error listing changed files:", err)
			os.Exit(1)
		}
	}

	ctx := core.ProjectContext{
		Root:      absRoot,
		DiffOnly:  *diff,
		Files:     files,
		Languages: registry.Detect(files),
		Changed:   changed,
	}

	// Analyzers — add new analyzers here, orchestrator stays untouched.
	// add new analyzers here
	orch := core.New(
		semgrep.New("", customRuleDirs...),
		oxlint.New(),
		reacthint.New(),
		osv.New(),
		gitlog.New(),
	)
	r := orch.Run(ctx)

	// Format auto-selects: JSON when piped (an agent reads it), human text on a
	// terminal. No flag needed.
	if isTTY(os.Stdout) {
		report.Terminal(os.Stdout, r)
	} else if err := report.JSON(os.Stdout, r); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	// non-zero exit on any error-level finding (CI / agents)
	for _, f := range r.Findings {
		if f.Severity == core.SeverityError {
			os.Exit(1)
		}
	}
}

// isTTY reports whether w is an interactive terminal (vs a pipe/file).
func isTTY(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func runContext(args []string) {
	fs := flag.NewFlagSet("context", flag.ExitOnError)
	rootFlag := fs.String("root", ".", "project root to index")
	_ = fs.Parse(args)
	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "error: missing target (file, file:symbol, or symbol)")
		os.Exit(2)
	}
	target := fs.Arg(0)

	absRoot, err := filepath.Abs(*rootFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	adapters := []core.LanguageAdapter{javascript.New("")}
	files, err := scan.Discover(absRoot, false, adapters)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error discovering files:", err)
		os.Exit(1)
	}

	g := codegraph.Build(absRoot, files)
	ctx := g.GetContext(target)

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
