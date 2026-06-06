package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/standardsoftware/culture_points_mall/internal/config"
	"github.com/standardsoftware/culture_points_mall/internal/platform/dingtalk"
)

type rejectingDingClient struct {
	called bool
}

func (f *rejectingDingClient) GetUserByCode(context.Context, string) (dingtalk.User, error) {
	f.called = true
	return dingtalk.User{}, context.Canceled
}

func (f *rejectingDingClient) CreateCalendarEvent(context.Context, dingtalk.CalendarRequest) (string, error) {
	return "", nil
}
func (f *rejectingDingClient) ListCalendarResponses(context.Context, string) ([]dingtalk.Response, error) {
	return nil, nil
}
func (f *rejectingDingClient) QueryMeetingRooms(context.Context, string) ([]dingtalk.MeetingRoom, error) {
	return nil, nil
}
func (f *rejectingDingClient) SendWorkNotice(context.Context, []string, dingtalk.Card) error {
	return nil
}
func (f *rejectingDingClient) SendInteractiveCard(context.Context, string, string, map[string]any) (dingtalk.CardInstance, error) {
	return dingtalk.CardInstance{}, nil
}
func (f *rejectingDingClient) BotBroadcast(context.Context, string, dingtalk.Card) error { return nil }
func (f *rejectingDingClient) StartOAProcess(context.Context, dingtalk.ApprovalRequest) (string, error) {
	return "", nil
}

func TestDingLogin_RejectsRealAuthCodeInMockMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ding := &rejectingDingClient{}
	cfg := &config.Config{}
	cfg.DingTalk.Mode = "mock"
	cfg.JWT.Secret = "test"
	cfg.JWT.TTLHours = 1
	h := NewHandler(nil, cfg, ding)
	r := gin.New()
	h.Register(r.Group("/"))
	srv := httptest.NewServer(r)
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{"code": "real-dingtalk-auth-code"})
	resp, err := http.Post(srv.URL+"/auth/dingtalk/login", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	require.Equal(t, http.StatusConflict, resp.StatusCode)
	require.False(t, ding.called)
}
