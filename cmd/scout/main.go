package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/aykutssert/scout/internal/app"
	"github.com/aykutssert/scout/internal/core"
	"github.com/aykutssert/scout/internal/report"
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
	case "doctor":
		runDoctor(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "scout — deterministic code security/quality scanner")
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  scout scan [--diff] [path]")
	fmt.Fprintln(os.Stderr, "  scout context [--root dir]                  # repo map")
	fmt.Fprintln(os.Stderr, "  scout context [--root dir] <file | file:symbol | symbol>")
	fmt.Fprintln(os.Stderr, "  scout doctor [--json]")
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

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")

	// No target → repo map mode.
	if fs.NArg() == 0 {
		m, err := app.Map(app.MapOptions{Root: *rootFlag}, nil)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		if err := enc.Encode(m); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}

	// With target → symbol / file context.
	ctx, err := app.Context(app.ContextOptions{
		Root:   *rootFlag,
		Target: fs.Arg(0),
	}, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if err := enc.Encode(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func runDoctor(args []string) {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	jsonFlag := fs.Bool("json", false, "output in JSON format")
	_ = fs.Parse(args)

	diag := app.Diagnose()

	if *jsonFlag {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(diag); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	} else {
		useColor := isTTY(os.Stdout)
		fmt.Println("Scout Diagnostic Report")
		fmt.Println("=======================")
		for _, res := range diag.Results {
			statusStr := string(res.Status)
			if useColor {
				switch res.Status {
				case app.StatusOk:
					statusStr = "\033[32mOK\033[0m"
				case app.StatusWarning:
					statusStr = "\033[33mWARNING\033[0m"
				case app.StatusError:
					statusStr = "\033[31mERROR\033[0m"
				}
			}

			if res.Status == app.StatusOk {
				fmt.Printf("[%s] %s (%s)\n", statusStr, res.Name, res.Version)
			} else {
				fmt.Printf("[%s] %s: %s\n", statusStr, res.Name, res.Error)
				if res.InstallHint != "" {
					fmt.Printf("      Fix: %s\n", res.InstallHint)
				}
			}
		}

		overallStr := string(diag.OverallStatus)
		if useColor {
			switch diag.OverallStatus {
			case app.StatusOk:
				overallStr = "\033[32mOK\033[0m"
			case app.StatusWarning:
				overallStr = "\033[33mWARNING\033[0m"
			case app.StatusError:
				overallStr = "\033[31mERROR\033[0m"
			}
		}
		fmt.Printf("\nOverall Status: %s\n", overallStr)
	}

	if diag.OverallStatus == app.StatusError {
		os.Exit(1)
	}
}
