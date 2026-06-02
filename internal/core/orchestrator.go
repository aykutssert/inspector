package core

import (
	"sort"
)

type Orchestrator struct {
	analyzers []Analyzer
}

func New(analyzers ...Analyzer) *Orchestrator {
	return &Orchestrator{analyzers: analyzers}
}

type Report struct {
	Findings []Finding `json:"findings"`
	Skipped  []string  `json:"skipped"`
}

func (o *Orchestrator) Run(ctx ProjectContext) Report {
	var report Report
	for _, a := range o.analyzers {
		if !a.Available() {
			report.Skipped = append(report.Skipped, a.Name())
			continue
		}
		found, err := a.Scan(ctx)
		if err != nil {
			// one analyzer failing must not kill the scan
			report.Findings = append(report.Findings, Finding{
				Analyzer: a.Name(),
				RuleID:   "analyzer-error",
				Severity: SeverityInfo,
				Level:    SeverityInfo.String(),
				Message:  a.Name() + " failed: " + err.Error(),
			})
			continue
		}
		report.Findings = append(report.Findings, found...)
	}
	o.sortFindings(report.Findings)
	return report
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
