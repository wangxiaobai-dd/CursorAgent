package util

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	reIndexLine = regexp.MustCompile(`^Index:\s+(.+)$`)
	reDiffLine  = regexp.MustCompile(`^(?:---|\+\+\+)\s+(?:[ab]/)?(.+?)(\s+\(.*\))?$`)
)

func ParseDiffFilePaths(diffContent string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, line := range strings.Split(diffContent, "\n") {
		trimmed := strings.TrimSpace(line)
		if m := reIndexLine.FindStringSubmatch(trimmed); len(m) > 1 {
			p := strings.TrimSpace(m[1])
			if _, ok := seen[p]; !ok {
				seen[p] = struct{}{}
				out = append(out, p)
			}
			continue
		}
		if m := reDiffLine.FindStringSubmatch(trimmed); len(m) > 1 {
			filePath := strings.TrimSpace(m[1])
			if filePath == "/dev/null" || strings.Contains(filePath, "nonexistent") {
				continue
			}
			if _, ok := seen[filePath]; !ok {
				seen[filePath] = struct{}{}
				out = append(out, filePath)
			}
		}
	}
	return out
}

// ParseRevisionFromDiffFilename 解析 diff 文件名（无扩展名）中的 revision 与提交说明。
func ParseRevisionFromDiffFilename(baseNoExt string) (revision, submitMsg string) {
	idx := strings.Index(baseNoExt, "_")
	if idx <= 0 {
		return baseNoExt, ""
	}
	return baseNoExt[:idx], baseNoExt[idx+1:]
}

// BuildSourceRefsFromDiff 读取 diff 并解析路径，在 svnWorkingDir 下存在则生成 @绝对路径 引用块。
func BuildSourceRefsFromDiff(diffPath, svnWorkingDir string) (string, error) {
	content, err := ReadFileStringAuto(diffPath)
	if err != nil {
		return "", err
	}
	relPaths := ParseDiffFilePaths(content)
	var refs []string
	for _, rel := range relPaths {
		abs := filepath.Join(svnWorkingDir, filepath.FromSlash(rel))
		if _, err := os.Stat(abs); err == nil {
			refs = append(refs, "@"+abs)
		}
	}
	if len(refs) == 0 {
		return "", nil
	}
	return "\n\n如需查看完整源文件，可参考以下文件：\n" + strings.Join(refs, "\n"), nil
}
