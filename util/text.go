package util

import "strings"

// FirstLine 返回多行文本的首行（去掉首尾空白）；若首行内含换行则截断到第一个 \r 或 \n 之前。
func FirstLine(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	return s
}

// SanitizeFilenameSegment 将任意字符串整理为适合作为文件名片段的片段（非法字符替换为 -，过长截断）。
func SanitizeFilenameSegment(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "no_msg"
	}
	var b strings.Builder
	lastUnderscore := false
	for _, r := range s {
		switch {
		case r < 32 || strings.ContainsRune(`<>:"/\|?*`, r):
			if !lastUnderscore {
				b.WriteByte('-')
				lastUnderscore = true
			}
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			if !lastUnderscore {
				b.WriteByte('-')
				lastUnderscore = true
			}
		default:
			b.WriteRune(r)
			lastUnderscore = false
		}
	}
	out := strings.Trim(b.String(), " ._")
	if out == "" {
		out = "no_msg"
	}
	runes := []rune(out)
	if len(runes) > 60 {
		out = string(runes[:60])
	}
	return out
}
