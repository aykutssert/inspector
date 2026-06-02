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

type Finding struct {
	Analyzer string   `json:"analyzer"`
	RuleID   string   `json:"rule_id"`
	Severity Severity `json:"-"`
	Level    string   `json:"severity"`
	// Category classifies the kind of issue, independent of severity:
	// "security", "bug", "performance", or "quality". Empty when unknown.
	Category string `json:"category,omitempty"`
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Message  string `json:"message"`
	Fix      string `json:"fix,omitempty"`
	Context  string `json:"context,omitempty"`
	Snippet  string `json:"snippet,omitempty"`
}
