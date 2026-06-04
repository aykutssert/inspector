package nesthint

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/aykutssert/inspector/internal/core"
)

func TestMissingProviderInSameModule(t *testing.T) {
	root := writeProject(t, map[string]string{
		"user.service.ts": `
import { Injectable } from "@nestjs/common";

@Injectable()
export class UserService {}
`,
		"user.controller.ts": `
import { Controller } from "@nestjs/common";
import { UserService } from "./user.service";

@Controller()
export class UserController {
  constructor(private readonly users: UserService) {}
}
`,
		"user.module.ts": `
import { Module } from "@nestjs/common";
import { UserController } from "./user.controller";

@Module({
  controllers: [UserController],
  providers: [],
})
export class UserModule {}
`,
	})

	findings := scanProject(t, root)
	if len(findings) != 1 {
		t.Fatalf("findings len = %d, want 1: %#v", len(findings), findings)
	}
	got := findings[0]
	if got.RuleID != "nestjs.provider-not-registered" {
		t.Fatalf("rule id = %q", got.RuleID)
	}
	if got.File != "user.controller.ts" || got.Line != 7 {
		t.Fatalf("location = %s:%d, want user.controller.ts:7", got.File, got.Line)
	}
}

func TestProviderInSameModuleIsSafe(t *testing.T) {
	root := writeProject(t, map[string]string{
		"user.service.ts": `
import { Injectable } from "@nestjs/common";

@Injectable()
export class UserService {}
`,
		"user.controller.ts": `
import { Controller } from "@nestjs/common";
import { UserService } from "./user.service";

@Controller()
export class UserController {
  constructor(private readonly users: UserService) {}
}
`,
		"user.module.ts": `
import { Module } from "@nestjs/common";
import { UserController } from "./user.controller";
import { UserService } from "./user.service";

@Module({
  controllers: [UserController],
  providers: [UserService],
})
export class UserModule {}
`,
	})

	if findings := scanProject(t, root); len(findings) != 0 {
		t.Fatalf("findings len = %d, want 0: %#v", len(findings), findings)
	}
}

func TestProviderDependencyIsChecked(t *testing.T) {
	root := writeProject(t, map[string]string{
		"repo.service.ts": `
import { Injectable } from "@nestjs/common";

@Injectable()
export class RepoService {}
`,
		"user.service.ts": `
import { Injectable } from "@nestjs/common";
import { RepoService } from "./repo.service";

@Injectable()
export class UserService {
  constructor(private readonly repo: RepoService) {}
}
`,
		"user.module.ts": `
import { Module } from "@nestjs/common";
import { UserService } from "./user.service";

@Module({
  providers: [UserService],
})
export class UserModule {}
`,
	})

	findings := scanProject(t, root)
	if len(findings) != 1 {
		t.Fatalf("findings len = %d, want 1: %#v", len(findings), findings)
	}
	if findings[0].File != "user.service.ts" || findings[0].Line != 7 {
		t.Fatalf("location = %s:%d, want user.service.ts:7", findings[0].File, findings[0].Line)
	}
}

func TestImportedExportedProviderIsSafe(t *testing.T) {
	root := writeProject(t, map[string]string{
		"shared/user.service.ts": `
import { Injectable } from "@nestjs/common";

@Injectable()
export class UserService {}
`,
		"shared/shared.module.ts": `
import { Module } from "@nestjs/common";
import { UserService } from "./user.service";

@Module({
  providers: [UserService],
  exports: [UserService],
})
export class SharedModule {}
`,
		"user.controller.ts": `
import { Controller } from "@nestjs/common";
import { UserService } from "./shared/user.service";

@Controller()
export class UserController {
  constructor(private readonly users: UserService) {}
}
`,
		"user.module.ts": `
import { Module } from "@nestjs/common";
import { UserController } from "./user.controller";
import { SharedModule } from "./shared/shared.module";

@Module({
  imports: [SharedModule],
  controllers: [UserController],
})
export class UserModule {}
`,
	})

	if findings := scanProject(t, root); len(findings) != 0 {
		t.Fatalf("findings len = %d, want 0: %#v", len(findings), findings)
	}
}

func TestBarrelReExportedProviderIsSafe(t *testing.T) {
	root := writeProject(t, map[string]string{
		"shared/user.service.ts": `
import { Injectable } from "@nestjs/common";

@Injectable()
export class UserService {}
`,
		"shared/shared.module.ts": `
import { Module } from "@nestjs/common";
import { UserService } from "./user.service";

@Module({
  providers: [UserService],
  exports: [UserService],
})
export class SharedModule {}
`,
		"shared/index.ts": `
export { UserService } from "./user.service";
export * from "./shared.module";
`,
		"user.controller.ts": `
import { Controller } from "@nestjs/common";
import { UserService } from "./shared";

@Controller()
export class UserController {
  constructor(private readonly users: UserService) {}
}
`,
		"user.module.ts": `
import { Module } from "@nestjs/common";
import { UserController } from "./user.controller";
import { SharedModule } from "./shared";

@Module({
  imports: [SharedModule],
  controllers: [UserController],
})
export class UserModule {}
`,
	})

	if findings := scanProject(t, root); len(findings) != 0 {
		t.Fatalf("findings len = %d, want 0: %#v", len(findings), findings)
	}
}

func TestPathAliasImportedExportedProviderIsSafe(t *testing.T) {
	root := writeProject(t, map[string]string{
		"tsconfig.base.json": `{
  // Nest monorepos often keep aliases in the base config.
  "compilerOptions": {
    "baseUrl": ".",
    "paths": {
      "@app/*": ["src/*"],
    },
  },
}`,
		"src/shared/user.service.ts": `
import { Injectable } from "@nestjs/common";

@Injectable()
export class UserService {}
`,
		"src/shared/shared.module.ts": `
import { Module } from "@nestjs/common";
import { UserService } from "./user.service";

@Module({
  providers: [UserService],
  exports: [UserService],
})
export class SharedModule {}
`,
		"src/shared/index.ts": `
export { UserService } from "./user.service";
export { SharedModule } from "./shared.module";
`,
		"src/user.controller.ts": `
import { Controller } from "@nestjs/common";
import { UserService } from "@app/shared";

@Controller()
export class UserController {
  constructor(private readonly users: UserService) {}
}
`,
		"src/user.module.ts": `
import { Module } from "@nestjs/common";
import { UserController } from "./user.controller";
import { SharedModule } from "@app/shared";

@Module({
  imports: [SharedModule],
  controllers: [UserController],
})
export class UserModule {}
`,
	})

	if findings := scanProject(t, root); len(findings) != 0 {
		t.Fatalf("findings len = %d, want 0: %#v", len(findings), findings)
	}
}

func TestPathAliasImportedButNotExportedProviderIsReported(t *testing.T) {
	root := writeProject(t, map[string]string{
		"tsconfig.json": `{
  "compilerOptions": {
    "baseUrl": ".",
    "paths": {
      "@app/*": ["src/*"]
    }
  }
}`,
		"src/shared/user.service.ts": `
import { Injectable } from "@nestjs/common";

@Injectable()
export class UserService {}
`,
		"src/shared/shared.module.ts": `
import { Module } from "@nestjs/common";
import { UserService } from "./user.service";

@Module({
  providers: [UserService],
})
export class SharedModule {}
`,
		"src/shared/index.ts": `
export { UserService } from "./user.service";
export { SharedModule } from "./shared.module";
`,
		"src/user.controller.ts": `
import { Controller } from "@nestjs/common";
import { UserService } from "@app/shared";

@Controller()
export class UserController {
  constructor(private readonly users: UserService) {}
}
`,
		"src/user.module.ts": `
import { Module } from "@nestjs/common";
import { UserController } from "./user.controller";
import { SharedModule } from "@app/shared";

@Module({
  imports: [SharedModule],
  controllers: [UserController],
})
export class UserModule {}
`,
	})

	findings := scanProject(t, root)
	if len(findings) != 1 {
		t.Fatalf("findings len = %d, want 1: %#v", len(findings), findings)
	}
	if findings[0].File != "src/user.controller.ts" || findings[0].Line != 7 {
		t.Fatalf("location = %s:%d, want src/user.controller.ts:7", findings[0].File, findings[0].Line)
	}
}

func TestForwardRefImportedExportedProviderIsSafe(t *testing.T) {
	root := writeProject(t, map[string]string{
		"shared/user.service.ts": `
import { Injectable } from "@nestjs/common";

@Injectable()
export class UserService {}
`,
		"shared/shared.module.ts": `
import { Module } from "@nestjs/common";
import { UserService } from "./user.service";

@Module({
  providers: [UserService],
  exports: [UserService],
})
export class SharedModule {}
`,
		"user.controller.ts": `
import { Controller } from "@nestjs/common";
import { UserService } from "./shared/user.service";

@Controller()
export class UserController {
  constructor(private readonly users: UserService) {}
}
`,
		"user.module.ts": `
import { Module, forwardRef } from "@nestjs/common";
import { UserController } from "./user.controller";
import { SharedModule } from "./shared/shared.module";

@Module({
  imports: [forwardRef(() => SharedModule)],
  controllers: [UserController],
})
export class UserModule {}
`,
	})

	if findings := scanProject(t, root); len(findings) != 0 {
		t.Fatalf("findings len = %d, want 0: %#v", len(findings), findings)
	}
}

func TestForwardRefImportedButNotExportedProviderIsReported(t *testing.T) {
	root := writeProject(t, map[string]string{
		"shared/user.service.ts": `
import { Injectable } from "@nestjs/common";

@Injectable()
export class UserService {}
`,
		"shared/shared.module.ts": `
import { Module } from "@nestjs/common";
import { UserService } from "./user.service";

@Module({
  providers: [UserService],
})
export class SharedModule {}
`,
		"user.controller.ts": `
import { Controller } from "@nestjs/common";
import { UserService } from "./shared/user.service";

@Controller()
export class UserController {
  constructor(private readonly users: UserService) {}
}
`,
		"user.module.ts": `
import { Module, forwardRef } from "@nestjs/common";
import { UserController } from "./user.controller";
import { SharedModule } from "./shared/shared.module";

@Module({
  imports: [forwardRef(function ref() { return SharedModule; })],
  controllers: [UserController],
})
export class UserModule {}
`,
	})

	findings := scanProject(t, root)
	if len(findings) != 1 {
		t.Fatalf("findings len = %d, want 1: %#v", len(findings), findings)
	}
	if findings[0].File != "user.controller.ts" || findings[0].Line != 7 {
		t.Fatalf("location = %s:%d, want user.controller.ts:7", findings[0].File, findings[0].Line)
	}
}

// An ambiguous forwardRef body (here a ternary) is not a bare module
// identifier, so the reader cannot know which module is imported. It must bail
// and mark imports unknown rather than guess a name — guessing would let a
// stale match suppress a real finding, or a wrong match invent one. With the
// import unresolved the module is treated as unknown, so no finding is emitted
// (conservative: never a false positive on code we cannot read).
func TestForwardRefAmbiguousBodyBailsToUnknown(t *testing.T) {
	root := writeProject(t, map[string]string{
		"shared/user.service.ts": `
import { Injectable } from "@nestjs/common";

@Injectable()
export class UserService {}
`,
		"shared/shared.module.ts": `
import { Module } from "@nestjs/common";
import { UserService } from "./user.service";

@Module({
  providers: [UserService],
})
export class SharedModule {}
`,
		"user.controller.ts": `
import { Controller } from "@nestjs/common";
import { UserService } from "./shared/user.service";

@Controller()
export class UserController {
  constructor(private readonly users: UserService) {}
}
`,
		"user.module.ts": `
import { Module, forwardRef } from "@nestjs/common";
import { UserController } from "./user.controller";
import { SharedModule } from "./shared/shared.module";
import { OtherModule } from "./other.module";

@Module({
  imports: [forwardRef(() => true ? SharedModule : OtherModule)],
  controllers: [UserController],
})
export class UserModule {}
`,
		"other.module.ts": `
import { Module } from "@nestjs/common";

@Module({})
export class OtherModule {}
`,
	})

	if findings := scanProject(t, root); len(findings) != 0 {
		t.Fatalf("ambiguous forwardRef must mark imports unknown and suppress findings; got %d: %#v", len(findings), findings)
	}
}

func TestImportedButNotExportedProviderIsReported(t *testing.T) {
	root := writeProject(t, map[string]string{
		"shared/user.service.ts": `
import { Injectable } from "@nestjs/common";

@Injectable()
export class UserService {}
`,
		"shared/shared.module.ts": `
import { Module } from "@nestjs/common";
import { UserService } from "./user.service";

@Module({
  providers: [UserService],
})
export class SharedModule {}
`,
		"user.controller.ts": `
import { Controller } from "@nestjs/common";
import { UserService } from "./shared/user.service";

@Controller()
export class UserController {
  constructor(private readonly users: UserService) {}
}
`,
		"user.module.ts": `
import { Module } from "@nestjs/common";
import { UserController } from "./user.controller";
import { SharedModule } from "./shared/shared.module";

@Module({
  imports: [SharedModule],
  controllers: [UserController],
})
export class UserModule {}
`,
	})

	findings := scanProject(t, root)
	if len(findings) != 1 {
		t.Fatalf("findings len = %d, want 1: %#v", len(findings), findings)
	}
	if findings[0].File != "user.controller.ts" || findings[0].Line != 7 {
		t.Fatalf("location = %s:%d, want user.controller.ts:7", findings[0].File, findings[0].Line)
	}
}

func TestCustomInjectTokenIsSkipped(t *testing.T) {
	root := writeProject(t, map[string]string{
		"user.controller.ts": `
import { Controller, Inject } from "@nestjs/common";

@Controller()
export class UserController {
  constructor(@Inject("USER_REPO") private readonly repo: UserRepo) {}
}
`,
		"user.module.ts": `
import { Module } from "@nestjs/common";
import { UserController } from "./user.controller";

@Module({
  controllers: [UserController],
})
export class UserModule {}
`,
	})

	if findings := scanProject(t, root); len(findings) != 0 {
		t.Fatalf("findings len = %d, want 0: %#v", len(findings), findings)
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

func scanProject(t *testing.T, root string) []core.Finding {
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
	got, err := New().Scan(core.ProjectContext{Root: root, Files: files, Languages: []string{"javascript"}})
	if err != nil {
		t.Fatal(err)
	}
	return got
}
