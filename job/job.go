package job

import (
	"log"
	"strings"
	"time"

	"CursorAgent/config"
	"CursorAgent/cursor"
	"CursorAgent/input/types"
	"CursorAgent/server"
	"CursorAgent/util"
)

type Job struct {
	Task           *config.Task
	Source         types.InputSource
	Cursor         *cursor.Client
	Opt            *config.Option
	PluginListener *server.PluginListener
}

func NewJob(task *config.Task, source types.InputSource, cursorClient *cursor.Client, opt *config.Option, pluginListener *server.PluginListener) *Job {
	return &Job{Task: task, Source: source, Cursor: cursorClient, Opt: opt, PluginListener: pluginListener}
}

func (j *Job) Run() {
	items, err := j.Source.GetInputs()
	if err != nil {
		log.Printf("job %s GetInputs failed: %v", j.Task.Name, err)
		return
	}
	if len(items) == 0 {
		log.Printf("job %s: no input items", j.Task.Name)
		return
	}

	defer func() {
		if c, ok := j.Source.(types.Cleanable); ok {
			c.Cleanup()
		}
	}()

	if err := j.clearResultFile(); err != nil {
		log.Printf("job %s: clear result file: %v", j.Task.Name, err)
		return
	}

	r, err := newTaskRunner(j)
	if err != nil {
		log.Printf("job %s: runner: %v", j.Task.Name, err)
		return
	}
	if err := r.run(items); err != nil {
		log.Printf("job %s: run failed: %v", j.Task.Name, err)
		return
	}

	if !j.Task.LaunchCursor {
		log.Printf("job %s: completed (items=%d)", j.Task.Name, len(items))
	}
}

func (j *Job) clearResultFile() error {
	out := j.Task.Output
	if strings.TrimSpace(out.OutDir) == "" || strings.TrimSpace(out.OutPrefix) == "" {
		return nil
	}
	date := time.Now().Format("2006-01-02")
	return util.ClearOrEmptyFile(out.ResultFilePath(date))
}
