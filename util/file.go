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

// ClearOrEmptyFile 删除当日结果文件（若存在），保证输出目录存在。
// 若只截断为 0 字节，部分 agent 会认为「文件已存在」而不写入；删除后与首次运行一致（路径不存在），写入更可靠。
func ClearOrEmptyFile(path string) error {
	if err := ensureParentDirForFile(path); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ResultFileIsEmpty 结果文件不存在或去空白后为空时返回 true。
func ResultFileIsEmpty(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return true
	}
	return strings.TrimSpace(string(data)) == ""
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
