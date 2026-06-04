package complexity

// Entity represents a structural unit of code (class, function, struct, UI component) to assess.
type Entity struct {
	Name      string // Entity name (e.g. UsersController, Card)
	Type      string // Entity type ("class", "function", "struct", "component")
	LineCount int    // Lines of code (excluding comments and blank lines)
	Inputs    int    // Girdiler: Parameters count, props count, or fields count
	Deps      int    // Bağımlılıklar: Injected services count, hooks count, or external calls
	StartLine int    // Starting line of the entity
}

// Rule defines complexity thresholds for entities.
type Rule struct {
	ID        string
	Type      string // Target entity type ("class", "function", "struct", "component", "all")
	MaxLines  int    // Maximum allowed lines
	MaxInputs int    // Maximum allowed inputs (parameters/props/fields)
	MaxDeps   int    // Maximum allowed dependencies (hooks/injections/calls)
	Message   string // Custom warning message
}

// Violation represents an entity violating complexity limits.
type Violation struct {
	RuleID  string `json:"rule_id"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Message string `json:"message"`
}

// Analyze evaluates a list of entities in a file against complexity rules.
func Analyze(filePath string, entities []Entity, rules []Rule) []Violation {
	var out []Violation

	for _, rule := range rules {
		for _, ent := range entities {
			if rule.Type != "all" && ent.Type != rule.Type {
				continue
			}

			violation := false

			// Structural fan-in/out is a standalone smell: too many inputs
			// (params/props/fields) or too many dependencies (calls/hooks/
			// injections) is hard to maintain regardless of length.
			if rule.MaxInputs > 0 && ent.Inputs >= rule.MaxInputs {
				violation = true
			}
			if rule.MaxDeps > 0 && ent.Deps >= rule.MaxDeps {
				violation = true
			}

			// Size alone is NOT a violation. A long but simple entity (a big
			// switch, a config aggregator, generated code) is usually justified,
			// and flagging it is low-actionable noise. Length only counts when it
			// is corroborated by moderate busyness (60% of the input/dep budget).
			if !violation && rule.MaxLines > 0 && (rule.MaxInputs > 0 || rule.MaxDeps > 0) {
				busyLines := int(float64(rule.MaxLines) * 0.6)
				busyInputs := int(float64(rule.MaxInputs) * 0.6)
				busyDeps := int(float64(rule.MaxDeps) * 0.6)

				isModeratelyLarge := ent.LineCount >= busyLines
				hasModerateInputs := rule.MaxInputs > 0 && ent.Inputs >= busyInputs
				hasModerateDeps := rule.MaxDeps > 0 && ent.Deps >= busyDeps

				if isModeratelyLarge && (hasModerateInputs || hasModerateDeps) {
					violation = true
				}
			}

			if violation {
				out = append(out, Violation{
					RuleID:  rule.ID,
					File:    filePath,
					Line:    ent.StartLine,
					Message: rule.Message,
				})
			}
		}
	}

	return out
}
