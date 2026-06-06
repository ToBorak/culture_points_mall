package publish

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	cpmctx "github.com/standardsoftware/culture_points_mall/internal/shared/ctx"
)

type Handler struct{ Svc *Service }

func NewHandler(s *Service) *Handler { return &Handler{Svc: s} }

func (h *Handler) RegisterAdmin(rg *gin.RouterGroup) {
	rg.POST("/admin/activities/publish", h.publish)
}

type publishReq struct {
	Title           string   `json:"title" binding:"required"`
	DimensionCode   string   `json:"dimensionCode" binding:"required"`
	StartAt         string   `json:"startAt" binding:"required"`
	EndAt           string   `json:"endAt" binding:"required"`
	PointsReward    int      `json:"pointsReward"`
	Capacity        *int     `json:"capacity"`
	Location        string   `json:"location"`
	Detail          string   `json:"detail"`
	RoomIDs         []string `json:"roomIds"`
	GroupIDs        []string `json:"groupIds"`
	PushGroup       bool     `json:"pushGroup"`
	AttendeeAll     bool     `json:"attendeeAll"`
	AttendeeUserIDs []string `json:"attendeeUserIds"`
}

// publish 走 SSE，把发布过程的每个阶段实时渲染成对话气泡（与 /admin/agent/chat 同款 step 协议）。
func (h *Handler) publish(c *gin.Context) {
	var req publishReq
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
	// 与 schedule handler 一致：datetime-local 经 toISOString 来的是 UTC，归一到东八区。
	if loc, e := time.LoadLocation("Asia/Shanghai"); e == nil {
		start = start.In(loc)
		end = end.In(loc)
	}

	tid := cpmctx.TenantID(c.Request.Context())
	uid := cpmctx.UserID(c.Request.Context())

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Writer.Flush()

	emit := func(step map[string]any) {
		raw, _ := json.Marshal(step)
		_, _ = c.Writer.Write([]byte("event: step\ndata: "))
		_, _ = c.Writer.Write(raw)
		_, _ = c.Writer.Write([]byte("\n\n"))
		c.Writer.Flush()
	}

	res := h.Svc.Publish(c.Request.Context(), Cmd{
		TenantID: tid, CreatedBy: uid,
		DimensionCode: req.DimensionCode, Title: req.Title,
		StartAt: start, EndAt: end, PointsReward: req.PointsReward, Capacity: req.Capacity,
		Location: req.Location, Detail: req.Detail,
		RoomIDs: req.RoomIDs, GroupIDs: req.GroupIDs, PushGroup: req.PushGroup,
		AttendeeAll: req.AttendeeAll, AttendeeUserIDs: req.AttendeeUserIDs,
	})

	for _, st := range res.Stages {
		emit(map[string]any{"kind": "tool_use", "toolName": st.Name})
		if st.OK {
			emit(map[string]any{"kind": "tool_result", "toolName": st.Name, "output": st.Output})
		} else {
			emit(map[string]any{"kind": "tool_result", "toolName": st.Name, "error": st.Error})
		}
	}
	emit(map[string]any{"kind": "llm_text", "text": summary(res)})
	emit(map[string]any{"kind": "done"})
}

func summary(res Result) string {
	for _, st := range res.Stages {
		if !st.OK {
			return "⚠️ 活动「" + res.Title + "」发布未完全成功，请看上面各步骤的报错。"
		}
	}
	b := "✅ 活动「" + res.Title + "」已发布"
	if res.CalendarEventID != "" {
		b += "，已建钉钉日程并通知 " + strconv.Itoa(res.AttendeeCount) + " 人"
	} else if res.AttendeeCount == 0 {
		b += "（未建日程：没有可通知的成员，确认成员已绑定钉钉）"
	}
	return b + "。"
}
