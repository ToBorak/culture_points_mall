package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/standardsoftware/culture_points_mall/internal/modules/publication/domain"
	"github.com/standardsoftware/culture_points_mall/internal/platform/dingtalk"
	"github.com/standardsoftware/culture_points_mall/internal/platform/llm"
)

// ─── fake LLM ─────────────────────────────────────────────────────────────────

type fakeLLMPub struct{ reply string }

func (f *fakeLLMPub) Messages(_ context.Context, _ llm.MessagesRequest) (llm.MessagesResponse, error) {
	return llm.MessagesResponse{Content: []llm.Block{{Type: "text", Text: f.reply}}}, nil
}
func (f *fakeLLMPub) MessagesStream(_ context.Context, _ llm.MessagesRequest) (<-chan llm.StreamEvent, error) {
	return nil, nil
}

// ─── fake dingtalk ────────────────────────────────────────────────────────────

type fakeDing struct {
	calledGroupID string
	calledCard    dingtalk.Card
}

func (d *fakeDing) GetUserByCode(_ context.Context, _ string) (dingtalk.User, error) {
	return dingtalk.User{}, nil
}
func (d *fakeDing) CreateCalendarEvent(_ context.Context, _ dingtalk.CalendarRequest) (string, error) {
	return "", nil
}
func (d *fakeDing) ListCalendarResponses(_ context.Context, _ string) ([]dingtalk.Response, error) {
	return nil, nil
}
func (d *fakeDing) QueryMeetingRooms(_ context.Context, _ string) ([]dingtalk.MeetingRoom, error) {
	return nil, nil
}
func (d *fakeDing) SendWorkNotice(_ context.Context, _ []string, _ dingtalk.Card) error {
	return nil
}
func (d *fakeDing) SendInteractiveCard(_ context.Context, _, _ string, _ map[string]any) (dingtalk.CardInstance, error) {
	return dingtalk.CardInstance{}, nil
}
func (d *fakeDing) BotBroadcast(_ context.Context, groupID string, msg dingtalk.Card) error {
	d.calledGroupID = groupID
	d.calledCard = msg
	return nil
}
func (d *fakeDing) StartOAProcess(_ context.Context, _ dingtalk.ApprovalRequest) (string, error) {
	return "", nil
}

// ─── fake repo（嵌入接口，只覆写被调方法）─────────────────────────────────────

type fakePubRepo struct {
	domain.Repository
	pub                *domain.Publication
	sections           []domain.Section
	snaps              []domain.Snapshot
	noms               []domain.SelectedNominationRow
	dims               []domain.ValueRow
	existsArticle      bool
	createdArticles    []*domain.Article
	introText          string
	updatedSectionCopy map[int64]string
}

func newFakePubRepo() *fakePubRepo {
	return &fakePubRepo{
		updatedSectionCopy: make(map[int64]string),
	}
}

func (r *fakePubRepo) GetPublication(_ context.Context, _, _ int64) (*domain.Publication, error) {
	if r.pub == nil {
		return nil, errors.New("not found")
	}
	return r.pub, nil
}

func (r *fakePubRepo) ListSections(_ context.Context, _ int64) ([]domain.Section, error) {
	return r.sections, nil
}

func (r *fakePubRepo) ListSnapshots(_ context.Context, _ int64) ([]domain.Snapshot, error) {
	return r.snaps, nil
}

func (r *fakePubRepo) UpdatePublicationIntro(_ context.Context, _, _ int64, intro string) error {
	r.introText = intro
	return nil
}

func (r *fakePubRepo) UpdateSectionAICopy(_ context.Context, sectionID int64, aiCopy string) error {
	r.updatedSectionCopy[sectionID] = aiCopy
	return nil
}

func (r *fakePubRepo) ListSelectedNominations(_ context.Context, _, _ int64) ([]domain.SelectedNominationRow, error) {
	return r.noms, nil
}

func (r *fakePubRepo) ExistsArticleFromNomination(_ context.Context, _, _, _ int64) (bool, error) {
	return r.existsArticle, nil
}

func (r *fakePubRepo) CreateArticle(_ context.Context, a *domain.Article) error {
	r.createdArticles = append(r.createdArticles, a)
	return nil
}

func (r *fakePubRepo) ListValueDimensions(_ context.Context, _ int64) ([]domain.ValueRow, error) {
	return r.dims, nil
}

// ─── 辅助：建 Service ─────────────────────────────────────────────────────────

func newPubAITestSvc(repo domain.Repository, lc llm.Client, ding dingtalk.Client) *Service {
	return &Service{Repo: repo, LLM: lc, Ding: ding}
}

// ─── Compose ─────────────────────────────────────────────────────────────────

func TestCompose_LLMNil_ReturnsErrUnavailable(t *testing.T) {
	svc := newPubAITestSvc(newFakePubRepo(), nil, nil)
	err := svc.Compose(context.Background(), 1, 1)
	if !errors.Is(err, ErrLLMUnavailable) {
		t.Fatalf("expected ErrLLMUnavailable, got %v", err)
	}
}

func TestCompose_HasLLM_CallsUpdateIntroAndSectionCopy(t *testing.T) {
	intro := "温暖有力的刊首语正文。"
	repo := newFakePubRepo()
	repo.pub = &domain.Publication{ID: 1, TenantID: 1, Title: "2026年5月刊"}
	repo.sections = []domain.Section{
		{ID: 10, Type: domain.SecStar, Title: "明星员工", Visible: true},
		{ID: 11, Type: domain.SecCustom, Title: "自定义", Visible: true},
		{ID: 12, Type: domain.SecStar, Title: "不可见栏目", Visible: false},
	}
	repo.snaps = []domain.Snapshot{
		{SectionID: 10, DataJSON: `[{"userId":1}]`},
	}

	svc := newPubAITestSvc(repo, &fakeLLMPub{reply: intro}, nil)
	err := svc.Compose(context.Background(), 1, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.introText == "" {
		t.Fatal("expected UpdatePublicationIntro to be called")
	}
	// 两个 visible 栏目都应有 ai_copy
	if len(repo.updatedSectionCopy) < 2 {
		t.Fatalf("expected UpdateSectionAICopy for 2 visible sections, got %d", len(repo.updatedSectionCopy))
	}
	if _, ok := repo.updatedSectionCopy[10]; !ok {
		t.Fatal("expected UpdateSectionAICopy called for section 10")
	}
	if _, ok := repo.updatedSectionCopy[11]; !ok {
		t.Fatal("expected UpdateSectionAICopy called for section 11")
	}
	// 不可见栏目不应被调
	if _, ok := repo.updatedSectionCopy[12]; ok {
		t.Fatal("expected UpdateSectionAICopy NOT called for invisible section 12")
	}
}

// ─── CultureQA ────────────────────────────────────────────────────────────────

func TestCultureQA_LLMNil_ReturnsErrUnavailable(t *testing.T) {
	svc := newPubAITestSvc(newFakePubRepo(), nil, nil)
	_, err := svc.CultureQA(context.Background(), 1, "什么是诚信？")
	if !errors.Is(err, ErrLLMUnavailable) {
		t.Fatalf("expected ErrLLMUnavailable, got %v", err)
	}
}

func TestCultureQA_HasLLM_ReturnsAnswer(t *testing.T) {
	want := "诚信是我们的核心价值观，体现在言行一致、兑现承诺。"
	repo := newFakePubRepo()
	repo.dims = []domain.ValueRow{
		{DimensionID: 1, Name: "诚信", Description: "言行一致"},
	}
	svc := newPubAITestSvc(repo, &fakeLLMPub{reply: "  " + want + "  "}, nil)
	got, err := svc.CultureQA(context.Background(), 1, "什么是诚信？")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

// ─── GenerateCaseArticles ──────────────────────────────────────────────────────

func TestGenerateCaseArticles_LLMNil_ReturnsErrUnavailable(t *testing.T) {
	svc := newPubAITestSvc(newFakePubRepo(), nil, nil)
	_, err := svc.GenerateCaseArticles(context.Background(), 1, 1)
	if !errors.Is(err, ErrLLMUnavailable) {
		t.Fatalf("expected ErrLLMUnavailable, got %v", err)
	}
}

func TestGenerateCaseArticles_AlreadyExists_IsIdempotent(t *testing.T) {
	seasonID := int64(5)
	repo := newFakePubRepo()
	repo.pub = &domain.Publication{ID: 1, TenantID: 1, Title: "刊", SeasonID: &seasonID}
	repo.noms = []domain.SelectedNominationRow{
		{NominationID: 100, NomineeName: "张三", Dimension: "创新", DimensionID: 1, CaseText: "abc", CaseRefined: "已提炼"},
	}
	repo.existsArticle = true // 已存在，不应重建

	svc := newPubAITestSvc(repo, &fakeLLMPub{reply: "文章正文"}, nil)
	n, err := svc.GenerateCaseArticles(context.Background(), 1, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 new articles (idempotent), got %d", n)
	}
	if len(repo.createdArticles) != 0 {
		t.Fatalf("expected CreateArticle not called, got %d calls", len(repo.createdArticles))
	}
}

func TestGenerateCaseArticles_NewNomination_CreatesArticle(t *testing.T) {
	seasonID := int64(5)
	repo := newFakePubRepo()
	repo.pub = &domain.Publication{ID: 1, TenantID: 1, Title: "刊", SeasonID: &seasonID}
	repo.noms = []domain.SelectedNominationRow{
		{NominationID: 101, NomineeName: "李四", Dimension: "协作", DimensionID: 2, CaseText: "xyz", CaseRefined: "已提炼内容"},
	}
	repo.existsArticle = false

	svc := newPubAITestSvc(repo, &fakeLLMPub{reply: "文章正文"}, nil)
	n, err := svc.GenerateCaseArticles(context.Background(), 1, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 new article, got %d", n)
	}
	if len(repo.createdArticles) != 1 {
		t.Fatalf("expected CreateArticle called once, got %d", len(repo.createdArticles))
	}
}

// ─── PushDingtalk ─────────────────────────────────────────────────────────────

func TestPushDingtalk_CallsBotBroadcast_WithPubTitle(t *testing.T) {
	title := "2026年5月刊"
	intro := "本期刊首语正文。"
	repo := newFakePubRepo()
	repo.pub = &domain.Publication{
		ID:         1,
		TenantID:   1,
		Title:      title,
		PeriodCode: "2026-05",
		IntroText:  &intro,
	}
	ding := &fakeDing{}
	svc := newPubAITestSvc(repo, nil, ding)

	err := svc.PushDingtalk(context.Background(), 1, 1, "group-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ding.calledGroupID != "group-abc" {
		t.Fatalf("expected groupID=group-abc, got %q", ding.calledGroupID)
	}
	// Card.Title 应含刊名
	if len(ding.calledCard.Title) == 0 {
		t.Fatal("expected non-empty card title")
	}
	// 确认刊名出现在标题中
	if !containsStr(ding.calledCard.Title, title) {
		t.Fatalf("expected card title to contain %q, got %q", title, ding.calledCard.Title)
	}
}

func TestPushDingtalk_NoIntro_UsesFallbackDetail(t *testing.T) {
	repo := newFakePubRepo()
	repo.pub = &domain.Publication{
		ID:         2,
		TenantID:   1,
		Title:      "无刊首语测试刊",
		PeriodCode: "2026-06",
		IntroText:  nil, // 没有刊首语
	}
	ding := &fakeDing{}
	svc := newPubAITestSvc(repo, nil, ding)

	err := svc.PushDingtalk(context.Background(), 1, 2, "group-xyz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ding.calledCard.Detail == "" {
		t.Fatal("expected non-empty card detail (fallback)")
	}
}

// ─── helper ───────────────────────────────────────────────────────────────────

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && findSubstr(s, substr))
}

func findSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// 确保 time 包被使用
var _ = time.Now
