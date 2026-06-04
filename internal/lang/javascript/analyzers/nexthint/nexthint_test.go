package nexthint

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/aykutssert/inspector/internal/core"
)

func TestFlagsDirectServerFileImportFromClientComponent(t *testing.T) {
	findings := scanProject(t, map[string]string{
		"app/client.tsx": `"use client";

import { getUser } from "../lib/auth.server";

export function Client() {
  return <button onClick={() => getUser()}>Load</button>;
}
`,
		"lib/auth.server.ts": `export function getUser() { return null; }`,
	})

	f := oneFinding(t, findings)
	if f.RuleID != "next.local-server-boundary" {
		t.Fatalf("rule = %q", f.RuleID)
	}
	if f.File != "app/client.tsx" || f.Line != 3 {
		t.Fatalf("location = %s:%d, want app/client.tsx:3", f.File, f.Line)
	}
	if f.Confidence != core.ConfidenceRule {
		t.Fatalf("confidence = %q, want rule", f.Confidence)
	}
	if !strings.Contains(f.Message, "lib/auth.server.ts") {
		t.Fatalf("message should name reached server file, got %q", f.Message)
	}
}

func TestFlagsTransitiveServerFileThroughBarrel(t *testing.T) {
	findings := scanProject(t, map[string]string{
		"app/client.tsx": `"use client";

import { getUser } from "../lib";

export function Client() {
  return <button onClick={() => getUser()}>Load</button>;
}
`,
		"lib/index.ts":       `export { getUser } from "./auth.server";`,
		"lib/auth.server.ts": `export function getUser() { return null; }`,
	})

	f := oneFinding(t, findings)
	if f.File != "app/client.tsx" || f.Line != 3 {
		t.Fatalf("location = %s:%d, want app/client.tsx:3", f.File, f.Line)
	}
	if !strings.Contains(f.Message, "app/client.tsx -> lib/index.ts -> lib/auth.server.ts") {
		t.Fatalf("message should include import chain, got %q", f.Message)
	}
}

func TestFlagsLocalDatabaseModuleThatImportsServerOnlyPackage(t *testing.T) {
	findings := scanProject(t, map[string]string{
		"app/client.tsx": `"use client";
import { db } from "../lib/db";
export function Client() { return null; }
`,
		"lib/db.ts": `import { PrismaClient } from "@prisma/client";
export const db = new PrismaClient();
`,
	})

	f := oneFinding(t, findings)
	if f.File != "app/client.tsx" || f.Line != 2 {
		t.Fatalf("location = %s:%d, want app/client.tsx:2", f.File, f.Line)
	}
	if !strings.Contains(f.Message, "imports server-only package @prisma/client") {
		t.Fatalf("message should include package reason, got %q", f.Message)
	}
}

func TestFlagsLocalORMServiceThatImportsServerOnlyPackage(t *testing.T) {
	findings := scanProject(t, map[string]string{
		"app/client.tsx": `"use client";
import { repo } from "../lib/user.repository";
export function Client() { return null; }
`,
		"lib/user.repository.ts": `import { DataSource } from "typeorm";
export const repo = new DataSource({});
`,
	})

	f := oneFinding(t, findings)
	if f.File != "app/client.tsx" || f.Line != 2 {
		t.Fatalf("location = %s:%d, want app/client.tsx:2", f.File, f.Line)
	}
	if !strings.Contains(f.Message, "imports server-only package typeorm") {
		t.Fatalf("message should include package reason, got %q", f.Message)
	}
}

func TestSafeClientImportAndServerComponentImportAreIgnored(t *testing.T) {
	findings := scanProject(t, map[string]string{
		"app/client.tsx": `"use client";
import { formatName } from "../lib/format";
export function Client() { return formatName("a"); }
`,
		"app/server.tsx": `import { getUser } from "../lib/auth.server";
export default async function Page() { return getUser(); }
`,
		"lib/format.ts":      `export function formatName(v: string) { return v.toUpperCase(); }`,
		"lib/auth.server.ts": `export function getUser() { return null; }`,
	})

	if len(findings) != 0 {
		t.Fatalf("got findings for safe/non-client imports: %+v", findings)
	}
}

func TestDatabaseNamedModuleWithoutServerSignalIsIgnored(t *testing.T) {
	findings := scanProject(t, map[string]string{
		"app/client.tsx": `"use client";
import { db } from "../lib/db";
export function Client() { return db; }
`,
		"lib/db.ts": `export const db = {};`,
	})

	if len(findings) != 0 {
		t.Fatalf("got findings for name-only db module: %+v", findings)
	}
}

func TestUseClientDirectiveMayFollowCommentsAndUseStrict(t *testing.T) {
	root := writeProject(t, map[string]string{
		"app/client.tsx": `// generated header
"use strict";
'use client';
import { secret } from "../lib/secret.server";
export function Client() { return secret; }
`,
		"lib/secret.server.ts": `export const secret = 1;`,
	})
	got, err := New().Scan(core.ProjectContext{Root: root, Files: projectFiles(t, root), Languages: []string{"javascript"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("findings = %+v, want one", got)
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
