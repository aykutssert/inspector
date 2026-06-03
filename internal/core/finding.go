package core

// lower value = more critical (sorts first)
type Severity int

const (
	SeverityError Severity = iota
	SeverityWarning
	SeverityInfo
)

func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	default:
		return "info"
	}
}

// Confidence separates deterministic, high-precision findings from heuristic
// signals the agent should verify rather than trust outright.
const (
	// ConfidenceRule: deterministic, high precision. A false positive is a bug.
	ConfidenceRule = "rule"
	// ConfidenceHint: heuristic — "this may be a problem here." The LLM judges
	// it against the surrounding context; never presented as a definite defect.
	ConfidenceHint = "hint"
)

type Finding struct {
	Analyzer string   `json:"analyzer"`
	RuleID   string   `json:"rule_id"`
	Severity Severity `json:"-"`
	Level    string   `json:"severity"`
	// Category classifies the kind of issue, independent of severity:
	// "security", "bug", "performance", or "quality". Empty when unknown.
	Category string `json:"category,omitempty"`
	// Confidence is "rule" (deterministic) or "hint" (heuristic, verify).
	// Empty is treated as "rule".
	Confidence string `json:"confidence,omitempty"`
	File       string `json:"file,omitempty"`
	Line       int    `json:"line,omitempty"`
	Message    string `json:"message"`
	Fix        string `json:"fix,omitempty"`
	Context    string `json:"context,omitempty"`
	Snippet    string `json:"snippet,omitempty"`
}
