package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAskUserTool_SignalShape(t *testing.T) {
	out, err := AskUserTool{}.Execute(context.Background(), map[string]any{
		"intent": "add_points",
		"questions": []any{
			map[string]any{"field": "amount", "label": "加多少分", "type": "number"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "slot_form", out["form"])
	require.Equal(t, "ask", out["source"])
	require.Equal(t, "add_points", out["intent"])
	require.NotNil(t, out["fields"])
	require.Equal(t, "请补充以下信息", out["title"]) // 未传 title 时的默认值
}

func TestOpenPointsFormTool_SignalShape(t *testing.T) {
	out, err := OpenPointsFormTool{}.Execute(context.Background(), map[string]any{
		"prefill": map[string]any{"amount": 100},
	})
	require.NoError(t, err)
	require.Equal(t, "slot_form", out["form"])
	require.Equal(t, "open", out["source"])
	require.Equal(t, "add_points", out["intent"])
	fields, ok := out["fields"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, fields, 4) // user_id / amount / dimension_code / reason
	require.Equal(t, map[string]any{"amount": 100}, out["prefill"])
}

func TestOpenMallItemFormTool_SignalShape(t *testing.T) {
	out, err := OpenMallItemFormTool{}.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	require.Equal(t, "slot_form", out["form"])
	require.Equal(t, "create_mall_item", out["intent"])
}

// 信号工具注册后应出现在给 LLM 的工具定义里（纯信号，不依赖 DB，可用空 deps 注册）。
func TestRegisterInteractive_SignalToolsExposed(t *testing.T) {
	r := NewRegistry()
	RegisterInteractive(r, InteractiveDeps{})
	have := map[string]bool{}
	for _, d := range AsLLMDefs(r) {
		have[d.Name] = true
	}
	for _, name := range []string{"ask_user", "open_points_form", "open_mall_item_form", "open_award_badge_form", "list_users", "list_badges"} {
		require.True(t, have[name], "expected tool %q registered", name)
	}
}
