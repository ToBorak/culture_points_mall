package handler

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/standardsoftware/culture_points_mall/internal/modules/schedule/service"
	cpmctx "github.com/standardsoftware/culture_points_mall/internal/shared/ctx"
)

type Handler struct{ Svc *service.Service }

func New(s *service.Service) *Handler { return &Handler{Svc: s} }

func (h *Handler) RegisterAdmin(rg *gin.RouterGroup) {
	rg.POST("/admin/schedules", h.create)
	rg.GET("/admin/schedules", h.list)
}

type createReq struct {
	Title           string   `json:"title" binding:"required"`
	StartAt         string   `json:"startAt" binding:"required"`
	EndAt           string   `json:"endAt" binding:"required"`
	Location        string   `json:"location"`
	Detail          string   `json:"detail"`
	AttendeeUserIDs []string `json:"attendeeUserIds"`
	GroupIDs        []string `json:"groupIds"`
	PushCalendar    bool     `json:"pushCalendar"`
	PushGroup       bool     `json:"pushGroup"`
}

func (h *Handler) create(c *gin.Context) {
	var req createReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	start, err := time.Parse(time.RFC3339, req.StartAt)
	if err != nil {
		c.JSON(400, gin.H{"error": "startAt 非 RFC3339: " + err.Error()})
		return
	}
	end, err := time.Parse(time.RFC3339, req.EndAt)
	if err != nil {
		c.JSON(400, gin.H{"error": "endAt 非 RFC3339: " + err.Error()})
		return
	}
	// 归一到东八区：前端 datetime-local 经 toISOString() 发来的是 UTC，
	// 而 CreateCalendarEvent 用 timeZone=Asia/Shanghai，不归一会差 8 小时。
	if loc, e := time.LoadLocation("Asia/Shanghai"); e == nil {
		start = start.In(loc)
		end = end.In(loc)
	}
	tid := cpmctx.TenantID(c.Request.Context())
	uid := cpmctx.UserID(c.Request.Context())
	sch, err := h.Svc.Create(c.Request.Context(), service.CreateCmd{
		TenantID: tid, Title: req.Title, StartAt: start, EndAt: end,
		Location: req.Location, Detail: req.Detail, AttendeeUserIDs: req.AttendeeUserIDs,
		GroupIDs: req.GroupIDs, PushCalendar: req.PushCalendar, PushGroup: req.PushGroup, CreatedBy: uid,
	})
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{
		"id": sch.ID, "status": sch.Status, "calendarEventId": sch.CalendarEventID, "resultNote": sch.ResultNote,
	})
}

func (h *Handler) list(c *gin.Context) {
	tid := cpmctx.TenantID(c.Request.Context())
	rows, err := h.Svc.List(c.Request.Context(), tid)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	out := make([]gin.H, 0, len(rows))
	for _, s := range rows {
		out = append(out, gin.H{
			"id": s.ID, "title": s.Title, "startAt": s.StartAt, "endAt": s.EndAt,
			"location": s.Location, "status": s.Status, "calendarEventId": s.CalendarEventID,
			"resultNote": s.ResultNote, "createdAt": s.CreatedAt,
		})
	}
	c.JSON(200, gin.H{"items": out})
}
