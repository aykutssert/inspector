package policy

import "testing"

func TestPolicyCoverageEngine(t *testing.T) {
	tests := []struct {
		name      string
		endpoints []Endpoint
		rules     []Rule
		wantCount int
		wantRules []string
	}{
		{
			name: "NestJS Auth Guard Coverage",
			endpoints: []Endpoint{
				{
					Framework: "nestjs",
					File:      "users.controller.ts",
					Line:      12,
					Class:     "UsersController",
					Handler:   "getProfile",
					Route:     "GET /users/profile",
					Policies:  []string{"JwtAuthGuard"},
				},
				{
					Framework:  "nestjs",
					File:       "users.controller.ts",
					Line:       18,
					Class:      "UsersController",
					Handler:    "register",
					Route:      "POST /users/register",
					Exclusions: []string{"Public"},
				},
				{
					Framework: "nestjs",
					File:      "users.controller.ts",
					Line:      25,
					Class:     "UsersController",
					Handler:   "unsafeDelete",
					Route:     "DELETE /users/:id",
					// Missing both JwtAuthGuard (policy) and Public (exclusion)
				},
			},
			rules: []Rule{
				{
					ID:            "nestjs.missing-auth-guard",
					Framework:     "nestjs",
					RequiredAnyOf: []string{"JwtAuthGuard", "AuthGuard"},
					Exclusions:    []string{"Public"},
					Message:       "NestJS controller route is missing authentication guards or explicit @Public() exclusion.",
				},
			},
			wantCount: 1,
			wantRules: []string{"nestjs.missing-auth-guard"},
		},
		{
			name: "Express Middleware Chain Coverage",
			endpoints: []Endpoint{
				{
					Framework: "express",
					File:      "routes.js",
					Line:      10,
					Handler:   "getDashboard",
					Route:     "GET /dashboard",
					Policies:  []string{"auth", "rateLimiter"},
				},
				{
					Framework: "express",
					File:      "routes.js",
					Line:      15,
					Handler:   "getMetrics",
					Route:     "GET /metrics",
					// Missing auth middleware
					Policies: []string{"rateLimiter"},
				},
			},
			rules: []Rule{
				{
					ID:            "express.missing-auth-middleware",
					Framework:     "express",
					RequiredAnyOf: []string{"auth", "requireAuth"},
					Message:       "Express route is missing required authentication middleware.",
				},
			},
			wantCount: 1,
			wantRules: []string{"express.missing-auth-middleware"},
		},
		{
			name: ".NET (ASP.NET Core) Authorization Attribute Coverage",
			endpoints: []Endpoint{
				{
					Framework: "dotnet",
					File:      "AccountController.cs",
					Line:      15,
					Class:     "AccountController",
					Handler:   "GetSettings",
					Route:     "GET /account/settings",
					Policies:  []string{"Authorize"},
				},
				{
					Framework:  "dotnet",
					File:       "AccountController.cs",
					Line:       22,
					Class:      "AccountController",
					Handler:    "Login",
					Route:      "POST /account/login",
					Exclusions: []string{"AllowAnonymous"},
				},
				{
					Framework: "dotnet",
					File:      "AccountController.cs",
					Line:      30,
					Class:     "AccountController",
					Handler:   "UnsafeAdminReset",
					Route:     "POST /account/admin/reset",
					// Missing [Authorize] or [AllowAnonymous]
				},
			},
			rules: []Rule{
				{
					ID:            "dotnet.missing-authorize-attribute",
					Framework:     "dotnet",
					RequiredAnyOf: []string{"Authorize"},
					Exclusions:    []string{"AllowAnonymous"},
					Message:       "ASP.NET Core controller endpoint is missing [Authorize] attribute or [AllowAnonymous] bypass.",
				},
			},
			wantCount: 1,
			wantRules: []string{"dotnet.missing-authorize-attribute"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations := Analyze(tt.endpoints, tt.rules)
			if len(violations) != tt.wantCount {
				t.Fatalf("want %d violations, got %d: %+v", tt.wantCount, len(violations), violations)
			}
			for i, v := range violations {
				if v.RuleID != tt.wantRules[i] {
					t.Errorf("want violation rule ID %q, got %q", tt.wantRules[i], v.RuleID)
				}
			}
		})
	}
}
