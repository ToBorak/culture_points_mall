package dingtalk

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	cpmctx "github.com/standardsoftware/culture_points_mall/internal/shared/ctx"
)

type mockOutbox struct {
	ID        int64           `gorm:"primaryKey"`
	TenantID  int64           `gorm:"column:tenant_id"`
	API       string          `gorm:"column:api"`
	Target    string          `gorm:"column:target"`
	Payload   json.RawMessage `gorm:"column:payload"`
	CreatedAt time.Time       `gorm:"column:created_at"`
}

func (mockOutbox) TableName() string { return "dingtalk_mock_outbox" }

type MockClient struct {
	DB  *gorm.DB
	Bus *Bus
}

func NewMock(db *gorm.DB, bus *Bus) *MockClient { return &MockClient{DB: db, Bus: bus} }

func (m *MockClient) record(ctx context.Context, api, target string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	row := mockOutbox{
		TenantID: cpmctx.TenantID(ctx),
		API:      api,
		Target:   target,
		Payload:  raw,
	}
	if err := m.DB.WithContext(ctx).Create(&row).Error; err != nil {
		return err
	}
	var pm map[string]any
	_ = json.Unmarshal(raw, &pm)
	m.Bus.Publish(Event{API: api, Target: target, Payload: pm})
	return nil
}

func (m *MockClient) GetUserByCode(_ context.Context, code string) (User, error) {
	return User{
		DingUserID: "mock_" + code,
		Name:       "Mock 用户 " + code,
		AvatarURL:  fmt.Sprintf("https://api.dicebear.com/9.x/notionists/svg?seed=%s", code),
		UnionID:    "mock_union_" + code,
		IsAdmin:    false,
	}, nil
}

func (m *MockClient) CreateCalendarEvent(ctx context.Context, req CalendarRequest) (string, error) {
	eventID := fmt.Sprintf("mock-cal-%d", time.Now().UnixNano())
	return eventID, m.record(ctx, "create_calendar", strings.Join(req.UserIDs, ","), req)
}

func (m *MockClient) ListCalendarResponses(_ context.Context, _ string) ([]Response, error) {
	return nil, nil
}

func (m *MockClient) QueryMeetingRooms(_ context.Context, _ string) ([]MeetingRoom, error) {
	return []MeetingRoom{
		{RoomID: "mock-room-1", RoomName: "三楼大会议室", Capacity: 20, Status: 1, Location: "总部 3F"},
		{RoomID: "mock-room-2", RoomName: "二楼洽谈室", Capacity: 6, Status: 1, Location: "总部 2F"},
	}, nil
}

func (m *MockClient) SendWorkNotice(ctx context.Context, userIDs []string, msg Card) error {
	return m.record(ctx, "send_work_notice", strings.Join(userIDs, ","), msg)
}

func (m *MockClient) SendInteractiveCard(ctx context.Context, target, tpl string, data map[string]any) (CardInstance, error) {
	if err := m.record(ctx, "send_interactive_card", target, map[string]any{"template": tpl, "data": data}); err != nil {
		return CardInstance{}, err
	}
	return CardInstance{InstanceID: fmt.Sprintf("mock-card-%d", time.Now().UnixNano())}, nil
}

func (m *MockClient) BotBroadcast(ctx context.Context, groupID string, msg Card) error {
	return m.record(ctx, "bot_broadcast", groupID, msg)
}

func (m *MockClient) StartOAProcess(ctx context.Context, req ApprovalRequest) (string, error) {
	instance := fmt.Sprintf("mock-oa-%d", time.Now().UnixNano())
	return instance, m.record(ctx, "start_oa_process", req.UserID, req)
}
