package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/aykutssert/inspector/internal/app"
	"github.com/aykutssert/inspector/internal/core"
	"github.com/aykutssert/inspector/internal/report"
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

	r, err := app.Scan(app.ScanOptions{
		Root:     root,
		DiffOnly: *diff,
		RulesDir: *rulesDir,
	}, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

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

	ctx, err := app.Context(app.ContextOptions{
		Root:   *rootFlag,
		Target: target,
	}, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
