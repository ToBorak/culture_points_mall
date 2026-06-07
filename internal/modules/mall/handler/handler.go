package handler

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/standardsoftware/culture_points_mall/internal/modules/mall/repository"
	"github.com/standardsoftware/culture_points_mall/internal/modules/mall/service"
	cpmctx "github.com/standardsoftware/culture_points_mall/internal/shared/ctx"
)

type Handler struct {
	Repo      *repository.GormRepo
	Svc       *service.Service
	UploadDir string
}

func New(r *repository.GormRepo, s *service.Service, uploadDir string) *Handler {
	if uploadDir == "" {
		uploadDir = "./uploads"
	}
	return &Handler{Repo: r, Svc: s, UploadDir: uploadDir}
}

func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.GET("/api/v1/mall/items", h.list)
	rg.GET("/api/v1/mall/blindbox/:id/prizes", h.listPrizes)
	rg.POST("/api/v1/mall/blindbox/draw", h.draw)
	rg.POST("/api/v1/mall/items/:id/redeem", h.redeem)
	rg.GET("/api/v1/me/orders", h.myOrders)
}

func (h *Handler) RegisterAdmin(rg *gin.RouterGroup) {
	rg.POST("/api/v1/admin/mall/items", h.create)
	rg.PUT("/api/v1/admin/mall/items/:id", h.updateItem)
	rg.DELETE("/api/v1/admin/mall/items/:id", h.deleteItem)
	rg.POST("/api/v1/admin/mall/upload", h.upload)
	rg.GET("/api/v1/admin/mall/blindbox/:id/config", h.getBoxConfig)
	rg.PUT("/api/v1/admin/mall/blindbox/:id/config", h.putBoxConfig)
}

// upload 接收 multipart 图片，存到 UploadDir，返回可访问的相对 URL（/api/uploads/<file>）。
func (h *Handler) upload(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"error": "缺少文件字段 file"})
		return
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowed := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true}
	if !allowed[ext] {
		c.JSON(400, gin.H{"error": "仅支持 jpg/jpeg/png/webp/gif"})
		return
	}
	if file.Size > 8<<20 {
		c.JSON(400, gin.H{"error": "图片大小不能超过 8MB"})
		return
	}
	if err := os.MkdirAll(h.UploadDir, 0o755); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	name := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	if err := c.SaveUploadedFile(file, filepath.Join(h.UploadDir, name)); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"url": "/api/uploads/" + name})
}

func (h *Handler) list(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	if tid == 0 {
		tid = 1
	}
	rows, err := h.Repo.ListItems(c.Request.Context(), tid, c.Query("type"), true) // 商城只展示在售商品
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"items": rows})
}

func (h *Handler) listPrizes(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if id <= 0 {
		c.JSON(400, gin.H{"error": "invalid id"})
		return
	}
	rows, err := h.Repo.ListPrizeViews(c.Request.Context(), id)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	out := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		out = append(out, gin.H{
			"id":         r.ID,
			"itemId":     r.ItemID,
			"prizeName":  r.PrizeName,
			"prizeImage": r.PrizeImage,
			"weight":     r.Weight,
			"stock":      r.Stock,
			"cost":       r.Cost,
		})
	}
	c.JSON(200, gin.H{"items": out})
}

type drawReq struct {
	BoxID int64 `json:"boxId" binding:"required"`
}

func (h *Handler) draw(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	uid := cpmctx.UserID(c.Request.Context())
	var req drawReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	res, err := h.Svc.Draw(c.Request.Context(), tid, uid, req.BoxID)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, res)
}

type createItemReq struct {
	Type     string `json:"type" binding:"required"`
	Name     string `json:"name" binding:"required"`
	Cost     int    `json:"cost" binding:"required"`
	Stock    *int   `json:"stock"`
	ImageURL string `json:"image_url"`
}

func (h *Handler) create(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	if tid == 0 {
		tid = 1
	}
	var req createItemReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	it, err := h.Svc.CreateItem(c.Request.Context(), service.CreateItemCmd{
		TenantID: tid,
		Type:     req.Type,
		Name:     req.Name,
		Cost:     req.Cost,
		Stock:    req.Stock,
		ImageURL: req.ImageURL,
	})
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, it)
}

func (h *Handler) redeem(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	uid := cpmctx.UserID(c.Request.Context())
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if id <= 0 {
		c.JSON(400, gin.H{"error": "invalid id"})
		return
	}
	res, err := h.Svc.Redeem(c.Request.Context(), tid, uid, id)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, res)
}

func (h *Handler) myOrders(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	uid := cpmctx.UserID(c.Request.Context())
	rows, err := h.Repo.ListOrdersByUser(c.Request.Context(), tid, uid)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"items": rows})
}

func (h *Handler) deleteItem(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	if tid == 0 {
		tid = 1
	}
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if id <= 0 {
		c.JSON(400, gin.H{"error": "invalid id"})
		return
	}
	if err := h.Repo.DeleteItem(c.Request.Context(), tid, id); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

type updateItemReq struct {
	Name     *string `json:"name"`
	Cost     *int    `json:"cost"`
	Stock    *int    `json:"stock"`
	ImageURL *string `json:"image_url"`
}

// updateItem 局部更新商品基础信息（名称/积分/图片省略则不改）；stock 按提交值整体设置：
// 传数字=该库存，留空/null=不限量（编辑表单总是全量回填 stock，故不会被误清空）。
func (h *Handler) updateItem(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	if tid == 0 {
		tid = 1
	}
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if id <= 0 {
		c.JSON(400, gin.H{"error": "invalid id"})
		return
	}
	var req updateItemReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	it, _, err := h.Svc.UpdateItem(c.Request.Context(), service.UpdateItemCmd{
		TenantID: tid,
		ItemID:   id,
		Name:     req.Name,
		Cost:     req.Cost,
		Stock:    req.Stock,
		StockSet: true,
		ImageURL: req.ImageURL,
	})
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, it)
}

// getBoxConfig 返回某盲盒的配置：盲盒信息 + 无奖品权重 + 已配奖品 + 全部可选好物。
func (h *Handler) getBoxConfig(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	if tid == 0 {
		tid = 1
	}
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if id <= 0 {
		c.JSON(400, gin.H{"error": "invalid id"})
		return
	}
	box, err := h.Repo.GetItem(c.Request.Context(), tid, id)
	if err != nil {
		c.JSON(404, gin.H{"error": "盲盒不存在"})
		return
	}
	if box.Type != "blindbox" {
		c.JSON(400, gin.H{"error": "该商品不是盲盒"})
		return
	}
	prizes, err := h.Repo.ListPrizeViews(c.Request.Context(), id)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	goods, err := h.Repo.ListItems(c.Request.Context(), tid, "item", false)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	noPrizeWeight := 0
	prizeRows := make([]gin.H, 0, len(prizes))
	for _, p := range prizes {
		if p.ItemID == nil {
			noPrizeWeight = p.Weight
			continue
		}
		prizeRows = append(prizeRows, gin.H{
			"itemId": *p.ItemID, "weight": p.Weight, "stock": p.Stock,
			"name": p.PrizeName, "image": p.PrizeImage, "cost": p.Cost,
		})
	}
	c.JSON(200, gin.H{
		"box":           gin.H{"id": box.ID, "name": box.Name, "cost": box.Cost, "chargeOnMiss": box.ChargeOnMiss},
		"noPrizeWeight": noPrizeWeight,
		"prizes":        prizeRows,
		"goods":         goods,
	})
}

type boxPrizeReq struct {
	ItemID int64 `json:"itemId" binding:"required"`
	Weight int   `json:"weight"`
	Stock  *int  `json:"stock"`
}

type putBoxConfigReq struct {
	ChargeOnMiss  bool          `json:"chargeOnMiss"`
	NoPrizeWeight int           `json:"noPrizeWeight"`
	Prizes        []boxPrizeReq `json:"prizes"`
}

func (h *Handler) putBoxConfig(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	if tid == 0 {
		tid = 1
	}
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if id <= 0 {
		c.JSON(400, gin.H{"error": "invalid id"})
		return
	}
	var req putBoxConfigReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	cmd := service.SaveBoxConfigCmd{ChargeOnMiss: req.ChargeOnMiss, NoPrizeWeight: req.NoPrizeWeight}
	for _, p := range req.Prizes {
		iid := p.ItemID
		cmd.Prizes = append(cmd.Prizes, service.BoxPrizeCmd{ItemID: &iid, Weight: p.Weight, Stock: p.Stock})
	}
	if err := h.Svc.SaveBoxConfig(c.Request.Context(), tid, id, cmd); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
