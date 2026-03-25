package input

import (
	"fmt"

	"CursorAgent/config"
	"CursorAgent/input/code"
	"CursorAgent/input/generic"
	"CursorAgent/input/types"
)

// RunnerKind 与 Input.Type 对应，供 job 选择 taskRunner，避免在多处手写 switch。
type RunnerKind int

const (
	RunnerKindSvnDiff RunnerKind = iota
	RunnerKindGeneric
)

type inputTypeSpec struct {
	newSource  func(*config.TaskInput) types.InputSource
	runnerKind RunnerKind
}

// 新增输入类型时只改此表：同时登记 Source 工厂与 Runner 种类。
var inputTypes = map[string]inputTypeSpec{
	"svn_diff": {
		newSource:  func(in *config.TaskInput) types.InputSource { return code.NewSvnDiffSource(in) },
		runnerKind: RunnerKindSvnDiff,
	},
	"file": {
		newSource:  func(in *config.TaskInput) types.InputSource { return generic.NewFileSource(in) },
		runnerKind: RunnerKindGeneric,
	},
	"prompt": {
		newSource:  func(in *config.TaskInput) types.InputSource { return generic.NewPromptSource(in) },
		runnerKind: RunnerKindGeneric,
	},
}

// NewSource 根据 task.Input.Type 构造输入源。
func NewSource(task *config.Task) (types.InputSource, error) {
	spec, ok := inputTypes[task.Input.Type]
	if !ok {
		return nil, fmt.Errorf("unsupported input type: %s", task.Input.Type)
	}
	return spec.newSource(&task.Input), nil
}

// RunnerKindForInputType 返回该输入类型对应的 runner 策略；未知类型 ok 为 false。
func RunnerKindForInputType(t string) (RunnerKind, bool) {
	spec, ok := inputTypes[t]
	if !ok {
		return 0, false
	}
	return spec.runnerKind, true
}
