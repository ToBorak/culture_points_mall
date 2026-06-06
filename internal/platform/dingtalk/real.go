package dingtalk

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
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

	// 指定了会议室：建完事件再把会议室加到事件上（钉钉「预定会议室」就是这一步），
	// 钉钉会按 roomId 自动把会议室名称回填到日程里显示。
	if len(req.RoomIDs) > 0 {
		if err := c.addMeetingRooms(ctx, tok, organizer, out.ID, req.RoomIDs); err != nil {
			// 事件已建成功，连同 eventID 一起返回，让上层既能记录事件又能看到加会议室失败。
			return out.ID, fmt.Errorf("event %s created but add meeting rooms failed: %w", out.ID, err)
		}
	}
	return out.ID, nil
}

// addMeetingRooms 把会议室加到已建好的日程事件上。
// POST /v1.0/calendar/users/{organizerUnionId}/calendars/primary/events/{eventId}/meetingRooms
func (c *RealClient) addMeetingRooms(ctx context.Context, token, organizerUnionID, eventID string, roomIDs []string) error {
	items := make([]map[string]string, 0, len(roomIDs))
	for _, rid := range roomIDs {
		items = append(items, map[string]string{"roomId": rid})
	}
	var out struct {
		Result bool `json:"result"`
	}
	path := "/v1.0/calendar/users/" + organizerUnionID + "/calendars/primary/events/" + eventID + "/meetingRooms"
	if err := c.api.apiPost(ctx, path, token, map[string]any{"meetingRoomsToAdd": items}, &out); err != nil {
		return err
	}
	if !out.Result {
		return errors.New("dingtalk: add meeting rooms returned result=false")
	}
	return nil
}

// QueryMeetingRooms 列出 unionID 可见的智能会议室。
// GET /v1.0/rooms/meetingRoomLists?unionId=&maxResults=（参数走 query）。
func (c *RealClient) QueryMeetingRooms(ctx context.Context, unionID string) ([]MeetingRoom, error) {
	tok, err := c.tokens.corpToken(ctx)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("unionId", unionID)
	q.Set("maxResults", "100")

	var out struct {
		Result []struct {
			RoomID       string `json:"roomId"`
			RoomName     string `json:"roomName"`
			RoomCapacity int    `json:"roomCapacity"`
			RoomStatus   int    `json:"roomStatus"`
			RoomLocation struct {
				Title string `json:"title"`
				Desc  string `json:"desc"`
			} `json:"roomLocation"`
		} `json:"result"`
	}
	if err := c.api.apiGet(ctx, "/v1.0/rooms/meetingRoomLists", tok, q, &out); err != nil {
		return nil, err
	}
	rooms := make([]MeetingRoom, 0, len(out.Result))
	for _, r := range out.Result {
		rooms = append(rooms, MeetingRoom{
			RoomID:   r.RoomID,
			RoomName: r.RoomName,
			Capacity: r.RoomCapacity,
			Status:   r.RoomStatus,
			Location: strings.TrimSpace(r.RoomLocation.Title + " " + r.RoomLocation.Desc),
		})
	}
	return rooms, nil
}

// ResolveUnionID 用 corp userid 换 unionId。供 cmd 手验脚本使用；
// 后台接口走 users 表里登录时落的 union_id，不用这个。
func (c *RealClient) ResolveUnionID(ctx context.Context, userid string) (string, error) {
	tok, err := c.tokens.corpToken(ctx)
	if err != nil {
		return "", err
	}
	return c.unionIDByUserID(ctx, tok, userid)
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

// BotBroadcast 通过群里的「自定义机器人」Webhook（加签）推一条 markdown 消息。
// groupID 对应 config.dingtalk.robots[].id。
func (c *RealClient) BotBroadcast(ctx context.Context, groupID string, msg Card) error {
	var robot *config.RobotCfg
	for i := range c.cfg.Robots {
		if c.cfg.Robots[i].ID == groupID {
			robot = &c.cfg.Robots[i]
			break
		}
	}
	if robot == nil {
		return fmt.Errorf("dingtalk: 未找到 groupID=%q 对应的群机器人配置", groupID)
	}

	// 加签：sign = base64(HmacSHA256(secret, "{timestamp}\n{secret}"))，再 urlEncode
	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	mac := hmac.New(sha256.New, []byte(robot.Secret))
	mac.Write([]byte(ts + "\n" + robot.Secret))
	sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	signedURL := robot.Webhook + "&timestamp=" + ts + "&sign=" + url.QueryEscape(sign)

	title := msg.Title
	if title == "" {
		title = "通知"
	}
	text := msg.Detail
	if msg.Title != "" {
		text = "### " + msg.Title + "\n\n" + msg.Detail
	}
	// 按钮跳转 URL：优先 Card.Extra["url"]，否则默认打开钉钉
	btnURL := "https://www.dingtalk.com"
	if msg.Extra != nil {
		if u, ok := msg.Extra["url"].(string); ok && u != "" {
			btnURL = u
		}
	}
	body := map[string]any{
		"msgtype": "actionCard",
		"actionCard": map[string]any{
			"title":       title,
			"text":        text,
			"singleTitle": "查看详情",
			"singleURL":   btnURL,
		},
	}

	raw, err := c.api.do(ctx, signedURL, "", body)
	if err != nil {
		return err
	}
	var env struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("dingtalk robot send: invalid json response: %w", err)
	}
	if env.ErrCode != 0 {
		return fmt.Errorf("dingtalk robot send errcode=%d errmsg=%s", env.ErrCode, env.ErrMsg)
	}
	return nil
}

func (c *RealClient) StartOAProcess(_ context.Context, _ ApprovalRequest) (string, error) {
	return "", errNotImplemented
}
