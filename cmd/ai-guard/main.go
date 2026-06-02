package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aykutssert/inspector/internal/analyzers/gitlog"
	"github.com/aykutssert/inspector/internal/analyzers/osv"
	"github.com/aykutssert/inspector/internal/analyzers/semgrep"
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
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "ai-guard — deterministic code security/quality scanner")
	fmt.Fprintln(os.Stderr, "usage: ai-guard scan [--diff] [--json] [path]")
}

func runScan(args []string) {
	fs := flag.NewFlagSet("scan", flag.ExitOnError)
	diff := fs.Bool("diff", false, "scan only locally changed files")
	asJSON := fs.Bool("json", false, "emit JSON report (for agent harnesses)")
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

	files, err := scan.Discover(absRoot, *diff, adapters)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error discovering files:", err)
		os.Exit(1)
	}

	ctx := core.ProjectContext{
		Root:      absRoot,
		DiffOnly:  *diff,
		Files:     files,
		Languages: registry.Detect(files),
	}

	// Analyzers — add new analyzers here, orchestrator stays untouched.
	// add new analyzers here
	orch := core.New(
		semgrep.New("auto"),
		osv.New(),
		gitlog.New(),
	)
	r := orch.Run(ctx)

	if *asJSON {
		if err := report.JSON(os.Stdout, r); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	} else {
		report.Terminal(os.Stdout, r)
	}

	// non-zero exit on any error-level finding (CI / agents)
	for _, f := range r.Findings {
		if f.Severity == core.SeverityError {
			os.Exit(1)
		}
	}
}
