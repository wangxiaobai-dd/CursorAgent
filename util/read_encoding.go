package util

import (
	"os"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// ReadFileStringAuto 尝试按 UTF-8 读取；非法 UTF-8 时按 GBK 解码（与常见 SVN/Windows 中文 diff 兼容）。
func ReadFileStringAuto(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if utf8.Valid(b) {
		return string(b), nil
	}
	out, err := simplifiedchinese.GBK.NewDecoder().Bytes(b)
	if err != nil {
		return string(b), nil
	}
	return string(out), nil
}
