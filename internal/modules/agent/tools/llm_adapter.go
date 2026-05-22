package tools

import "github.com/standardsoftware/culture_points_mall/internal/platform/llm"

func AsLLMDefs(r *Registry) []llm.ToolDef {
	tools := r.List()
	out := make([]llm.ToolDef, 0, len(tools))
	for _, t := range tools {
		out = append(out, llm.ToolDef{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.InputSchema(),
		})
	}
	return out
}
