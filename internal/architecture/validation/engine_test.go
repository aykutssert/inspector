package validation

import (
	"testing"
)

func TestValidationEngine(t *testing.T) {
	endpoints := []Endpoint{
		{
			Framework: "nestjs",
			File:      "controller.ts",
			Line:      10,
			Route:     "POST /users",
			Handler:   "createUser",
			HasBody:   true,
			Validated: false,
		},
		{
			Framework: "nestjs",
			File:      "controller.ts",
			Line:      20,
			Route:     "POST /login",
			Handler:   "login",
			HasBody:   true,
			Validated: true,
		},
		{
			Framework: "express",
			File:      "routes.js",
			Line:      30,
			Route:     "PUT /items",
			Handler:   "updateItem",
			HasBody:   true,
			Validated: false,
		},
	}

	rules := []Rule{
		{
			ID:        "nestjs-missing-validation-pipe",
			Framework: "nestjs",
			Message:   "Missing NestJS validation pipe",
		},
	}

	violations := Analyze(endpoints, rules)

	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}

	if violations[0].File != "controller.ts" || violations[0].Line != 10 {
		t.Errorf("expected violation at controller.ts:10, got %s:%d", violations[0].File, violations[0].Line)
	}

	if violations[0].RuleID != "nestjs-missing-validation-pipe" {
		t.Errorf("expected rule ID nestjs-missing-validation-pipe, got %s", violations[0].RuleID)
	}
}
