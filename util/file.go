package util

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

func ensureParentDirForFile(filePath string) error {
	dir := filepath.Dir(filePath)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0755)
}

func ClearOrEmptyFile(path string) error {
	if err := ensureParentDirForFile(path); err != nil {
		return err
	}
	return os.WriteFile(path, nil, 0644)
}

func OpenAppendFile(path string) (*os.File, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	if err := ensureParentDirForFile(path); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
}

func WriteContentToFile(content, fileName string) {
	if err := ensureParentDirForFile(fileName); err != nil {
		log.Printf("failed to makedir, err:%v", err)
		return
	}
	f, err := os.Create(fileName)
	if err != nil {
		log.Printf("failed to create file, err:%v", err)
		return
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		log.Printf("failed to write to file, err:%v", err)
	}
}

func ClearDir(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return nil
}
