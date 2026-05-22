package llm

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// 内部统一消息块。参考 Anthropic 风格。
type Block struct {
	Type    string      `json:"type"`
	Text    string      `json:"text,omitempty"`
	ToolUse *ToolUse    `json:"tool_use,omitempty"`
	ToolRes *ToolResult `json:"tool_result,omitempty"`
}

type ToolUse struct {
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Input map[string]any `json:"input"`
}

type ToolResult struct {
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
	IsError   bool   `json:"is_error,omitempty"`
}

type Message struct {
	Role    Role    `json:"role"`
	Content []Block `json:"content"`
}

type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

type MessagesRequest struct {
	System    string    `json:"system,omitempty"`
	Messages  []Message `json:"messages"`
	Tools     []ToolDef `json:"tools,omitempty"`
	MaxTokens int       `json:"max_tokens,omitempty"`
	Stream    bool      `json:"stream,omitempty"`
}

type StopReason string

const (
	StopEnd   StopReason = "end_turn"
	StopTool  StopReason = "tool_use"
	StopMax   StopReason = "max_tokens"
	StopError StopReason = "error"
)

type MessagesResponse struct {
	Content    []Block    `json:"content"`
	StopReason StopReason `json:"stop_reason"`
	Model      string     `json:"model"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type StreamEvent struct {
	Type       string     `json:"type"`
	Delta      string     `json:"delta,omitempty"`
	ToolUse    *ToolUse   `json:"tool_use,omitempty"`
	BlockIndex int        `json:"block_index,omitempty"`
	StopReason StopReason `json:"stop_reason,omitempty"`
	Err        error      `json:"-"`
}
