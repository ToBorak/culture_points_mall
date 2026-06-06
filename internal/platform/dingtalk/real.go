package dingtalk

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/standardsoftware/culture_points_mall/internal/config"
)

// 编译期接口满足自检。
var _ Client = (*RealClient)(nil)

var errNotImplemented = errors.New("dingtalk: real client method not implemented in this phase")

// RealClient 调用钉钉真实 OpenAPI。本期只做实 GetUserByCode，其余 Phase 3 填实。
type RealClient struct {
	api    *caller
	tokens *tokenManager
	cfg    config.DingTalkCfg
}

// NewReal 构造 RealClient。api 指针被 tokenManager 复用，测试改 oapiBase/apiBase 对两者同时生效。
func NewReal(cfg config.DingTalkCfg, rdb *redis.Client) *RealClient {
	api := newCaller()
	return &RealClient{
		api:    api,
		tokens: &tokenManager{api: api, rdb: rdb, appKey: cfg.AppKey, appSecret: cfg.AppSecret},
		cfg:    cfg,
	}
}

func (c *RealClient) GetUserByCode(ctx context.Context, code string) (User, error) {
	tok, err := c.tokens.corpToken(ctx)
	if err != nil {
		return User{}, err
	}

	// 第一步：用免登 code 换 userid / unionid / sys
	var gi struct {
		Result struct {
			UserID  string `json:"userid"`
			UnionID string `json:"unionid"`
			Sys     bool   `json:"sys"`
		} `json:"result"`
	}
	if err := c.api.oapiPost(ctx, "/topapi/v2/user/getuserinfo", tok, map[string]any{"code": code}, &gi); err != nil {
		return User{}, err
	}
	if gi.Result.UserID == "" {
		return User{}, errors.New("dingtalk: empty userid from getuserinfo")
	}

	// 第二步：用 userid 拉用户详情
	var ug struct {
		Result struct {
			UserID     string  `json:"userid"`
			UnionID    string  `json:"unionid"`
			Name       string  `json:"name"`
			Avatar     string  `json:"avatar"`
			DeptIDList []int64 `json:"dept_id_list"`
		} `json:"result"`
	}
	if err := c.api.oapiPost(ctx, "/topapi/v2/user/get", tok, map[string]any{"userid": gi.Result.UserID, "language": "zh_CN"}, &ug); err != nil {
		return User{}, err
	}

	// unionid 以第一步为准，第二步做兜底
	union := gi.Result.UnionID
	if union == "" {
		union = ug.Result.UnionID
	}

	return User{
		DingUserID: gi.Result.UserID,
		Name:       ug.Result.Name,
		AvatarURL:  ug.Result.Avatar,
		DeptIDs:    ug.Result.DeptIDList,
		UnionID:    union,
		IsAdmin:    gi.Result.Sys,
	}, nil
}

// 以下方法 Phase 3 填实。

// unionIDByUserID 用 corp userid 拉 unionId（日历接口路径与参与人都要 unionId）。
func (c *RealClient) unionIDByUserID(ctx context.Context, token, userid string) (string, error) {
	var ug struct {
		Result struct {
			UnionID string `json:"unionid"`
		} `json:"result"`
	}
	if err := c.api.oapiPost(ctx, "/topapi/v2/user/get", token, map[string]any{"userid": userid, "language": "zh_CN"}, &ug); err != nil {
		return "", err
	}
	if ug.Result.UnionID == "" {
		return "", fmt.Errorf("dingtalk: empty unionId for userid %s", userid)
	}
	return ug.Result.UnionID, nil
}

// CreateCalendarEvent 在组织者的主日历上创建日程，参与人会在各自钉钉日历里看到。
// 组织者默认取 cfg.CalendarOrganizerUnionID，否则用参与人列表第一个。
func (c *RealClient) CreateCalendarEvent(ctx context.Context, req CalendarRequest) (string, error) {
	tok, err := c.tokens.corpToken(ctx)
	if err != nil {
		return "", err
	}

	attendees := make([]map[string]string, 0, len(req.UserIDs))
	organizer := c.cfg.CalendarOrganizerUnionID
	for _, uid := range req.UserIDs {
		union, err := c.unionIDByUserID(ctx, tok, uid)
		if err != nil {
			return "", fmt.Errorf("resolve unionId for %s: %w", uid, err)
		}
		if organizer == "" {
			organizer = union
		}
		attendees = append(attendees, map[string]string{"id": union})
	}
	if organizer == "" {
		return "", errors.New("dingtalk: no organizer unionId for calendar event")
	}

	body := map[string]any{
		"summary":     req.Title,
		"description": req.Detail,
		"start":       map[string]string{"dateTime": req.StartAt.Format(time.RFC3339), "timeZone": "Asia/Shanghai"},
		"end":         map[string]string{"dateTime": req.EndAt.Format(time.RFC3339), "timeZone": "Asia/Shanghai"},
		"attendees":   attendees,
	}
	if req.Location != "" {
		body["location"] = map[string]string{"displayName": req.Location}
	}

	var out struct {
		ID string `json:"id"`
	}
	path := "/v1.0/calendar/users/" + organizer + "/calendars/primary/events"
	if err := c.api.apiPost(ctx, path, tok, body, &out); err != nil {
		return "", err
	}
	return out.ID, nil
}

func (c *RealClient) ListCalendarResponses(_ context.Context, _ string) ([]Response, error) {
	return nil, errNotImplemented
}

func (c *RealClient) SendWorkNotice(_ context.Context, _ []string, _ Card) error {
	return errNotImplemented
}

func (c *RealClient) SendInteractiveCard(_ context.Context, _, _ string, _ map[string]any) (CardInstance, error) {
	return CardInstance{}, errNotImplemented
}

func (c *RealClient) BotBroadcast(_ context.Context, _ string, _ Card) error {
	return errNotImplemented
}

func (c *RealClient) StartOAProcess(_ context.Context, _ ApprovalRequest) (string, error) {
	return "", errNotImplemented
}
