package job

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"CursorAgent/cursor"
	"CursorAgent/input/code"
	"CursorAgent/input/types"
	"CursorAgent/util"
)

type taskRunner interface {
	run(items []types.InputItem) error
}

func newTaskRunner(j *Job) (taskRunner, error) {
	if j.Task.LaunchCursor {
		return &launchCursorRunner{j: j}, nil
	}
	switch j.Task.Input.Type {
	case "svn_diff":
		return &svnDiffRunner{j: j}, nil
	case "file", "prompt":
		return &genericAgentRunner{j: j}, nil
	default:
		return nil, fmt.Errorf("unsupported Input.Type for runner: %q", j.Task.Input.Type)
	}
}

// --- LaunchCursor ---
type launchCursorRunner struct {
	j *Job
}

func (r *launchCursorRunner) run(items []types.InputItem) error {
	_ = items
	if r.j.Opt == nil {
		log.Printf("job %s: LaunchCursor but Opt is nil", r.j.Task.Name)
		return fmt.Errorf("opt is nil")
	}
	if r.j.PluginListener == nil {
		log.Printf("job %s: LaunchCursor but PluginListener is nil", r.j.Task.Name)
		return fmt.Errorf("plugin listener is nil")
	}
	cmd := cursor.LaunchWithExtension(r.j.Opt.CursorExePath)
	if err := cmd.Start(); err != nil {
		log.Printf("job %s launch cursor failed: %v", r.j.Task.Name, err)
		return err
	}
	if err := cmd.Wait(); err != nil {
		log.Printf("job %s cursor exited with error: %v", r.j.Task.Name, err)
		return err
	}
	log.Printf("job %s: completed (LaunchCursor)", r.j.Task.Name)
	return nil
}

// --- svn_diff ---
type svnDiffRunner struct {
	j *Job
}

func (r *svnDiffRunner) run(items []types.InputItem) error {
	j := r.j
	jt := j.Task
	taskOut := jt.Output
	svnSrc, ok := j.Source.(*code.SvnDiffSource)
	if !ok {
		return fmt.Errorf("svn_diff task must use SvnDiffSource")
	}
	if err := os.MkdirAll(taskOut.OutDir, 0755); err != nil {
		log.Printf("job %s mkdir OutDir: %v", jt.Name, err)
		return err
	}
	date := time.Now().Format("2006-01-02")
	resultFileAbs, _ := filepath.Abs(taskOut.ResultFilePath(date))

	// 你的 OutDir 本身就是绝对路径：直接用它作为 agent workspace 即可。
	agentWorkspaceAbs, werr := filepath.Abs(taskOut.OutDir)
	if werr != nil {
		log.Printf("job %s: abs OutDir: %v", jt.Name, werr)
		agentWorkspaceAbs = taskOut.OutDir
	}

	sort.SliceStable(items, func(i, j int) bool {
		ri, e1 := strconv.Atoi(items[i].Meta["revision"])
		rj, e2 := strconv.Atoi(items[j].Meta["revision"])
		if e1 == nil && e2 == nil {
			return ri < rj
		}
		return items[i].Meta["revision"] < items[j].Meta["revision"]
	})

	for idx, item := range items {
		absDiff, _ := filepath.Abs(item.Path)
		header := svnSrc.RevisionHeaderForItem(item)

		// 可选：引入与 diff 相关的源文件引用，供 skill 读取上下文。
		var sourceRefs string
		if jt.Input.IncludeSvnSource && strings.TrimSpace(jt.Input.ProjectPath) != "" {
			if refs, err := util.BuildSourceRefsFromDiff(absDiff, jt.Input.ProjectPath); err != nil {
				log.Printf("job %s BuildSourceRefsFromDiff %s: %v", jt.Name, absDiff, err)
			} else {
				sourceRefs = refs
			}
		}

		prompt := "请使用 skill 文件 " + jt.Skill + " 检查 @" + absDiff + "。"
		prompt += "只输出审查结果，不要输出其它内容。"
		prompt += "\n输出必须严格按以下顺序与标记：\n"
		prompt += "[文件名]\n"
		prompt += "[问题代码]\n"
		prompt += "[修改建议]\n"
		prompt += "并在输出末尾单独写入一行：=== END ==="
		if strings.TrimSpace(sourceRefs) != "" {
			prompt += "\n" + sourceRefs
		}

		stdout, err := j.Cursor.RunSkill("", agentWorkspaceAbs, "", prompt)
		if err != nil {
			log.Printf("job %s cursor failed (diff %d): %v", jt.Name, idx, err)
			if errors.Is(err, cursor.ErrAgentCommandUnavailable) {
				return err
			}
			return err
		}

		body := stdout
		if strings.TrimSpace(body) == "" {
			log.Printf("job %s: empty skill output (diff %d): %s", jt.Name, idx, absDiff)
			continue
		}
		if !strings.Contains(body, "=== END ===") {
			body += "\n=== END ==="
		}

		f, err := util.OpenAppendFile(resultFileAbs)
		if err != nil {
			return err
		}
		if f == nil {
			return fmt.Errorf("open append result file failed: %s", resultFileAbs)
		}
		if idx > 0 {
			_, _ = f.WriteString("\n")
		}
		_, _ = f.WriteString(header + "\n")
		_, _ = f.WriteString(body + "\n")
		_ = f.Close()
	}

	if util.ResultFileIsEmpty(resultFileAbs) {
		return fmt.Errorf("result file is empty: %s", resultFileAbs)
	}
	if jt.Output.UploadURL != "" {
		if upErr := util.UploadFile(jt.Output.UploadURL, resultFileAbs); upErr != nil {
			log.Printf("job %s upload result file failed: %v", jt.Name, upErr)
		}
	}
	return nil
}

// --- file / prompt ---
type genericAgentRunner struct {
	j *Job
}

func (r *genericAgentRunner) run(items []types.InputItem) error {
	j := r.j
	jt := j.Task
	workDir := j.Source.WorkDir()
	for _, item := range items {
		var err error
		if item.Prompt != "" {
			_, err = j.Cursor.RunSkill(jt.Skill, workDir, "", item.Prompt)
		} else {
			_, err = j.Cursor.RunSkill(jt.Skill, workDir, item.Path, "")
		}
		if err != nil {
			log.Printf("job %s cursor failed for item %s: %v", jt.Name, item.Path, err)
			if errors.Is(err, cursor.ErrAgentCommandUnavailable) {
				return err
			}
			continue
		}
	}
	return nil
}
