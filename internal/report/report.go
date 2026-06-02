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
		return
	}
	s := r.Summary
	fmt.Fprintf(w, "%d finding(s): %d error, %d warning, %d info\n",
		s.Total, s.Counts["error"], s.Counts["warning"], s.Counts["info"])
	if len(s.TopFiles) > 0 {
		fmt.Fprint(w, "hotspots:")
		for _, tf := range s.TopFiles {
			fmt.Fprintf(w, " %s(%d)", tf.File, tf.Count)
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintln(w)
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
}
