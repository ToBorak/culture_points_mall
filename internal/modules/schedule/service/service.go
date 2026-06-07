package service

import (
	"context"
	"strings"
	"time"

	"github.com/standardsoftware/culture_points_mall/internal/modules/schedule/domain"
	"github.com/standardsoftware/culture_points_mall/internal/platform/dingtalk"
	"github.com/standardsoftware/culture_points_mall/internal/platform/llm"
)

// Repo 抽象仓储接口，便于单测用内存实现。*repository.GormRepo 满足它。
type Repo interface {
	Create(ctx context.Context, s *domain.Schedule) error
	ListByTenant(ctx context.Context, tenantID int64, limit int) ([]domain.Schedule, error)
}

type Service struct {
	Repo Repo
	Ding dingtalk.Client
	LLM  llm.Client // 可空：用于群推送卡片的 AI 润色文案
}

func New(repo Repo, ding dingtalk.Client) *Service {
	return &Service{Repo: repo, Ding: ding}
}

// WithLLM 注入 LLM 客户端（群机器人卡片会用它生成一句润色描述）。
func (s *Service) WithLLM(c llm.Client) *Service {
	s.LLM = c
	return s
}

type CreateCmd struct {
	TenantID        int64
	Title           string
	StartAt         time.Time
	EndAt           time.Time
	Location        string
	Detail          string
	AttendeeUserIDs []string
	GroupIDs        []string
	RoomIDs         []string
	PushCalendar    bool
	PushGroup       bool
	CreatedBy       int64
}

func (s *Service) Create(ctx context.Context, cmd CreateCmd) (*domain.Schedule, error) {
	sch := &domain.Schedule{
		TenantID: cmd.TenantID, Title: cmd.Title, StartAt: cmd.StartAt, EndAt: cmd.EndAt,
		Location: cmd.Location, Detail: cmd.Detail, AttendeeUserIDs: cmd.AttendeeUserIDs,
		GroupIDs: cmd.GroupIDs, PushCalendar: cmd.PushCalendar, PushGroup: cmd.PushGroup,
		CreatedBy: cmd.CreatedBy, Status: domain.StatusPublished,
	}
	var notes []string

	if cmd.PushCalendar && len(cmd.AttendeeUserIDs) > 0 {
		eventID, err := s.Ding.CreateCalendarEvent(ctx, dingtalk.CalendarRequest{
			Title: cmd.Title, StartAt: cmd.StartAt, EndAt: cmd.EndAt,
			UserIDs: cmd.AttendeeUserIDs, Location: cmd.Location, Detail: cmd.Detail,
			RoomIDs: cmd.RoomIDs,
		})
		// 事件可能已建成但加会议室失败：此时 eventID 非空且 err 非空，两者都记录。
		if eventID != "" {
			sch.CalendarEventID = eventID
		}
		if err != nil {
			notes = append(notes, "日历失败:"+err.Error())
			sch.Status = domain.StatusPartial
		} else {
			note := "日历OK:" + eventID
			if len(cmd.RoomIDs) > 0 {
				note += " 会议室:" + strings.Join(cmd.RoomIDs, ",")
			}
			notes = append(notes, note)
		}
	}

	if cmd.PushGroup {
		card := dingtalk.Card{Title: cmd.Title, Detail: s.groupCardDetail(ctx, cmd)}
		for _, gid := range cmd.GroupIDs {
			if err := s.Ding.BotBroadcast(ctx, gid, card); err != nil {
				notes = append(notes, "群"+gid+"失败:"+err.Error())
				sch.Status = domain.StatusPartial
			} else {
				notes = append(notes, "群"+gid+"OK")
			}
		}
	}

	sch.ResultNote = strings.Join(notes, "; ")
	if err := s.Repo.Create(ctx, sch); err != nil {
		return nil, err
	}
	return sch, nil
}

func (s *Service) List(ctx context.Context, tenantID int64) ([]domain.Schedule, error) {
	return s.Repo.ListByTenant(ctx, tenantID, 50)
}

// groupCardDetail 组装群机器人卡片正文：AI 润色的一句话开场 + 时间/地点 + 原始详情。
func (s *Service) groupCardDetail(ctx context.Context, cmd CreateCmd) string {
	var b strings.Builder
	if blurb := s.aiBlurb(ctx, cmd); blurb != "" {
		b.WriteString(blurb + "\n\n")
	}
	b.WriteString("> 📅 **时间**：" + cmd.StartAt.Format("2006-01-02 15:04") + " ~ " + cmd.EndAt.Format("15:04") + "\n\n")
	if cmd.Location != "" {
		b.WriteString("> 📍 **地点**：" + cmd.Location + "\n\n")
	}
	if strings.TrimSpace(cmd.Detail) != "" {
		b.WriteString(strings.TrimSpace(cmd.Detail))
	}
	return strings.TrimRight(b.String(), "\n")
}

// aiBlurb 用 LLM 为本次日程/活动写一句热情、吸引人的群通知开场白（无 LLM 或失败时返回空串）。
func (s *Service) aiBlurb(ctx context.Context, cmd CreateCmd) string {
	if s.LLM == nil {
		return ""
	}
	info := "标题：" + cmd.Title + "；时间：" + cmd.StartAt.Format("2006-01-02 15:04")
	if cmd.Location != "" {
		info += "；地点：" + cmd.Location
	}
	if strings.TrimSpace(cmd.Detail) != "" {
		info += "；说明：" + strings.TrimSpace(cmd.Detail)
	}
	resp, err := s.LLM.Messages(ctx, llm.MessagesRequest{
		System: "你在为公司企业文化活动/日程写钉钉群通知的开场白。根据给定信息写一句热情、有感染力、邀请大家参与的话，不超过 40 字，最多 1 个 emoji；不要重复时间和地点（卡片另有字段展示），也不要照抄标题。只输出这句话本身。",
		Messages: []llm.Message{{Role: llm.RoleUser, Content: []llm.Block{{Type: "text", Text: info}}}},
		MaxTokens: 80,
	})
	if err != nil {
		return ""
	}
	var out string
	for _, blk := range resp.Content {
		if blk.Type == "text" {
			out += blk.Text
		}
	}
	return strings.TrimSpace(out)
}
