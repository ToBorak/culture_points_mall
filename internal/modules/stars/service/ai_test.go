package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/standardsoftware/culture_points_mall/internal/modules/stars/domain"
	"github.com/standardsoftware/culture_points_mall/internal/platform/llm"
)

// ─── fake LLM ─────────────────────────────────────────────────────────────────

type fakeLLM struct{ reply string }

func (f *fakeLLM) Messages(_ context.Context, _ llm.MessagesRequest) (llm.MessagesResponse, error) {
	return llm.MessagesResponse{Content: []llm.Block{{Type: "text", Text: f.reply}}}, nil
}
func (f *fakeLLM) MessagesStream(_ context.Context, _ llm.MessagesRequest) (<-chan llm.StreamEvent, error) {
	return nil, nil
}

// ─── fake repo（嵌入接口，只覆写被调方法）─────────────────────────────────────

type fakeStarsRepo struct {
	domain.Repository
	// ListNominationsBySeason 返回值
	listNoms []domain.Nomination
	listErr  error
	// UpdateNominationRefined 记录
	refinedID   int64
	refinedText string
	refinedTags string
	updateErr   error
}

func (r *fakeStarsRepo) ListNominationsBySeason(_ context.Context, _, _ int64) ([]domain.Nomination, error) {
	return r.listNoms, r.listErr
}

func (r *fakeStarsRepo) UpdateNominationRefined(_ context.Context, _, id int64, refined string, tags string) error {
	r.refinedID = id
	r.refinedText = refined
	r.refinedTags = tags
	return r.updateErr
}

// ─── 辅助：建最小 Service（不注入 Points/Cfg，单测 AI 方法不走积分路径）────────

func newAITestSvc(repo domain.Repository, lc llm.Client) *Service {
	return &Service{Repo: repo, LLM: lc}
}

// ─── DraftCase ────────────────────────────────────────────────────────────────

func TestDraftCase_LLMNil_ReturnsErrUnavailable(t *testing.T) {
	svc := newAITestSvc(&fakeStarsRepo{}, nil)
	_, err := svc.DraftCase(context.Background(), "诚信", "按时交付项目")
	if !errors.Is(err, ErrLLMUnavailable) {
		t.Fatalf("expected ErrLLMUnavailable, got %v", err)
	}
}

func TestDraftCase_HasLLM_ReturnsText(t *testing.T) {
	want := "他在项目关键时刻坚守承诺，按时交付了全部功能。"
	svc := newAITestSvc(&fakeStarsRepo{}, &fakeLLM{reply: "  " + want + "  "})
	got, err := svc.DraftCase(context.Background(), "诚信", "按时交付项目")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

// ─── JudgeDigest ──────────────────────────────────────────────────────────────

func TestJudgeDigest_LLMNil_ReturnsErrUnavailable(t *testing.T) {
	svc := newAITestSvc(&fakeStarsRepo{}, nil)
	_, err := svc.JudgeDigest(context.Background(), 1, 1)
	if !errors.Is(err, ErrLLMUnavailable) {
		t.Fatalf("expected ErrLLMUnavailable, got %v", err)
	}
}

func TestJudgeDigest_ParsesFakeJSON(t *testing.T) {
	fakeJSON := `{"summary":"本季共 3 份提名，覆盖创新/协作两个维度","duplicates":["提名#1 与 #3 疑似同一事迹"]}`
	repo := &fakeStarsRepo{
		listNoms: []domain.Nomination{
			{ID: 1, NomineeID: 10, DimensionID: 1, CaseText: "abc"},
			{ID: 3, NomineeID: 11, DimensionID: 1, CaseText: "def"},
		},
	}
	svc := newAITestSvc(repo, &fakeLLM{reply: fakeJSON})
	d, err := svc.JudgeDigest(context.Background(), 1, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Summary == "" {
		t.Fatal("expected non-empty summary")
	}
	if len(d.Duplicates) == 0 {
		t.Fatal("expected at least one duplicate entry")
	}
	if d.GeneratedAt.IsZero() {
		t.Fatal("expected GeneratedAt to be set")
	}
	// 时间误差不超过 5 秒
	if time.Since(d.GeneratedAt) > 5*time.Second {
		t.Fatalf("GeneratedAt too old: %v", d.GeneratedAt)
	}
}

// ─── refineNomination ─────────────────────────────────────────────────────────

func TestRefineNomination_CallsUpdateWithCorrectArgs(t *testing.T) {
	fakeJSON := `{"refined":"他凭借出色的创新思维，在短时间内完成了系统重构。","tags":["勇于创新","高效执行"]}`
	repo := &fakeStarsRepo{}
	svc := newAITestSvc(repo, &fakeLLM{reply: fakeJSON})

	nom := &domain.Nomination{ID: 42, CaseText: "他做了一件很棒的事"}
	svc.refineNomination(context.Background(), 1, nom)

	if repo.refinedID != 42 {
		t.Fatalf("expected UpdateNominationRefined called with id=42, got id=%d", repo.refinedID)
	}
	if repo.refinedText == "" {
		t.Fatal("expected non-empty refined text")
	}
	if repo.refinedTags == "" {
		t.Fatal("expected non-empty tags JSON")
	}
}

func TestRefineNomination_InvalidJSON_DoesNotCallUpdate(t *testing.T) {
	repo := &fakeStarsRepo{}
	svc := newAITestSvc(repo, &fakeLLM{reply: "不是 json"})

	nom := &domain.Nomination{ID: 99, CaseText: "某事"}
	svc.refineNomination(context.Background(), 1, nom)

	// refined 字段为空表示 Update 未被调用（refinedID 保持零值）
	if repo.refinedID != 0 {
		t.Fatalf("expected no Update call, but got refinedID=%d", repo.refinedID)
	}
}
