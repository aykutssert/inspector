package splitting

// Component represents a UI component (React, Svelte, etc.) in a file.
type Component struct {
	Name       string // Component name
	LineCount  int    // Actual line count (excluding comments/blanks)
	IsExported bool   // Whether it is exported
	StartLine  int    // Start line in the file
}

// FileMetrics defines the sizing and component metrics of a source file.
type FileMetrics struct {
	FilePath   string      // Relative file path
	TotalLines int         // Total physical lines of the file
	Components []Component // List of components in the file
}

// Rule defines thresholds for splitting a component file.
type Rule struct {
	ID                    string
	MaxFileLines          int    // Overall file line limit
	MaxComponentLines     int    // Individual component line limit
	MaxExportedLargeComps int    // Max allowed exported large components in a single file
	Message               string // Violation message
}

// Violation represents a failure to meet the splitting rules.
type Violation struct {
	RuleID  string `json:"rule_id"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Message string `json:"message"`
}

// Analyze evaluates file metrics against splitting rules.
// Flags files exceeding MaxFileLines with more than MaxExportedLargeComps exported large components.
func Analyze(files []FileMetrics, rules []Rule) []Violation {
	var out []Violation

	for _, rule := range rules {
		for _, file := range files {
			// Check file size threshold
			if file.TotalLines < rule.MaxFileLines {
				continue
			}

			// Count exported large components
			largeExportedCount := 0
			var firstLargeComponent Component

			for _, comp := range file.Components {
				if comp.IsExported && comp.LineCount >= rule.MaxComponentLines {
					largeExportedCount++
					if largeExportedCount > rule.MaxExportedLargeComps && firstLargeComponent.Name == "" {
						firstLargeComponent = comp
					}
				}
			}

			// Report violation if threshold exceeded
			if largeExportedCount > rule.MaxExportedLargeComps {
				msg := rule.Message
				line := 1
				if firstLargeComponent.Name != "" {
					line = firstLargeComponent.StartLine
				} else if len(file.Components) > 0 {
					line = file.Components[0].StartLine
				}

				out = append(out, Violation{
					RuleID:  rule.ID,
					File:    file.FilePath,
					Line:    line,
					Message: msg,
				})
			}
		}
	}

	return out
}
