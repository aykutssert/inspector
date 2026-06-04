package policy

import "fmt"

// Endpoint represents an HTTP route handler or controller endpoint in any
// framework (NestJS, Express, FastAPI, ASP.NET Core, Gin, etc.).
type Endpoint struct {
	Framework  string   // Framework identifier (e.g. "nestjs", "express").
	File       string   // File path containing the endpoint.
	Line       int      // Line number of the route definition.
	Class      string   // Class name (optional, e.g. "UsersController").
	Handler    string   // Handler function or method name (e.g. "getUser").
	Route      string   // HTTP method and path (e.g. "GET /users/:id").
	Policies   []string // Policies/guards/middleware applied to this endpoint (e.g. ["AuthGuard", "JwtAuthGuard"]).
	Exclusions []string // Bypass decorators/tags applied to this endpoint (e.g. ["Public", "AllowAnonymous"]).
}

// Rule defines security policy coverage requirements.
type Rule struct {
	ID            string   // Unique rule identifier (e.g. "nestjs.missing-auth-guard").
	Framework     string   // Framework identifier (e.g. "nestjs", "express") to restrict evaluation.
	RequiredAnyOf []string // At least one of these policies must be present on the endpoint (or its parent class/router).
	Exclusions    []string // If any of these exclusion tags are present, the rule is skipped.
	Message       string   // Error message to display on violation.
}

// Violation represents an endpoint that failed to meet the policy requirements.
type Violation struct {
	RuleID  string `json:"rule_id"`
	File    string `json:"file"`
	Line    int    `json:"line"`
	Route   string `json:"route"`
	Handler string `json:"handler"`
	Message string `json:"message"`
}

// Analyze evaluates a list of endpoints against policy rules.
// It is completely language and framework agnostic, acting as a pure policy decision engine.
func Analyze(endpoints []Endpoint, rules []Rule) []Violation {
	var out []Violation

	for _, rule := range rules {
		reqMap := make(map[string]bool)
		for _, p := range rule.RequiredAnyOf {
			reqMap[p] = true
		}

		exclMap := make(map[string]bool)
		for _, e := range rule.Exclusions {
			exclMap[e] = true
		}

		for _, ep := range endpoints {
			// 0. Filter by framework if specified in the rule
			if rule.Framework != "" && ep.Framework != rule.Framework {
				continue
			}

			// 1. Check if endpoint is excluded
			isExcluded := false
			for _, excl := range ep.Exclusions {
				if exclMap[excl] {
					isExcluded = true
					break
				}
			}
			if isExcluded {
				continue
			}

			// 2. Check if required policies are satisfied
			hasPolicy := false
			for _, pol := range ep.Policies {
				if reqMap[pol] {
					hasPolicy = true
					break
				}
			}

			// 3. If required policies are defined but none matched, report violation
			if len(rule.RequiredAnyOf) > 0 && !hasPolicy {
				msg := rule.Message
				if msg == "" {
					msg = fmt.Sprintf("Endpoint %s (%s) is missing required security policy.", ep.Route, ep.Handler)
				}
				out = append(out, Violation{
					RuleID:  rule.ID,
					File:    ep.File,
					Line:    ep.Line,
					Route:   ep.Route,
					Handler: ep.Handler,
					Message: msg,
				})
			}
		}
	}

	return out
}
