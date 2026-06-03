package scan

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aykutssert/inspector/internal/core"
)

var skipDirs = map[string]bool{
	"node_modules": true, ".git": true, "vendor": true,
	"dist": true, "build": true, ".next": true, "out": true,
}

func Discover(root string, diffOnly bool, adapters []core.LanguageAdapter) ([]string, error) {
	var candidates []string
	var err error
	if diffOnly {
		candidates, err = changedFiles(root)
	} else {
		candidates, err = walk(root)
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, f := range candidates {
		for _, a := range adapters {
			if a.Matches(f) {
				out = append(out, f)
				break
			}
		}
	}
	return out, nil
}

func walk(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		files = append(files, rel)
		return nil
	})
	return files, err
}

// Changed returns the raw list of git-changed paths (unfiltered by language and
// including deletions), for analyzers that key off non-source files like
// dependency manifests — e.g. a deleted package-lock.json must still trigger a
// dependency scan.
func Changed(root string) ([]string, error) {
	return parseChanged(root, true)
}

// changedFiles returns paths suitable as scanner input: deletions are dropped
// because the path no longer exists and handing it to a file scanner errors.
func changedFiles(root string) ([]string, error) {
	return parseChanged(root, false)
}

func parseChanged(root string, includeDeletions bool) ([]string, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		if len(line) < 4 {
			continue
		}
		// porcelain status is "XY path": X=staged, Y=worktree.
		if (line[0] == 'D' || line[1] == 'D') && !includeDeletions {
			continue
		}
		// "XY path" or "XY old -> new" (rename); keep the new path.
		path := strings.TrimSpace(line[3:])
		if idx := strings.Index(path, " -> "); idx >= 0 {
			path = path[idx+4:]
		}
		files = append(files, path)
	}
	return files, nil
}
