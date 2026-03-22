package job

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"CursorAgent/cursor"
	"CursorAgent/input/code"
	"CursorAgent/input/types"
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
	out := jt.Output
	svnSrc, ok := j.Source.(*code.SvnDiffSource)
	if !ok {
		return fmt.Errorf("svn_diff task must use SvnDiffSource")
	}
	if err := os.MkdirAll(out.OutDir, 0755); err != nil {
		log.Printf("job %s mkdir OutDir: %v", jt.Name, err)
		return err
	}
	date := time.Now().Format("2006-01-02")
	resultFileAbs, _ := filepath.Abs(out.ResultFilePath(date))
	agentWorkspaceAbs, werr := filepath.Abs(out.OutDir)
	if werr != nil {
		log.Printf("job %s: abs OutDir: %v", jt.Name, werr)
		agentWorkspaceAbs = out.OutDir
	}
	prompt := svnSrc.BuildMergedDiffReviewPrompt(jt.Skill, resultFileAbs, items)
	_, err := j.Cursor.RunSkill("", agentWorkspaceAbs, "", prompt)
	if err != nil {
		log.Printf("job %s cursor failed: %v", jt.Name, err)
		if errors.Is(err, cursor.ErrAgentCommandUnavailable) {
			return err
		}
		return err
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
