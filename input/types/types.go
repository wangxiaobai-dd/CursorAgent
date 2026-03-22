package types

// InputItem 是一次任务中的单个输入单元（例如文件路径或纯提示词）。
type InputItem struct {
	Path   string            // 用于 @file 的文件路径；为空表示仅使用 Prompt
	Meta   map[string]string // 元信息（例如 revision/msg 用于结果头部）
	Prompt string            // Path 为空时，直接用该提示词调用
}

// InputSource 根据任务配置生成输入列表。
type InputSource interface {
	GetInputs() ([]InputItem, error)
	WorkDir() string // Cursor CLI 的工作目录（用于解析 @file 等相对路径）
}

// Cleanable 可选接口：用于在任务结束后清理临时文件（例如生成的 diff）。
type Cleanable interface {
	Cleanup()
}
