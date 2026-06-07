package parsers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Workspace string
	MaxBytes  int64
}

type ParserTools struct {
	root     string
	maxBytes int64
}

func New(config Config) (*ParserTools, error) {
	workspace := config.Workspace
	if workspace == "" {
		workspace = "."
	}
	root, err := filepath.Abs(workspace)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("workspace is not a directory: %s", root)
	}
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	maxBytes := config.MaxBytes
	if maxBytes <= 0 {
		maxBytes = 1024 * 1024
	}
	return &ParserTools{root: root, maxBytes: maxBytes}, nil
}

func (p *ParserTools) resolve(rel string) (string, error) {
	if rel == "" {
		return "", fmt.Errorf("path is required")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("absolute paths are not allowed: %s", rel)
	}
	full := filepath.Join(p.root, filepath.Clean(rel))
	if err := p.ensureInside(full); err != nil {
		return "", err
	}
	return full, nil
}

func (p *ParserTools) ensureInside(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if abs == p.root {
		return nil
	}
	if !strings.HasPrefix(abs, p.root+string(os.PathSeparator)) {
		return fmt.Errorf("path escapes workspace: %s", path)
	}
	return nil
}

func (p *ParserTools) rel(path string) string {
	rel, err := filepath.Rel(p.root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}
