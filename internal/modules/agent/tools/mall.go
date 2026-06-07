package tools

import (
	"context"
	"fmt"

	mallsvc "github.com/standardsoftware/culture_points_mall/internal/modules/mall/service"
	cpmctx "github.com/standardsoftware/culture_points_mall/internal/shared/ctx"
)

const (
	itemOnShelf  = "on_shelf"
	itemOffShelf = "off_shelf"
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
		"status":    it.Status,
		"_undo": map[string]any{
			"label":  "撤销新增「" + it.Name + "」",
			"action": "mall_delete",
			"params": map[string]any{"item_id": it.ID},
		},
	}, nil
}

type ListMallItemsTool struct{ Deps MallDeps }

func (ListMallItemsTool) Name() string { return "list_mall_items" }
func (ListMallItemsTool) Description() string {
	return "列出积分商城商品。type 可选 'item'（普通兑换）或 'blindbox'（盲盒），不传则列全部。"
}
func (ListMallItemsTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"type": map[string]any{"type": "string", "enum": []string{"", "item", "blindbox"}},
		},
	}
}

func (t ListMallItemsTool) Execute(ctx context.Context, in map[string]any) (map[string]any, error) {
	tid := cpmctx.TenantID(ctx)
	if tid == 0 {
		tid = 1
	}
	rows, err := t.Deps.Mall.ListItems(ctx, tid, anyString(in["type"]))
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"id":        r.ID,
			"type":      r.Type,
			"name":      r.Name,
			"cost":      r.Cost,
			"stock":     r.Stock,
			"image_url": r.ImageURL,
			"status":    r.Status,
		})
	}
	return map[string]any{"items": out, "total": len(out)}, nil
}

// ---- update_mall_item ----

type UpdateMallItemTool struct{ Deps MallDeps }

func (UpdateMallItemTool) Name() string { return "update_mall_item" }
func (UpdateMallItemTool) Description() string {
	return "修改已有商品（按 item_id，先用 list_mall_items 查到）：可改 name / cost / stock / image_url；不传的字段保持不变；改为不限量传 unlimited_stock=true。"
}
func (UpdateMallItemTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"item_id":         map[string]any{"type": "integer"},
			"name":            map[string]any{"type": "string"},
			"cost":            map[string]any{"type": "integer", "description": "兑换所需积分（>0）"},
			"stock":           map[string]any{"type": "integer", "description": "库存上限"},
			"unlimited_stock": map[string]any{"type": "boolean", "description": "true=改为不限量（忽略 stock）"},
			"image_url":       map[string]any{"type": "string"},
		},
		"required": []string{"item_id"},
	}
}
func (t UpdateMallItemTool) Execute(ctx context.Context, in map[string]any) (map[string]any, error) {
	tid := cpmctx.TenantID(ctx)
	if tid == 0 {
		tid = 1
	}
	cmd := mallsvc.UpdateItemCmd{TenantID: tid, ItemID: int64(anyInt(in["item_id"]))}
	if v, ok := in["name"]; ok {
		s := anyString(v)
		cmd.Name = &s
	}
	if v, ok := in["cost"]; ok {
		c := anyInt(v)
		cmd.Cost = &c
	}
	if b, _ := in["unlimited_stock"].(bool); b {
		cmd.StockSet = true
		cmd.Stock = nil
	} else if v, ok := in["stock"]; ok {
		n := anyInt(v)
		cmd.StockSet = true
		cmd.Stock = &n
	}
	if v, ok := in["image_url"]; ok {
		s := anyString(v)
		cmd.ImageURL = &s
	}
	newIt, old, err := t.Deps.Mall.UpdateItem(ctx, cmd)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"item_id": newIt.ID, "name": newIt.Name, "cost": newIt.Cost, "stock": newIt.Stock, "status": newIt.Status,
		"_undo": map[string]any{
			"label":  "撤销修改「" + old.Name + "」",
			"action": "mall_restore",
			"params": map[string]any{"item_id": old.ID, "name": old.Name, "cost": old.Cost, "stock": old.Stock, "image_url": old.ImageURL},
		},
	}, nil
}

// ---- delist_mall_item / relist_mall_item ----

type DelistMallItemTool struct{ Deps MallDeps }

func (DelistMallItemTool) Name() string { return "delist_mall_item" }
func (DelistMallItemTool) Description() string {
	return "下架商品（按 item_id）：商城不再展示、不可兑换，商品记录保留，可随时上架恢复。"
}
func (DelistMallItemTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object", "properties": map[string]any{"item_id": map[string]any{"type": "integer"}},
		"required": []string{"item_id"},
	}
}
func (t DelistMallItemTool) Execute(ctx context.Context, in map[string]any) (map[string]any, error) {
	tid := cpmctx.TenantID(ctx)
	if tid == 0 {
		tid = 1
	}
	id := int64(anyInt(in["item_id"]))
	prev, err := t.Deps.Mall.SetItemStatus(ctx, tid, id, itemOffShelf)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"item_id": id, "status": itemOffShelf,
		"_undo": map[string]any{
			"label": "撤销下架（重新上架）", "action": "mall_set_status",
			"params": map[string]any{"item_id": id, "status": prev},
		},
	}, nil
}

type RelistMallItemTool struct{ Deps MallDeps }

func (RelistMallItemTool) Name() string { return "relist_mall_item" }
func (RelistMallItemTool) Description() string { return "上架商品（按 item_id）：让已下架的商品重新在商城展示、可兑换。" }
func (RelistMallItemTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object", "properties": map[string]any{"item_id": map[string]any{"type": "integer"}},
		"required": []string{"item_id"},
	}
}
func (t RelistMallItemTool) Execute(ctx context.Context, in map[string]any) (map[string]any, error) {
	tid := cpmctx.TenantID(ctx)
	if tid == 0 {
		tid = 1
	}
	id := int64(anyInt(in["item_id"]))
	prev, err := t.Deps.Mall.SetItemStatus(ctx, tid, id, itemOnShelf)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"item_id": id, "status": itemOnShelf,
		"_undo": map[string]any{
			"label": "撤销上架", "action": "mall_set_status",
			"params": map[string]any{"item_id": id, "status": prev},
		},
	}, nil
}

// ---- batch_update_mall ----

type BatchUpdateMallTool struct{ Deps MallDeps }

func (BatchUpdateMallTool) Name() string { return "batch_update_mall" }
func (BatchUpdateMallTool) Description() string {
	return "批量处理商品：item_ids 数组 + action（delist 批量下架 / relist 批量上架 / set_stock 批量改库存，set_stock 时配合 stock 或 unlimited_stock）。"
}
func (BatchUpdateMallTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"item_ids":        map[string]any{"type": "array", "items": map[string]any{"type": "integer"}},
			"action":          map[string]any{"type": "string", "enum": []string{"delist", "relist", "set_stock"}},
			"stock":           map[string]any{"type": "integer"},
			"unlimited_stock": map[string]any{"type": "boolean"},
		},
		"required": []string{"item_ids", "action"},
	}
}
func (t BatchUpdateMallTool) Execute(ctx context.Context, in map[string]any) (map[string]any, error) {
	tid := cpmctx.TenantID(ctx)
	if tid == 0 {
		tid = 1
	}
	var ids []int64
	if arr, ok := in["item_ids"].([]any); ok {
		for _, v := range arr {
			ids = append(ids, int64(anyInt(v)))
		}
	}
	action := anyString(in["action"])
	undos := make([]map[string]any, 0, len(ids))
	done := 0
	for _, id := range ids {
		switch action {
		case "delist", "relist":
			target := itemOffShelf
			if action == "relist" {
				target = itemOnShelf
			}
			prev, err := t.Deps.Mall.SetItemStatus(ctx, tid, id, target)
			if err != nil {
				continue
			}
			undos = append(undos, map[string]any{"action": "mall_set_status", "params": map[string]any{"item_id": id, "status": prev}})
			done++
		case "set_stock":
			cmd := mallsvc.UpdateItemCmd{TenantID: tid, ItemID: id, StockSet: true}
			if b, _ := in["unlimited_stock"].(bool); !b {
				n := anyInt(in["stock"])
				cmd.Stock = &n
			}
			_, old, err := t.Deps.Mall.UpdateItem(ctx, cmd)
			if err != nil {
				continue
			}
			undos = append(undos, map[string]any{"action": "mall_restore", "params": map[string]any{"item_id": old.ID, "name": old.Name, "cost": old.Cost, "stock": old.Stock, "image_url": old.ImageURL}})
			done++
		}
	}
	return map[string]any{
		"action": action, "count": done,
		"_undo": map[string]any{
			"label": fmt.Sprintf("撤销批量操作（%d 个商品）", done), "action": "batch",
			"params": map[string]any{"undos": undos},
		},
	}, nil
}

// ---- open_mall_batch（信号：弹出商品批量表格，内嵌全部商品）----

type OpenMallBatchTool struct{ Deps MallDeps }

func (OpenMallBatchTool) Name() string { return "open_mall_batch" }
func (OpenMallBatchTool) Description() string {
	return "当用户想批量管理商品（批量下架/上架/改库存）时调用，弹出商品批量表格列出所有商品供勾选。调用后只回一句话提示用户在表格里勾选商品并选择操作，不要自己逐个改。"
}
func (OpenMallBatchTool) InputSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}
func (t OpenMallBatchTool) Execute(ctx context.Context, _ map[string]any) (map[string]any, error) {
	tid := cpmctx.TenantID(ctx)
	if tid == 0 {
		tid = 1
	}
	rows, err := t.Deps.Mall.ListItems(ctx, tid, "")
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		items = append(items, map[string]any{
			"id": r.ID, "name": r.Name, "type": r.Type, "cost": r.Cost, "stock": r.Stock, "status": r.Status,
		})
	}
	return map[string]any{"form": "mall_batch", "title": "批量管理商品", "items": items}, nil
}

func RegisterMall(r *Registry, deps MallDeps) {
	r.MustRegister(CreateMallItemTool{deps})
	r.MustRegister(ListMallItemsTool{deps})
	r.MustRegister(UpdateMallItemTool{deps})
	r.MustRegister(DelistMallItemTool{deps})
	r.MustRegister(RelistMallItemTool{deps})
	r.MustRegister(BatchUpdateMallTool{deps})
	r.MustRegister(OpenMallBatchTool{deps})
}
