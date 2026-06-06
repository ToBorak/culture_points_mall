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
