package parsers

import (
	"fmt"
	"os"
	"path/filepath"
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

// resolve turns a tool's path argument into a path inside the workspace,
// relative to it or absolute.
func (p *ParserTools) resolve(rel string) (string, error) {
	return confine(p.root, rel)
}

func (p *ParserTools) rel(path string) string {
	rel, err := filepath.Rel(p.root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(rel)
}
