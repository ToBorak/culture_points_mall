package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/standardsoftware/culture_points_mall/internal/modules/schedule/domain"
	"github.com/standardsoftware/culture_points_mall/internal/platform/dingtalk"
	"github.com/standardsoftware/culture_points_mall/internal/platform/llm"
)

type fakeLLM struct{ reply string }

func (f fakeLLM) Messages(context.Context, llm.MessagesRequest) (llm.MessagesResponse, error) {
	return llm.MessagesResponse{Content: []llm.Block{{Type: "text", Text: f.reply}}}, nil
}
func (f fakeLLM) MessagesStream(context.Context, llm.MessagesRequest) (<-chan llm.StreamEvent, error) {
	return nil, nil
}

type memRepo struct{ saved *domain.Schedule }

func (m *memRepo) Create(_ context.Context, s *domain.Schedule) error { s.ID = 1; m.saved = s; return nil }
func (m *memRepo) ListByTenant(_ context.Context, _ int64, _ int) ([]domain.Schedule, error) {
	if m.saved == nil {
		return nil, nil
	}
	return []domain.Schedule{*m.saved}, nil
}

type fakeDing struct {
	calReq   dingtalk.CalendarRequest
	bcGroups []string
	bcCard   dingtalk.Card
	calErr   error
	bcErr    error
}

func (f *fakeDing) GetUserByCode(context.Context, string) (dingtalk.User, error) { return dingtalk.User{}, nil }
func (f *fakeDing) CreateCalendarEvent(_ context.Context, r dingtalk.CalendarRequest) (string, error) {
	f.calReq = r
	if f.calErr != nil {
		return "", f.calErr
	}
	return "evt-99", nil
}
func (f *fakeDing) ListCalendarResponses(context.Context, string) ([]dingtalk.Response, error) { return nil, nil }
func (f *fakeDing) QueryMeetingRooms(context.Context, string) ([]dingtalk.MeetingRoom, error) {
	return nil, nil
}
func (f *fakeDing) SendWorkNotice(context.Context, []string, dingtalk.Card) error             { return nil }
func (f *fakeDing) SendInteractiveCard(context.Context, string, string, map[string]any) (dingtalk.CardInstance, error) {
	return dingtalk.CardInstance{}, nil
}
func (f *fakeDing) BotBroadcast(_ context.Context, groupID string, card dingtalk.Card) error {
	f.bcGroups = append(f.bcGroups, groupID)
	f.bcCard = card
	return f.bcErr
}
func (f *fakeDing) StartOAProcess(context.Context, dingtalk.ApprovalRequest) (string, error) { return "", nil }

func TestService_Create_BothChannels(t *testing.T) {
	repo := &memRepo{}
	ding := &fakeDing{}
	s := New(repo, ding)
	now := time.Now()
	out, err := s.Create(context.Background(), CreateCmd{
		TenantID: 1, Title: "周会", StartAt: now, EndAt: now.Add(time.Hour),
		Location: "线上", Detail: "聊聊", AttendeeUserIDs: []string{"u1", "u2"},
		GroupIDs: []string{"culture"}, PushCalendar: true, PushGroup: true, CreatedBy: 9,
	})
	require.NoError(t, err)
	require.Equal(t, domain.StatusPublished, out.Status)
	require.Equal(t, "evt-99", out.CalendarEventID)
	require.Equal(t, []string{"u1", "u2"}, ding.calReq.UserIDs)
	require.Equal(t, []string{"culture"}, ding.bcGroups)
	require.NotNil(t, repo.saved)
}

func TestService_GroupCard_AIBlurbAndRichDetail(t *testing.T) {
	repo := &memRepo{}
	ding := &fakeDing{}
	s := New(repo, ding).WithLLM(fakeLLM{reply: "快来一起庆祝六一，重拾童心！🎉"})
	now := time.Date(2025, 6, 12, 14, 0, 0, 0, time.UTC)
	_, err := s.Create(context.Background(), CreateCmd{
		TenantID: 1, Title: "六一儿童节", StartAt: now, EndAt: now.Add(2 * time.Hour),
		Location: "线上会议", Detail: "全员参与", GroupIDs: []string{"g1"}, PushGroup: true,
	})
	require.NoError(t, err)
	d := ding.bcCard.Detail
	require.Equal(t, "六一儿童节", ding.bcCard.Title)
	require.Contains(t, d, "快来一起庆祝六一") // AI 润色开场白
	require.Contains(t, d, "时间")
	require.Contains(t, d, "2025-06-12 14:00")
	require.Contains(t, d, "地点")
	require.Contains(t, d, "线上会议")
	require.Contains(t, d, "全员参与") // 原始详情
}

func TestService_Create_PartialOnBotError(t *testing.T) {
	repo := &memRepo{}
	ding := &fakeDing{bcErr: context.DeadlineExceeded}
	s := New(repo, ding)
	now := time.Now()
	out, err := s.Create(context.Background(), CreateCmd{
		TenantID: 1, Title: "x", StartAt: now, EndAt: now.Add(time.Hour),
		GroupIDs: []string{"culture"}, PushGroup: true,
	})
	require.NoError(t, err)
	require.Equal(t, domain.StatusPartial, out.Status)
	require.Contains(t, out.ResultNote, "culture")
}
