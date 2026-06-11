package jscontext

import (
	"os"
	"path/filepath"
	"testing"

	inspectctx "github.com/aykutssert/scout/internal/context"
)

func TestJSParserParsesJSFile(t *testing.T) {
	dir := t.TempDir()
	path := "hello.ts"
	content := `
import { useState } from "react";
export function hello(name: string) { return "hi " + name; }
`
	if err := os.WriteFile(filepath.Join(dir, path), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	p := JSParser{}
	fp, err := p.Parse(dir, path)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if fp == nil {
		t.Fatal("Parse returned nil")
	}
	if fp.Path != path {
		t.Fatalf("want path %q, got %q", path, fp.Path)
	}
	if len(fp.Imports) != 1 {
		t.Fatalf("expected 1 import, got %d", len(fp.Imports))
	}
	if len(fp.Defs) != 1 {
		t.Fatalf("expected 1 def, got %d", len(fp.Defs))
	}
	if fp.Defs[0].Name != "hello" {
		t.Fatalf("want def name hello, got %q", fp.Defs[0].Name)
	}
	if !fp.Defs[0].Exported {
		t.Fatal("expected exported def")
	}
}

func TestJSParserNormalizesImportEdges(t *testing.T) {
	dir := t.TempDir()
	writeParserFixture(t, dir, "a.ts", `import { value } from "./b"; import React from "react";`)
	writeParserFixture(t, dir, "b.ts", `export const value = 1;`)

	fp, err := (JSParser{}).Parse(dir, "a.ts")
	if err != nil {
		t.Fatal(err)
	}
	if fp.Imports[0].Target != "b.ts" {
		t.Fatalf("relative import target = %q, want b.ts", fp.Imports[0].Target)
	}
	if fp.Imports[1].Package != "react" {
		t.Fatalf("external package = %q, want react", fp.Imports[1].Package)
	}
}

func TestJSParserNormalizesScopedPackageAndAlias(t *testing.T) {
	dir := t.TempDir()
	writeParserFixture(t, dir, "tsconfig.json", `{"compilerOptions":{"paths":{"@app/*":["src/*"]}}}`)
	writeParserFixture(t, dir, "a.ts", `import { Controller } from "@nestjs/common/decorators"; import { helper } from "@app/helper";`)

	fp, err := (JSParser{}).Parse(dir, "a.ts")
	if err != nil {
		t.Fatal(err)
	}
	if fp.Imports[0].Package != "@nestjs/common" {
		t.Fatalf("scoped package = %q, want @nestjs/common", fp.Imports[0].Package)
	}
	if fp.Imports[1].Package != "" {
		t.Fatalf("tsconfig alias leaked as package: %q", fp.Imports[1].Package)
	}
}

func TestJSParserFrameworkEntriesAndRoles(t *testing.T) {
	tests := []struct {
		name      string
		pkg       string
		files     map[string]string
		wantFrame string
		wantEntry string
		wantRole  string
	}{
		{
			name:      "nextjs",
			pkg:       `{"dependencies":{"next":"16.0.0"}}`,
			files:     map[string]string{"app/page.tsx": `export default function Page() { return null; }`},
			wantFrame: "nextjs",
			wantEntry: "app/page.tsx",
			wantRole:  "page",
		},
		{
			name:      "express",
			pkg:       `{"dependencies":{"express":"5.0.0"}}`,
			files:     map[string]string{"server.ts": `import express from "express"; const app = express(); app.listen(3000);`},
			wantFrame: "express",
			wantEntry: "server.ts",
			wantRole:  "server",
		},
		{
			name:      "nestjs",
			pkg:       `{"dependencies":{"@nestjs/core":"11.0.0"}}`,
			files:     map[string]string{"src/main.ts": `import { NestFactory } from "@nestjs/core"; NestFactory.create(AppModule);`},
			wantFrame: "nestjs",
			wantEntry: "src/main.ts",
			wantRole:  "bootstrap",
		},
		{
			name:      "vite",
			pkg:       `{"devDependencies":{"vite":"7.0.0"}}`,
			files:     map[string]string{"index.html": `<div id="app"></div>`, "src/main.ts": `createApp(App).mount("#app");`},
			wantFrame: "vite",
			wantEntry: "src/main.ts",
			wantRole:  "app-entry",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			writeParserFixture(t, root, "package.json", test.pkg)
			var sources []string
			for path, body := range test.files {
				writeParserFixture(t, root, path, body)
				if isJavaScriptPath(path) {
					sources = append(sources, path)
				}
			}
			repo, err := inspectctx.BuildRepoMap(root, sources, JSParser{})
			if err != nil {
				t.Fatal(err)
			}
			if repo.Framework != test.wantFrame {
				t.Fatalf("framework = %q, want %q", repo.Framework, test.wantFrame)
			}
			if !containsString(repo.EntryPoints, test.wantEntry) {
				t.Fatalf("entry points = %v, want %q", repo.EntryPoints, test.wantEntry)
			}
			if role := repoRole(repo, test.wantEntry); role != test.wantRole {
				t.Fatalf("role = %q, want %q", role, test.wantRole)
			}
		})
	}
}

func writeParserFixture(t *testing.T, root, path, body string) {
	t.Helper()
	absolute := filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(absolute, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func repoRole(repo inspectctx.RepoMap, path string) string {
	for _, dir := range repo.Dirs {
		for _, file := range dir.Files {
			if file.Path == path {
				return file.Role
			}
		}
	}
	return ""
}
