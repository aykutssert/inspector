package archhint

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/aykutssert/inspector/internal/core"
)

func TestFlagsControllerImportingRepositoryDirectly(t *testing.T) {
	findings := scanProject(t, map[string]string{
		"user.controller.ts": `import { UserRepository } from "./user.repository";

@Controller("users")
export class UserController {
  constructor(private readonly repo: UserRepository) {}
}
`,
		"user.repository.ts": `export class UserRepository {}`,
	})

	f := oneFinding(t, findings)
	if f.RuleID != "architecture.layered-boundary" {
		t.Fatalf("rule = %q", f.RuleID)
	}
	if f.File != "user.controller.ts" || f.Line != 1 {
		t.Fatalf("location = %s:%d, want user.controller.ts:1", f.File, f.Line)
	}
	if f.Confidence != core.ConfidenceRule {
		t.Fatalf("confidence = %q, want rule", f.Confidence)
	}
	if !strings.Contains(f.Message, "user.repository.ts") {
		t.Fatalf("message should name repository target, got %q", f.Message)
	}
}

func TestFlagsControllerImportingRepositoryThroughBarrel(t *testing.T) {
	findings := scanProject(t, map[string]string{
		"user.controller.ts": `import { UserRepository } from "./data";

@Controller("users")
export class UserController {}
`,
		"data/index.ts":           `export { UserRepository } from "./user.repository";`,
		"data/user.repository.ts": `export class UserRepository {}`,
	})

	f := oneFinding(t, findings)
	if f.File != "user.controller.ts" || f.Line != 1 {
		t.Fatalf("location = %s:%d, want user.controller.ts:1", f.File, f.Line)
	}
	if !strings.Contains(f.Message, "user.controller.ts -> data/index.ts -> data/user.repository.ts") {
		t.Fatalf("message should include barrel chain, got %q", f.Message)
	}
}

func TestDoesNotFlagControllerServiceRepositoryDelegation(t *testing.T) {
	findings := scanProject(t, map[string]string{
		"user.controller.ts": `import { UserService } from "./user.service";

@Controller("users")
export class UserController {
  constructor(private readonly service: UserService) {}
}
`,
		"user.service.ts":    `import { UserRepository } from "./user.repository"; export class UserService {}`,
		"user.repository.ts": `export class UserRepository {}`,
	})

	if len(findings) != 0 {
		t.Fatalf("controller -> service -> repository should be allowed, got %+v", findings)
	}
}

func TestFlagsControllerImportingORMPackageDirectly(t *testing.T) {
	findings := scanProject(t, map[string]string{
		"user.controller.ts": `import { PrismaClient } from "@prisma/client";

@Controller("users")
export class UserController {
  private readonly prisma = new PrismaClient();
}
`,
	})

	f := oneFinding(t, findings)
	if f.File != "user.controller.ts" || f.Line != 1 {
		t.Fatalf("location = %s:%d, want user.controller.ts:1", f.File, f.Line)
	}
	if !strings.Contains(f.Message, `"@prisma/client"`) {
		t.Fatalf("message should name ORM package, got %q", f.Message)
	}
}

func TestFlagsRouteImportingLocalORMModule(t *testing.T) {
	findings := scanProject(t, map[string]string{
		"routes/user.route.ts": `import { prisma } from "../db/prisma";

export function register(app) {
  app.get("/users", () => prisma.user.findMany());
}
`,
		"db/prisma.ts": `import { PrismaClient } from "@prisma/client";
export const prisma = new PrismaClient();
`,
	})

	f := oneFinding(t, findings)
	if f.File != "routes/user.route.ts" || f.Line != 1 {
		t.Fatalf("location = %s:%d, want routes/user.route.ts:1", f.File, f.Line)
	}
	if !strings.Contains(f.Message, "db/prisma.ts") {
		t.Fatalf("message should name local ORM module, got %q", f.Message)
	}
}

func TestSafeControllerServiceImportIsIgnored(t *testing.T) {
	findings := scanProject(t, map[string]string{
		"user.controller.ts": `import { UserService } from "./user.service";

@Controller("users")
export class UserController {}
`,
		"user.service.ts": `export class UserService {}`,
	})

	if len(findings) != 0 {
		t.Fatalf("safe service import should be ignored, got %+v", findings)
	}
}

func writeProject(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func scanProject(t *testing.T, files map[string]string) []core.Finding {
	t.Helper()
	root := writeProject(t, files)
	got, err := New().Scan(core.ProjectContext{Root: root, Files: projectFiles(t, root), Languages: []string{"javascript"}})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func projectFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	return files
}

func oneFinding(t *testing.T, findings []core.Finding) core.Finding {
	t.Helper()
	if len(findings) != 1 {
		t.Fatalf("findings = %+v, want one", findings)
	}
	return findings[0]
}
