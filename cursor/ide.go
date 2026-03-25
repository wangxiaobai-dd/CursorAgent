package cursor

import (
	"log"
	"os/exec"
	"strings"
)

// LaunchWithExtension 唤起 Cursor IDE 并加载指定插件。
// 参数：cursorExe 为 cursor.cmd 完整路径；返回的 cmd 由调用方 Start() 后 Wait()。
func LaunchWithExtension(cursorExe string) *exec.Cmd {
	args := []string{}
	cmd := exec.Command(cursorExe, args...)
	log.Printf("cursor: %s %s", cursorExe, strings.Join(args, " "))
	return cmd
}
