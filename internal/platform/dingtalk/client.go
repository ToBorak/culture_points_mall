package dingtalk

import "context"

type Client interface {
	GetUserByCode(ctx context.Context, code string) (User, error)

	CreateCalendarEvent(ctx context.Context, req CalendarRequest) (eventID string, err error)
	ListCalendarResponses(ctx context.Context, eventID string) ([]Response, error)

	SendWorkNotice(ctx context.Context, userIDs []string, msg Card) error
	SendInteractiveCard(ctx context.Context, target, cardTemplateID string, data map[string]any) (CardInstance, error)
	BotBroadcast(ctx context.Context, groupID string, msg Card) error

	StartOAProcess(ctx context.Context, req ApprovalRequest) (instance string, err error)
}
