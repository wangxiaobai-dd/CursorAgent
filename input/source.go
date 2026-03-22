package input

import (
	"fmt"

	"CursorAgent/config"
	"CursorAgent/input/code"
	"CursorAgent/input/generic"
	"CursorAgent/input/types"
)

func NewSource(task *config.Task) (types.InputSource, error) {
	t := task.Input.Type
	switch t {
	case "svn_diff":
		return code.NewSvnDiffSource(&task.Input), nil
	case "file":
		return generic.NewFileSource(&task.Input), nil
	case "prompt":
		return generic.NewPromptSource(&task.Input), nil
	default:
		return nil, fmt.Errorf("unsupported input type: %s", t)
	}
}
