package dingtalk

import "time"

type User struct {
	DingUserID string
	Name       string
	AvatarURL  string
	DeptIDs    []int64
	UnionID    string
	IsAdmin    bool
}

type CalendarRequest struct {
	Title    string
	StartAt  time.Time
	EndAt    time.Time
	UserIDs  []string
	Location string
	Detail   string
	// RoomIDs 为钉钉智能会议室的 roomId 列表。非空时，建完日程会把这些会议室加到事件上，
	// 钉钉会自动按 roomId 回填会议室名称显示在日程里（无需我们传名称）。
	RoomIDs []string
}

// MeetingRoom 智能会议室（来自钉钉 QueryMeetingRoomList）。
type MeetingRoom struct {
	RoomID   string
	RoomName string
	Capacity int
	Status   int
	Location string // 楼层/位置，由 roomLocation.title + desc 组合
}

type Card struct {
	Title  string         `json:"title"`
	Detail string         `json:"detail"`
	Extra  map[string]any `json:"extra,omitempty"`
}

type CardInstance struct {
	InstanceID string
	TraceID    string
}

type Response struct {
	UserID string
	Status string
}

type ApprovalRequest struct {
	ProcessCode string
	UserID      string
	FormData    map[string]any
}
