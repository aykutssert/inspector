package validation

// Endpoint represents an API endpoint to evaluate.
type Endpoint struct {
	Framework string
	File      string
	Line      int
	Route     string
	Handler   string
	HasBody   bool
	Validated bool
}

// Rule defines input validation rules.
type Rule struct {
	ID        string
	Framework string
	Message   string
}

// Violation represents an unvalidated API endpoint.
type Violation struct {
	RuleID  string `json:"rule_id"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Message string `json:"message"`
}

// Analyze checks endpoints against rules to identify validation gaps.
func Analyze(endpoints []Endpoint, rules []Rule) []Violation {
	var out []Violation

	for _, rule := range rules {
		for _, ep := range endpoints {
			if rule.Framework != "all" && ep.Framework != rule.Framework {
				continue
			}

			// Endpoint accepts a body but has no validation configured
			if ep.HasBody && !ep.Validated {
				out = append(out, Violation{
					RuleID:  rule.ID,
					File:    ep.File,
					Line:    ep.Line,
					Message: rule.Message,
				})
			}
		}
	}

	return out
}
