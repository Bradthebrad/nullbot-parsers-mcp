package parsers

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// within reports whether path is root or inside it. filepath.Rel compares
// case-insensitively on Windows, where C:\Work and c:\work are one folder.
func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil || filepath.IsAbs(rel) {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// confine turns a tool's path argument into an absolute path inside root.
// A relative path is taken from root; an absolute one must already be inside
// it. The deepest part of the path that exists is resolved through links, so
// a link inside the workspace can't lead outside it.
func confine(root, path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("path is required")
	}
	path = filepath.FromSlash(path)
	full := filepath.Clean(path)
	if !filepath.IsAbs(full) {
		if filepath.VolumeName(full) != "" {
			return "", fmt.Errorf("%s names a drive but not a full path; use a path inside %s", path, root)
		}
		full = filepath.Join(root, full)
	}
	if !within(root, full) {
		return "", fmt.Errorf("%s is outside the workspace %s; use a path inside it, relative to it or absolute", path, root)
	}
	existing, rest := full, []string{}
	for {
		if _, err := os.Lstat(existing); err == nil {
			break
		}
		parent := filepath.Dir(existing)
		if parent == existing {
			break
		}
		rest = append([]string{filepath.Base(existing)}, rest...)
		existing = parent
	}
	resolved, err := filepath.EvalSymlinks(existing)
	if err != nil {
		return "", fmt.Errorf("resolve %s: %w", path, err)
	}
	if !within(root, resolved) {
		return "", fmt.Errorf("%s leads outside the workspace %s through a link", path, root)
	}
	return filepath.Join(append([]string{resolved}, rest...)...), nil
}
