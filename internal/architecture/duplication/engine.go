package duplication

import (
	"fmt"
	"sort"
)

// Literal represents a literal occurrence in source code.
type Literal struct {
	Value string // String representation of the literal (normalized)
	Kind  string // "string", "number", "boolean", etc.
	Line  int    // Line number where this literal appears
}

// Rule defines thresholds for identifying repeated magic literals.
type Rule struct {
	ID              string
	ThresholdString int
	ThresholdNumber int
	ThresholdOther  int
	MaxViolations   int
}

// Violation represents a duplicated literal that exceeded the threshold.
type Violation struct {
	RuleID  string `json:"rule_id"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Message string `json:"message"`
	Value   string `json:"value"`
	Kind    string `json:"kind"`
	Count   int    `json:"count"`
}

type literalStat struct {
	kind      string
	value     string
	firstLine int
	count     int
}

// Analyze performs frequency analysis on a list of literals inside a file.
func Analyze(filePath string, literals []Literal, rules []Rule) []Violation {
	var out []Violation

	for _, rule := range rules {
		stats := make(map[string]*literalStat)

		for _, lit := range literals {
			key := lit.Kind + ":" + lit.Value
			stat, exists := stats[key]
			if !exists {
				stat = &literalStat{
					kind:      lit.Kind,
					value:     lit.Value,
					firstLine: lit.Line,
				}
				stats[key] = stat
			}
			stat.count++
		}

		var fileViolations []Violation
		for _, stat := range stats {
			threshold := rule.ThresholdOther
			if stat.kind == "string" {
				threshold = rule.ThresholdString
			} else if stat.kind == "number" {
				threshold = rule.ThresholdNumber
			}

			if threshold > 0 && stat.count >= threshold {
				msg := fmt.Sprintf("%s literal %s is repeated %d times in this file; repeated domain values are easy to mistype and hard for agents to safely change.",
					stat.kind, stat.value, stat.count)
				fileViolations = append(fileViolations, Violation{
					RuleID:  rule.ID,
					File:    filePath,
					Line:    stat.firstLine,
					Message: msg,
					Value:   stat.value,
					Kind:    stat.kind,
					Count:   stat.count,
				})
			}
		}

		// Sort by first occurrence line number
		sort.Slice(fileViolations, func(i, j int) bool {
			return fileViolations[i].Line < fileViolations[j].Line
		})

		// Apply limit
		if rule.MaxViolations > 0 && len(fileViolations) > rule.MaxViolations {
			fileViolations = fileViolations[:rule.MaxViolations]
		}

		out = append(out, fileViolations...)
	}

	return out
}
