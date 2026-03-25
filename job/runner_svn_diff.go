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

	"CursorAgent/config"
	"CursorAgent/cursor"
	"CursorAgent/input/code"
	"CursorAgent/input/types"
	"CursorAgent/util"
)

func sortInputItemsByRevision(items []types.InputItem) {
	sort.SliceStable(items, func(i, j int) bool {
		ri, e1 := strconv.Atoi(items[i].Meta["revision"])
		rj, e2 := strconv.Atoi(items[j].Meta["revision"])
		if e1 == nil && e2 == nil {
			return ri < rj
		}
		return items[i].Meta["revision"] < items[j].Meta["revision"]
	})
}

func buildSvnDiffReviewPrompt(skill, absDiff, sourceRefs string) string {
	prompt := "请使用 skill 文件 " + skill + " 检查 @" + absDiff + "。"
	prompt += "只输出审查结果，不要输出其它内容。"
	prompt += "\n输出必须严格按以下顺序与标记：\n"
	prompt += "[文件名]\n"
	prompt += "[问题代码]\n"
	prompt += "[修改建议]\n"
	prompt += "并在输出末尾单独写入一行：=== END ==="
	if strings.TrimSpace(sourceRefs) != "" {
		prompt += "\n" + sourceRefs
	}
	return prompt
}

func ensureSkillEndMarker(body string) string {
	if strings.Contains(body, "=== END ===") {
		return body
	}
	return body + "\n=== END ==="
}

func sourceRefsForSvnDiff(taskName string, in config.TaskInput, absDiff string) string {
	if !in.IncludeSvnSource || strings.TrimSpace(in.ProjectPath) == "" {
		return ""
	}
	refs, err := util.BuildSourceRefsFromDiff(absDiff, in.ProjectPath)
	if err != nil {
		log.Printf("job %s BuildSourceRefsFromDiff %s: %v", taskName, absDiff, err)
		return ""
	}
	return refs
}

func appendSvnDiffResultRecord(resultFileAbs, header, body string, blankBefore bool) error {
	f, err := util.OpenAppendFile(resultFileAbs)
	if err != nil {
		return err
	}
	if f == nil {
		return fmt.Errorf("open append result file failed: %s", resultFileAbs)
	}
	if blankBefore {
		_, _ = f.WriteString("\n")
	}
	_, _ = f.WriteString(header + "\n")
	_, _ = f.WriteString(body + "\n")
	return f.Close()
}

func agentWorkspaceFromOutDir(outDir, taskName string) string {
	abs, err := filepath.Abs(outDir)
	if err != nil {
		log.Printf("job %s: abs OutDir: %v", taskName, err)
		return outDir
	}
	return abs
}

func uploadResultIfNeeded(uploadURL, resultFileAbs, taskName string) {
	if uploadURL == "" {
		return
	}
	if err := util.UploadFile(uploadURL, resultFileAbs); err != nil {
		log.Printf("job %s upload result file failed: %v", taskName, err)
	}
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
	agentWorkspaceAbs := agentWorkspaceFromOutDir(taskOut.OutDir, jt.Name)

	sortInputItemsByRevision(items)

	for idx, item := range items {
		absDiff, _ := filepath.Abs(item.Path)
		header := svnSrc.RevisionHeaderForItem(item)
		sourceRefs := sourceRefsForSvnDiff(jt.Name, jt.Input, absDiff)
		prompt := buildSvnDiffReviewPrompt(jt.Skill, absDiff, sourceRefs)

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
		body = ensureSkillEndMarker(body)

		if err := appendSvnDiffResultRecord(resultFileAbs, header, body, idx > 0); err != nil {
			return err
		}
	}

	if util.IsFileEmptyOrMissing(resultFileAbs) {
		return fmt.Errorf("result file is empty: %s", resultFileAbs)
	}
	uploadResultIfNeeded(jt.Output.UploadURL, resultFileAbs, jt.Name)
	return nil
}
