package validationcoverage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aykutssert/inspector/internal/core"
)

func TestValidationCoverage(t *testing.T) {
	src := `
@Controller("users")
export class UsersController {
  @Post()
  async create(@Body() body: any) {
    return body;
  }

  @Put()
  @UsePipes(ValidationPipe)
  async update(@Body() body: any) {
    return body;
  }
}

@Controller("items")
@UsePipes(ValidationPipe)
export class ItemsController {
  @Post()
  async create(@Body() body: any) {
    return body;
  }
}
`

	tempDir := t.TempDir()
	file := filepath.Join(tempDir, "controller.ts")
	if err := os.WriteFile(file, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	a := New()
	ctx := core.ProjectContext{
		Root:  tempDir,
		Files: []string{"controller.ts"},
	}

	findings, err := a.Scan(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Only UsersController.create should be flagged
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(findings), findings)
	}

	f := findings[0]
	if f.RuleID != "nestjs.nestjs-missing-validation-pipe" {
		t.Errorf("expected rule ID nestjs.nestjs-missing-validation-pipe, got %s", f.RuleID)
	}

	if f.Line != 5 {
		t.Errorf("expected violation at line 5 (UsersController.create), got line %d", f.Line)
	}
}
