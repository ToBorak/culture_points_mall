package dingtalk

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	cpmctx "github.com/standardsoftware/culture_points_mall/internal/shared/ctx"
)

// MeetingRoomsHandler 暴露当前管理员可见的智能会议室列表，供后台「发布日程」时选会议室。
type MeetingRoomsHandler struct {
	ding Client
	db   *gorm.DB
}

func NewMeetingRoomsHandler(ding Client, db *gorm.DB) *MeetingRoomsHandler {
	return &MeetingRoomsHandler{ding: ding, db: db}
}

func (h *MeetingRoomsHandler) RegisterAdmin(rg *gin.RouterGroup) {
	rg.GET("/admin/dingtalk/meeting-rooms", h.list)
}

func (h *MeetingRoomsHandler) list(c *gin.Context) {
	ctx := c.Request.Context()
	tid := cpmctx.TenantID(ctx)
	uid := cpmctx.UserID(ctx)

	// 钉钉查会议室需 unionId：取当前管理员登录时落库的 union_id。
	// mock 模式下 QueryMeetingRooms 忽略 unionId，空值也能返回示例会议室，不阻断本地联调。
	var row struct {
		UnionID string `gorm:"column:union_id"`
	}
	if err := h.db.WithContext(ctx).
		Table("users").Select("union_id").
		Where("tenant_id = ? AND id = ?", tid, uid).
		Scan(&row).Error; err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	// 兜底：管理后台开发态走 dev 登录，账号没有 unionId。会议室列表只用于「展示选择」，
	// 真正占用会议室走日程组织者的 unionId（见 CreateCalendarEvent→addMeetingRooms），
	// 故这里退而取本租户任一带真实 unionId 的员工来列会议室（排除 mock 冒烟种子用户；
	// 本组织会议室为全员可见）。生产中管理员走钉钉登录、自带 unionId，不会进此分支。
	if row.UnionID == "" {
		_ = h.db.WithContext(ctx).
			Table("users").Select("union_id").
			Where("tenant_id = ? AND union_id IS NOT NULL AND union_id <> '' AND union_id NOT LIKE 'mock%'", tid).
			Order("id").Limit(1).
			Scan(&row).Error
	}
	if row.UnionID == "" {
		// dev 登录且本租户尚无任何钉钉员工：返回空，避免拿空 unionId 调钉钉报 user.not.employee
		c.JSON(200, gin.H{"items": []gin.H{}, "note": "本租户暂无带 unionId 的员工，无法查询会议室（需先有员工用钉钉登录）"})
		return
	}

	rooms, err := h.ding.QueryMeetingRooms(ctx, row.UnionID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	out := make([]gin.H, 0, len(rooms))
	for _, r := range rooms {
		out = append(out, gin.H{
			"roomId": r.RoomID, "roomName": r.RoomName,
			"capacity": r.Capacity, "status": r.Status, "location": r.Location,
		})
	}
	c.JSON(200, gin.H{"items": out})
}
