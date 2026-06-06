package service

import (
	"context"
	"strings"
	"time"

	"github.com/standardsoftware/culture_points_mall/internal/modules/schedule/domain"
	"github.com/standardsoftware/culture_points_mall/internal/platform/dingtalk"
)

// Repo 抽象仓储接口，便于单测用内存实现。*repository.GormRepo 满足它。
type Repo interface {
	Create(ctx context.Context, s *domain.Schedule) error
	ListByTenant(ctx context.Context, tenantID int64, limit int) ([]domain.Schedule, error)
}

type Service struct {
	Repo Repo
	Ding dingtalk.Client
}

func New(repo Repo, ding dingtalk.Client) *Service {
	return &Service{Repo: repo, Ding: ding}
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
		})
		if err != nil {
			notes = append(notes, "日历失败:"+err.Error())
			sch.Status = domain.StatusPartial
		} else {
			sch.CalendarEventID = eventID
			notes = append(notes, "日历OK:"+eventID)
		}
	}

	if cmd.PushGroup {
		card := dingtalk.Card{Title: cmd.Title, Detail: scheduleMarkdown(cmd)}
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

func scheduleMarkdown(cmd CreateCmd) string {
	var b strings.Builder
	b.WriteString("**时间**：" + cmd.StartAt.Format("2006-01-02 15:04") + " ~ " + cmd.EndAt.Format("15:04") + "\n\n")
	if cmd.Location != "" {
		b.WriteString("**地点**：" + cmd.Location + "\n\n")
	}
	if cmd.Detail != "" {
		b.WriteString(cmd.Detail)
	}
	return b.String()
}
