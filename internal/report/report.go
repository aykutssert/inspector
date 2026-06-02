package report

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/aykutssert/inspector/internal/core"
)

func JSON(w io.Writer, r core.Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

func Terminal(w io.Writer, r core.Report) {
	if len(r.Findings) == 0 {
		fmt.Fprintln(w, "No findings.")
	}
	for _, f := range r.Findings {
		loc := f.File
		if f.Line > 0 {
			loc = fmt.Sprintf("%s:%d", f.File, f.Line)
		}
		fmt.Fprintf(w, "[%s] %s  %s\n", f.Level, f.Analyzer, loc)
		fmt.Fprintf(w, "  %s\n", f.Message)
		if f.Fix != "" {
			fmt.Fprintf(w, "  fix: %s\n", f.Fix)
		}
	}
	fmt.Fprintf(w, "\n%d finding(s).", len(r.Findings))
	if len(r.Skipped) > 0 {
		fmt.Fprintf(w, " skipped (not installed): %v", r.Skipped)
	}
	fmt.Fprintln(w)
}
