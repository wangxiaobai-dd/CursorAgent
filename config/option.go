package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

type Option struct {
	AgentCommand string `yaml:"AgentCommand"`
	// AgentModel 传给 Cursor agent 的 --model（如 gpt-5、sonnet-4）；auto 或空表示不传 --model，使用 CLI 默认模型。
	AgentModel   string `yaml:"AgentModel"`
	DefaultOutDir      string `yaml:"DefaultOutDir"`
	DefaultUploadURL   string `yaml:"DefaultUploadURL"`
	LogFile            string `yaml:"LogFile"`
	CursorExePath      string `yaml:"CursorExePath"`
	PluginListenerPort int    `yaml:"PluginListenerPort"`
	Tasks              []Task `yaml:"Tasks"`
}

type Task struct {
	Name         string     `yaml:"Name"`
	Skill        string     `yaml:"Skill"`
	LaunchCursor bool       `yaml:"LaunchCursor"`
	CronTime     string     `yaml:"CronTime"`
	Input        TaskInput  `yaml:"Input"`
	Output       TaskOutput `yaml:"Output"`
}

type SvnDiffInput struct {
	CheckDay         int    `yaml:"CheckDay"`
	ProjectPath      string `yaml:"ProjectPath"`
	RepoURL          string `yaml:"RepoURL"`
	UserName         string `yaml:"UserName"`
	Password         string `yaml:"Password"`
	DiffDir          string `yaml:"DiffDir"`
	IncludeSvnSource bool   `yaml:"IncludeSvnSource"`
}

type FileInput struct {
	Paths []string `yaml:"Paths"`
}

type PromptInput struct {
	Prompt string `yaml:"Prompt"`
}

type TaskInput struct {
	Type string `yaml:"Type"`

	SvnDiffInput `yaml:",inline"`
	FileInput    `yaml:",inline"`
	PromptInput  `yaml:",inline"`
}

type TaskOutput struct {
	OutDir    string `yaml:"OutDir"`
	OutPrefix string `yaml:"OutPrefix"`
	UploadURL string `yaml:"UploadURL"`
}

func (o TaskOutput) ResultFilePath(date string) string {
	name := o.OutPrefix + "." + date
	return filepath.Join(o.OutDir, name)
}

func LoadOption(filePath string) (*Option, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var opt Option
	if err := yaml.Unmarshal(data, &opt); err != nil {
		return nil, err
	}
	if opt.PluginListenerPort == 0 {
		opt.PluginListenerPort = 9150
	}
	if strings.TrimSpace(opt.AgentModel) == "" {
		opt.AgentModel = "auto"
	}
	for i := range opt.Tasks {
		t := &opt.Tasks[i]
		if t.Output.OutDir == "" {
			t.Output.OutDir = opt.DefaultOutDir
		}
		if t.Output.UploadURL == "" && opt.DefaultUploadURL != "" {
			t.Output.UploadURL = opt.DefaultUploadURL
		}
	}
	if err := opt.Validate(); err != nil {
		return nil, err
	}
	return &opt, nil
}

func (o *Option) Validate() error {
	for i := range o.Tasks {
		t := &o.Tasks[i]
		if t.Input.Type == "" {
			return fmt.Errorf("task %q: missing Input.Type", t.Name)
		}
		switch t.Input.Type {
		case "svn_diff":
			if strings.TrimSpace(t.Input.RepoURL) == "" {
				return fmt.Errorf("task %q: Input.Type=svn_diff but RepoURL is empty", t.Name)
			}
			if strings.TrimSpace(t.Input.ProjectPath) == "" {
				return fmt.Errorf("task %q: Input.Type=svn_diff but ProjectPath is empty", t.Name)
			}
		case "file":
			if len(t.Input.Paths) == 0 {
				return fmt.Errorf("task %q: Input.Type=file but Paths is empty", t.Name)
			}
		case "prompt":
			if strings.TrimSpace(t.Input.Prompt) == "" {
				return fmt.Errorf("task %q: Input.Type=prompt but Prompt is empty", t.Name)
			}
		default:
			return fmt.Errorf("task %q: unsupported Input.Type %q", t.Name, t.Input.Type)
		}
		if t.LaunchCursor {
			if strings.TrimSpace(o.CursorExePath) == "" {
				return fmt.Errorf("task %q: LaunchCursor is true but CursorExePath is empty", t.Name)
			}
		} else {
			if strings.TrimSpace(t.Skill) == "" {
				return fmt.Errorf("task %q: missing Skill (non-empty skill name, e.g. code-review-skill)", t.Name)
			}
		}
	}
	return nil
}
