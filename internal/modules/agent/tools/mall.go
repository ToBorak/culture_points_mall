package tools

import (
	"context"

	mallsvc "github.com/standardsoftware/culture_points_mall/internal/modules/mall/service"
	cpmctx "github.com/standardsoftware/culture_points_mall/internal/shared/ctx"
)

type MallDeps struct {
	Mall *mallsvc.Service
}

type CreateMallItemTool struct{ Deps MallDeps }

func (CreateMallItemTool) Name() string { return "create_mall_item" }
func (CreateMallItemTool) Description() string {
	return "新增积分商城商品。type='item' 普通兑换商品；type='blindbox' 盲盒。stock 为空表示不限量。"
}
func (CreateMallItemTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"type":      map[string]any{"type": "string", "enum": []string{"item", "blindbox"}},
			"name":      map[string]any{"type": "string"},
			"cost":      map[string]any{"type": "integer", "description": "兑换所需积分（必须 > 0）"},
			"stock":     map[string]any{"type": "integer", "description": "可选，库存上限"},
			"image_url": map[string]any{"type": "string"},
		},
		"required": []string{"type", "name", "cost"},
	}
}

func (t CreateMallItemTool) Execute(ctx context.Context, in map[string]any) (map[string]any, error) {
	tid := cpmctx.TenantID(ctx)
	if tid == 0 {
		tid = 1
	}
	cmd := mallsvc.CreateItemCmd{
		TenantID: tid,
		Type:     anyString(in["type"]),
		Name:     anyString(in["name"]),
		Cost:     anyInt(in["cost"]),
		ImageURL: anyString(in["image_url"]),
	}
	if _, ok := in["stock"]; ok {
		s := anyInt(in["stock"])
		cmd.Stock = &s
	}
	it, err := t.Deps.Mall.CreateItem(ctx, cmd)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"item_id":   it.ID,
		"type":      it.Type,
		"name":      it.Name,
		"cost":      it.Cost,
		"stock":     it.Stock,
		"image_url": it.ImageURL,
	}, nil
}

func RegisterMall(r *Registry, deps MallDeps) {
	r.MustRegister(CreateMallItemTool{deps})
}
