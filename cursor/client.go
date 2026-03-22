package cursor

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
)

// Client 用于调用 Cursor CLI（例如 agent）执行技能。
type Client struct {
	AgentCommand string
}

var ErrAgentCommandUnavailable = errors.New("agent command unavailable")

// resolveAgentCommand 解析 Cursor CLI 可执行命令。若配置的命令不在 PATH 中则返回空字符串，不做回退。
func resolveAgentCommand(agentCommand string) string {
	if agentCommand == "" {
		return ""
	}
	if path, err := exec.LookPath(agentCommand); err == nil {
		return path
	}
	log.Printf("cursor: 未找到可执行命令 %q，终止操作", agentCommand)
	return ""
}

func NewClient(agentCommand string) *Client {
	return &Client{AgentCommand: resolveAgentCommand(agentCommand)}
}

// RunSkill 运行一个技能：若 filePath 非空，会在提示中引用 @filePath。
// workDir 会作为 agent 的 cwd 与 --workspace；svn_diff 等任务应传「输出目录」而非 diff 目录，避免工具写文件落到错误路径。
// 若 AgentCommand 不可用，返回错误并终止本次调用。
func (c *Client) RunSkill(skillName, workDir, filePath, promptOnly string) (string, error) {
	if c.AgentCommand == "" {
		return "", fmt.Errorf("%w: empty or not found in PATH", ErrAgentCommandUnavailable)
	}
	absWorkDir, _ := filepath.Abs(workDir)
	var prompt string
	if promptOnly != "" {
		prompt = promptOnly
	} else if filePath != "" {
		absFile := filePath
		if !filepath.IsAbs(filePath) {
			absFile, _ = filepath.Abs(filepath.Join(workDir, filepath.Base(filePath)))
		}
		if skillName != "" {
			prompt = "请使用 skill " + skillName + " 检查 @" + absFile + "，只输出结果，不要其他操作。"
		} else {
			prompt = "请检查 @" + absFile + "，只输出结果，不要其他操作。"
		}
	} else {
		return "", nil
	}
	// 非交互运行需声明信任工作区，否则会阻塞在 Workspace Trust 提示并失败。
	// --trust 仅在与 -p/--print 一起使用时生效；须放在提示词参数之前，避免长 -p 内容导致解析异常。
	// --workspace 显式指定与 cmd.Dir 一致，避免 agent 与 cwd 不一致时仍弹信任框。
	args := []string{
		"--trust",
		"--workspace", absWorkDir,
		"--output-format", "text",
		"-p", prompt,
	}
	cmd := exec.Command(c.AgentCommand, args...)
	cmd.Dir = absWorkDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	log.Printf("[CursorAgent] Run: %s %s", c.AgentCommand, strings.Join(args, " "))
	if err := cmd.Run(); err != nil {
		if stderr.Len() > 0 {
			log.Printf("cursor stderr: %s", stderr.String())
		}
		return "", err
	}
	return stdout.String(), nil
}

// LaunchWithExtension 唤起 Cursor IDE 并加载指定插件。
// 参数：cursorExe 为 cursor.cmd 完整路径，workspace 为打开的工作区目录。
// extraEnv 会追加到子进程环境（如 CURSOR_AGENT_TASK_NAME=xxx），可为 nil。
// 返回的 cmd 由调用方 Start() 后 Wait()。
func LaunchWithExtension(cursorExe string) *exec.Cmd {
	args := []string{}
	cmd := exec.Command(cursorExe, args...)
	log.Printf("cursor: %s %s", cursorExe, strings.Join(args, " "))
	return cmd
}
