package dingtalk

import "context"

type Client interface {
	GetUserByCode(ctx context.Context, code string) (User, error)

	CreateCalendarEvent(ctx context.Context, req CalendarRequest) (eventID string, err error)
	ListCalendarResponses(ctx context.Context, eventID string) ([]Response, error)

	// QueryMeetingRooms 列出 unionID 这个用户可见的智能会议室（用于后台选会议室）。
	QueryMeetingRooms(ctx context.Context, unionID string) ([]MeetingRoom, error)

	SendWorkNotice(ctx context.Context, userIDs []string, msg Card) error
	SendInteractiveCard(ctx context.Context, target, cardTemplateID string, data map[string]any) (CardInstance, error)
	BotBroadcast(ctx context.Context, groupID string, msg Card) error

	StartOAProcess(ctx context.Context, req ApprovalRequest) (instance string, err error)
}
