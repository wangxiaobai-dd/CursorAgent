package generic

import (
	"bytes"
	"os"
	"text/template"
	"time"

	"CursorAgent/config"
	"CursorAgent/input/types"
)

type PromptSource struct {
	tpl     *template.Template
	workDir string
}

func NewPromptSource(opt *config.TaskInput) *PromptSource {
	tpl, _ := template.New("prompt").Parse(opt.Prompt)
	workDir, _ := os.Getwd()
	if workDir == "" {
		workDir = "."
	}
	return &PromptSource{tpl: tpl, workDir: workDir}
}

func (p *PromptSource) WorkDir() string { return p.workDir }

func (p *PromptSource) GetInputs() ([]types.InputItem, error) {
	data := map[string]string{
		"Date":      time.Now().Format("2006-01-02"),
		"WeekStart": weekStart().Format("2006-01-02"),
		"WeekEnd":   weekEnd().Format("2006-01-02"),
	}
	var buf bytes.Buffer
	if err := p.tpl.Execute(&buf, data); err != nil {
		return nil, err
	}
	prompt := buf.String()
	return []types.InputItem{{Prompt: prompt, Meta: map[string]string{"type": "prompt"}}}, nil
}

func weekStart() time.Time {
	t := time.Now()
	weekday := t.Weekday()
	if weekday == 0 {
		weekday = 7
	}
	return t.AddDate(0, 0, -int(weekday)+1)
}

func weekEnd() time.Time {
	return weekStart().AddDate(0, 0, 6)
}
