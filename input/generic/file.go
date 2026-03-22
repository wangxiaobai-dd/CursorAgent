package generic

import (
	"os"
	"path/filepath"

	"CursorAgent/config"
	"CursorAgent/input/types"
)

type FileSource struct {
	paths   []string
	workDir string
}

func NewFileSource(opt *config.TaskInput) *FileSource {
	paths := opt.Paths
	workDir := "."
	if len(paths) > 0 {
		if abs, err := filepath.Abs(paths[0]); err == nil {
			workDir = filepath.Dir(abs)
		}
	}
	return &FileSource{paths: paths, workDir: workDir}
}

func (p *FileSource) WorkDir() string { return p.workDir }

func (p *FileSource) GetInputs() ([]types.InputItem, error) {
	var items []types.InputItem
	for _, path := range p.paths {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		items = append(items, types.InputItem{Path: path, Meta: map[string]string{"file": path}})
	}
	return items, nil
}
