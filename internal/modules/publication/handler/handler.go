package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/standardsoftware/culture_points_mall/internal/modules/publication/domain"
	pubsvc "github.com/standardsoftware/culture_points_mall/internal/modules/publication/service"
	cpmctx "github.com/standardsoftware/culture_points_mall/internal/shared/ctx"
)

// Handler 文化刊 HTTP 处理器。
type Handler struct{ Svc *pubsvc.Service }

// New 构造 Handler。
func New(s *pubsvc.Service) *Handler { return &Handler{Svc: s} }

// RegisterAdmin 挂载 admin 端路由（需 admin 角色鉴权，由路由组在外层保证）。
func (h *Handler) RegisterAdmin(rg *gin.RouterGroup) {
	rg.GET("/admin/publications", h.adminList)
	rg.POST("/admin/publications", h.create)
	rg.PUT("/admin/publications/:id/sections", h.configureSections)
	rg.POST("/admin/publications/:id/aggregate", h.aggregate)
	rg.POST("/admin/publications/:id/articles", h.upsertArticle)
	rg.POST("/admin/publications/:id/publish", h.publish)
	rg.GET("/admin/publications/:id", h.adminDetail)
	rg.POST("/admin/publications/:id/ai-compose", h.aiCompose)
	rg.POST("/admin/publications/:id/ai-cases", h.aiCases)
	rg.POST("/admin/publications/:id/push-dingtalk", h.pushDingtalk)
}

// Register 挂载员工端只读路由（需登录鉴权，由路由组在外层保证）。
// 注意：/current 必须在 /:id 之前注册，否则 gin 会把 "current" 当成 :id 参数。
func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.GET("/api/v1/publications", h.list)
	rg.GET("/api/v1/publications/current", h.current)
	rg.GET("/api/v1/publications/:id", h.detail)
	rg.POST("/api/v1/culture-qa/ask", h.cultureQA)
}

// ─── Admin handlers ────────────────────────────────────────────────────────────

// adminList 列出租户下全部刊物（含草稿），供管理员使用。
func (h *Handler) adminList(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	rows, err := h.Svc.ListAllForAdmin(c.Request.Context(), tid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

func (h *Handler) create(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	var req struct {
		SeasonID   *int64 `json:"seasonId"`
		Title      string `json:"title" binding:"required"`
		PeriodCode string `json:"periodCode" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	p, err := h.Svc.CreateIssue(c.Request.Context(), pubsvc.CreateIssueCmd{
		TenantID: tid, SeasonID: req.SeasonID, Title: req.Title, PeriodCode: req.PeriodCode,
	})
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, p)
}

func (h *Handler) configureSections(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	pubID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		Sections []struct {
			Type      string `json:"type" binding:"required"`
			Title     string `json:"title" binding:"required"`
			SortOrder int    `json:"sortOrder"`
			Visible   bool   `json:"visible"`
		} `json:"sections" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	secs := make([]domain.Section, 0, len(req.Sections))
	for _, s := range req.Sections {
		secs = append(secs, domain.Section{
			Type:      domain.SectionType(s.Type),
			Title:     s.Title,
			SortOrder: s.SortOrder,
			Visible:   s.Visible,
		})
	}
	if err := h.Svc.ConfigureSections(c.Request.Context(), tid, pubID, secs); err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		case errors.Is(err, pubsvc.ErrNotDraft):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) aggregate(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	pubID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.Svc.Aggregate(c.Request.Context(), tid, pubID); err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) upsertArticle(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	pubID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var a domain.Article
	if err := c.ShouldBindJSON(&a); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	a.PublicationID = &pubID
	if err := h.Svc.UpsertArticle(c.Request.Context(), tid, &a); err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		case errors.Is(err, pubsvc.ErrNotDraft):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, a)
}

func (h *Handler) publish(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	pubID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.Svc.Publish(c.Request.Context(), tid, pubID); err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		case errors.Is(err, pubsvc.ErrNotDraft):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// adminDetail 返回指定刊物的完整视图（含草稿），供管理员预览。不校验 status，草稿可见。
func (h *Handler) adminDetail(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	pubID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	v, err := h.Svc.GetDetail(c.Request.Context(), tid, pubID)
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, v)
}

// ─── 员工端 handlers ───────────────────────────────────────────────────────────

func (h *Handler) list(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	rows, err := h.Svc.ListPublished(c.Request.Context(), tid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}

func (h *Handler) current(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	v, err := h.Svc.GetCurrent(c.Request.Context(), tid)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"publication": nil})
		return
	}
	c.JSON(http.StatusOK, v)
}

// detail 员工端按 id 获取刊物详情。
// 安全加固：校验 Status == PubPublished，未发布草稿返回 404，防止员工凭 id 读到草稿。
func (h *Handler) detail(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	pubID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	v, err := h.Svc.GetDetail(c.Request.Context(), tid, pubID)
	if err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	// 安全加固：员工端只能读已发布刊物，草稿/归档一律 404。
	if v.Publication.Status != domain.PubPublished {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, v)
}

// ─── AI 端点 ──────────────────────────────────────────────────────────────────

func (h *Handler) aiCompose(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	pubID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	if err := h.Svc.Compose(c.Request.Context(), tid, pubID); err != nil {
		switch {
		case errors.Is(err, pubsvc.ErrLLMUnavailable):
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		case errors.Is(err, gorm.ErrRecordNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) aiCases(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	pubID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	n, err := h.Svc.GenerateCaseArticles(c.Request.Context(), tid, pubID)
	if err != nil {
		if errors.Is(err, pubsvc.ErrLLMUnavailable) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"created": n})
}

func (h *Handler) cultureQA(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	var req struct {
		Question string `json:"question" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ans, err := h.Svc.CultureQA(c.Request.Context(), tid, req.Question)
	if err != nil {
		if errors.Is(err, pubsvc.ErrLLMUnavailable) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"answer": ans})
}

func (h *Handler) pushDingtalk(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	pubID, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	var req struct {
		GroupId string `json:"groupId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.Svc.PushDingtalk(c.Request.Context(), tid, pubID, req.GroupId); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
