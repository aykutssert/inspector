package policycoverage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aykutssert/inspector/internal/core"
)

func TestPolicyCoverageAnalyzer(t *testing.T) {
	tmp, err := os.MkdirTemp("", "inspector-policycoverage-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	// Write NestJS Controller fixture
	nestjsSrc := `
import { Controller, Get, Post, UseGuards } from '@nestjs/common';
import { JwtAuthGuard } from './jwt-auth.guard';
import { Public } from './public.decorator';

@Controller('users')
@UseGuards(JwtAuthGuard)
export class UsersController {
  @Get('profile')
  getProfile() {}

  @Public()
  @Post('login')
  login() {}
}

@Controller('admin')
export class AdminController {
  @UseGuards(JwtAuthGuard)
  @Get('stats')
  stats() {}

  @Post('delete')
  deleteUser() {}
}
`
	nestjsFile := "users.controller.ts"
	if err := os.WriteFile(filepath.Join(tmp, nestjsFile), []byte(nestjsSrc), 0o644); err != nil {
		t.Fatal(err)
	}

	// Write Express Router fixture
	expressSrc := `
const express = require('express');
const router = express.Router();
const { auth } = require('./middleware');

router.get('/profile', auth, getProfile);
router.post('/posts', passport.authenticate('jwt'), createPost);
router.delete('/delete', deleteHandler);
`
	expressFile := "routes.js"
	if err := os.WriteFile(filepath.Join(tmp, expressFile), []byte(expressSrc), 0o644); err != nil {
		t.Fatal(err)
	}

	analyzer := New()
	ctx := core.ProjectContext{
		Root:  tmp,
		Files: []string{nestjsFile, expressFile},
	}

	findings, err := analyzer.Scan(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Expected findings:
	// 1. NestJS: AdminController deleteUser is missing AuthGuard or @Public
	// 2. Express: router.delete('/delete') is missing auth middleware
	wantFindings := 2
	if len(findings) != wantFindings {
		t.Fatalf("want %d findings, got %d: %+v", wantFindings, len(findings), findings)
	}

	var foundNest, foundExpress bool
	for _, f := range findings {
		if f.RuleID == "nestjs.missing-auth-guard" {
			if f.File != nestjsFile {
				t.Errorf("nestjs finding in wrong file: %s", f.File)
			}
			if !strings.Contains(f.Message, "NestJS") {
				t.Errorf("wrong message for nestjs: %s", f.Message)
			}
			foundNest = true
		} else if f.RuleID == "express.missing-auth-middleware" {
			if f.File != expressFile {
				t.Errorf("express finding in wrong file: %s", f.File)
			}
			if !strings.Contains(f.Message, "Express") {
				t.Errorf("wrong message for express: %s", f.Message)
			}
			foundExpress = true
		}
	}

	if !foundNest {
		t.Error("missing nestjs finding")
	}
	if !foundExpress {
		t.Error("missing express finding")
	}
}
