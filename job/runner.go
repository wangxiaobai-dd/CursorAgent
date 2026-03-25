package job

import (
	"errors"
	"fmt"
	"log"

	"CursorAgent/cursor"
	"CursorAgent/input"
	"CursorAgent/input/types"
)

type taskRunner interface {
	run(items []types.InputItem) error
}

func newTaskRunner(j *Job) (taskRunner, error) {
	if j.Task.LaunchCursor {
		return &launchCursorRunner{j: j}, nil
	}
	kind, ok := input.RunnerKindForInputType(j.Task.Input.Type)
	if !ok {
		return nil, fmt.Errorf("unsupported Input.Type for runner: %q", j.Task.Input.Type)
	}
	switch kind {
	case input.RunnerKindSvnDiff:
		return &svnDiffRunner{j: j}, nil
	case input.RunnerKindGeneric:
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
