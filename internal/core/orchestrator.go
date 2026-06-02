package core

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
)

type Orchestrator struct {
	analyzers []Analyzer
}

func New(analyzers ...Analyzer) *Orchestrator {
	return &Orchestrator{analyzers: analyzers}
}

type FileCount struct {
	File  string `json:"file"`
	Count int    `json:"count"`
}

type Summary struct {
	Total    int            `json:"total"`
	Counts   map[string]int `json:"counts"`
	TopFiles []FileCount    `json:"top_files,omitempty"`
}

type Report struct {
	Summary  Summary   `json:"summary"`
	Findings []Finding `json:"findings"`
}

func (o *Orchestrator) Run(ctx ProjectContext) Report {
	var report Report
	for _, a := range o.analyzers {
		// Fail-closed: a missing scanner or a scanner error is surfaced as an
		// error-level finding (non-zero exit), never a silent pass. A clean scan
		// must mean the tools actually ran.
		if !a.Available() {
			report.Findings = append(report.Findings, Finding{
				Analyzer: a.Name(),
				RuleID:   "analyzer-unavailable",
				Severity: SeverityError,
				Level:    SeverityError.String(),
				Message:  a.Name() + " is not installed",
				Fix:      "install " + a.Name() + " to enable this scanner",
			})
			continue
		}
		found, err := a.Scan(ctx)
		if err != nil {
			report.Findings = append(report.Findings, Finding{
				Analyzer: a.Name(),
				RuleID:   "analyzer-error",
				Severity: SeverityError,
				Level:    SeverityError.String(),
				Message:  a.Name() + " failed: " + err.Error(),
			})
			continue
		}
		report.Findings = append(report.Findings, found...)
	}
	o.sortFindings(report.Findings)
	enrichSnippets(ctx.Root, report.Findings)
	report.Summary = buildSummary(report.Findings)
	return report
}

func buildSummary(findings []Finding) Summary {
	s := Summary{Total: len(findings), Counts: map[string]int{}}
	perFile := map[string]int{}
	for _, f := range findings {
		s.Counts[f.Level]++
		if f.File != "" {
			perFile[f.File]++
		}
	}
	for file, n := range perFile {
		s.TopFiles = append(s.TopFiles, FileCount{File: file, Count: n})
	}
	sort.Slice(s.TopFiles, func(i, j int) bool {
		if s.TopFiles[i].Count != s.TopFiles[j].Count {
			return s.TopFiles[i].Count > s.TopFiles[j].Count
		}
		return s.TopFiles[i].File < s.TopFiles[j].File
	})
	if len(s.TopFiles) > 5 {
		s.TopFiles = s.TopFiles[:5]
	}
	return s
}

// enrichSnippets attaches ±2 lines around each finding's line. Read failures
// leave Snippet empty — never fatal.
func enrichSnippets(root string, findings []Finding) {
	cache := map[string][]string{}
	for i := range findings {
		f := &findings[i]
		if f.File == "" || f.Line <= 0 {
			continue
		}
		lines, ok := cache[f.File]
		if !ok {
			lines = readLines(filepath.Join(root, f.File))
			cache[f.File] = lines
		}
		if len(lines) == 0 {
			continue
		}
		f.Snippet = renderSnippet(lines, f.Line)
	}
}

func readLines(path string) []string {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()
	var lines []string
	sc := bufio.NewScanner(file)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines
}

func renderSnippet(lines []string, line int) string {
	start := line - 2
	if start < 1 {
		start = 1
	}
	end := line + 2
	if end > len(lines) {
		end = len(lines)
	}
	var b []byte
	for n := start; n <= end; n++ {
		marker := "  "
		if n == line {
			marker = "> "
		}
		b = append(b, marker...)
		b = append(b, itoa(n)...)
		b = append(b, " | "...)
		b = append(b, lines[n-1]...)
		if n < end {
			b = append(b, '\n')
		}
	}
	return string(b)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func (o *Orchestrator) sortFindings(f []Finding) {
	sort.SliceStable(f, func(i, j int) bool {
		if f[i].Severity != f[j].Severity {
			return f[i].Severity < f[j].Severity
		}
		if f[i].File != f[j].File {
			return f[i].File < f[j].File
		}
		return f[i].Line < f[j].Line
	})
}
