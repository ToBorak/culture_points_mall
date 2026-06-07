package dingtalk

import "context"

type Client interface {
	GetUserByCode(ctx context.Context, code string) (User, error)

	CreateCalendarEvent(ctx context.Context, req CalendarRequest) (eventID string, err error)
	// DeleteCalendarEvent 删除日程（取消报名时移除自动加入的日程）。organizerUserID 为组织者的钉钉 userid。
	DeleteCalendarEvent(ctx context.Context, organizerUserID, eventID string) error
	ListCalendarResponses(ctx context.Context, eventID string) ([]Response, error)

	// QueryMeetingRooms 列出 unionID 这个用户可见的智能会议室（用于后台选会议室）。
	QueryMeetingRooms(ctx context.Context, unionID string) ([]MeetingRoom, error)

	SendWorkNotice(ctx context.Context, userIDs []string, msg Card) error
	SendInteractiveCard(ctx context.Context, target, cardTemplateID string, data map[string]any) (CardInstance, error)
	BotBroadcast(ctx context.Context, groupID string, msg Card) error

	StartOAProcess(ctx context.Context, req ApprovalRequest) (instance string, err error)
}
