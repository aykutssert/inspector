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

	// RequireConsistency raises the actionable rate: only flag an endpoint that
	// lacks the policy when a sibling in the same scope (controller class, or
	// file/router when there is no class) DOES apply it. A scope where every
	// endpoint is uncovered is treated as an intentionally public surface (a
	// health check, a webhook, global middleware we cannot see) and stays
	// silent. A mixed scope signals a forgotten guard, which is worth surfacing.
	RequireConsistency bool
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
		if len(rule.RequiredAnyOf) == 0 {
			continue
		}
		reqMap := make(map[string]bool)
		for _, p := range rule.RequiredAnyOf {
			reqMap[p] = true
		}
		exclMap := make(map[string]bool)
		for _, e := range rule.Exclusions {
			exclMap[e] = true
		}

		// In consistency mode, first find which scopes prove the policy is
		// applied somewhere; scopes that are entirely uncovered are skipped.
		var scopeHasCompliant map[string]bool
		if rule.RequireConsistency {
			scopeHasCompliant = make(map[string]bool)
			for _, ep := range endpoints {
				if !ruleApplies(rule, ep, exclMap) {
					continue
				}
				if hasAnyPolicy(ep, reqMap) {
					scopeHasCompliant[scopeKey(ep)] = true
				}
			}
		}

		for _, ep := range endpoints {
			if !ruleApplies(rule, ep, exclMap) {
				continue
			}
			if hasAnyPolicy(ep, reqMap) {
				continue
			}
			if rule.RequireConsistency && !scopeHasCompliant[scopeKey(ep)] {
				continue
			}
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

	return out
}

// ruleApplies reports whether the rule should evaluate this endpoint: the
// framework matches (when constrained) and no exclusion tag bypasses it.
func ruleApplies(rule Rule, ep Endpoint, exclMap map[string]bool) bool {
	if rule.Framework != "" && ep.Framework != rule.Framework {
		return false
	}
	for _, excl := range ep.Exclusions {
		if exclMap[excl] {
			return false
		}
	}
	return true
}

// hasAnyPolicy reports whether the endpoint carries at least one required policy.
func hasAnyPolicy(ep Endpoint, reqMap map[string]bool) bool {
	for _, pol := range ep.Policies {
		if reqMap[pol] {
			return true
		}
	}
	return false
}

// scopeKey groups endpoints by their enclosing unit — the controller class when
// present, otherwise the file (router module) — within a framework.
func scopeKey(ep Endpoint) string {
	unit := ep.Class
	if unit == "" {
		unit = ep.File
	}
	return ep.Framework + "|" + unit
}
