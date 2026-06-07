//go:build integration

package service_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/standardsoftware/culture_points_mall/internal/modules/publication/domain"
	pubsvc "github.com/standardsoftware/culture_points_mall/internal/modules/publication/service"
)

// 本文件覆盖「读组装」链路（分支最多、风险最高），与同目录 service_integration_test.go
// 共用 TestMain / testDB / newTestSvc / truncateAll / insertUser / insertDimension 样板。
//
// 覆盖：
//   - GetCurrent / GetCurrentPublished —— 取最新已发布并组装
//   - GetDetail —— 不限状态（草稿亦可，admin 预览用）
//   - assemble —— 快照(json.RawMessage)与文章按 section 合并、visible 过滤、sort_order 升序
//   - Publish / UpdatePublicationStatus —— status 翻 published + 写 published_at
//   - ListPublished —— 仅 published 且按 published_at 倒序
//   - UpsertArticle —— 新建(ID==0 默认 draft/manual) 与更新(ID>0) 两条路径

// findSection 在组装结果里按类型定位栏目视图，找不到立即失败。
func findSection(t *testing.T, view *pubsvc.PublishedView, typ domain.SectionType) pubsvc.SectionView {
	t.Helper()
	for _, sv := range view.Sections {
		if sv.Section.Type == typ {
			return sv
		}
	}
	require.FailNowf(t, "组装结果里未找到指定栏目", "type=%s", typ)
	return pubsvc.SectionView{}
}

// TestGetCurrent_ReordersSectionsAndPublishSideEffects 端到端验证读组装主链路，并与
// 同目录 TestGetCurrent_AssemblesView 互补——本用例额外证明：
//   - 配栏目时输入 sort_order 故意乱序（custom=30, invisible=20, star=10），断言 assemble
//     输出按 sort_order 升序「重排」（而非照搬输入序，TestGetCurrent_AssemblesView 的输入本就有序，
//     无法证明重排）。
//   - Aggregate 后快照表恰好 1 行（仅可见 SnapshotBacked 的 star）。
//   - Publish 的落库副作用：直接核对 publications.status='published' 且 published_at 非空。
//   - 快照类/成稿类不交叉污染：star.Articles 为空、custom.Snapshot 为空。
//
// 流程：建期(带 season) → 配栏目(乱序 + 1 不可见 + star + custom) → Aggregate
//
//	→ UpsertArticle(挂到 custom) → Publish → GetCurrent。
func TestGetCurrent_ReordersSectionsAndPublishSideEffects(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	svc := newTestSvc()
	const tenantID = int64(1)

	// ── 季次（star 聚合依赖 season_id）──────────────────────────────────────────
	require.NoError(t, testDB.Exec(
		"INSERT INTO star_seasons (tenant_id, name, quarter_code, status) VALUES (?, ?, ?, 'judging')",
		tenantID, "2026Q2季", "2026Q2",
	).Error)
	var seasonID int64
	require.NoError(t, testDB.Raw("SELECT LAST_INSERT_ID()").Scan(&seasonID).Error)

	// ── 建期（draft，带 season）─────────────────────────────────────────────────
	pub, err := svc.CreateIssue(ctx, pubsvc.CreateIssueCmd{
		TenantID:   tenantID,
		SeasonID:   &seasonID,
		Title:      "组装测试刊",
		PeriodCode: "2026-05",
	})
	require.NoError(t, err)
	require.NotZero(t, pub.ID)

	// ── star 源数据：2 个获奖者 ─────────────────────────────────────────────────
	u1 := insertUser(t, tenantID, "明星甲")
	u2 := insertUser(t, tenantID, "明星乙")
	d1 := insertDimension(t, tenantID, "innovation")
	d2 := insertDimension(t, tenantID, "teamwork")
	require.NoError(t, testDB.Exec(
		"INSERT INTO star_winners (tenant_id, season_id, user_id, dimension_id, citation) VALUES (?,?,?,?,?),(?,?,?,?,?)",
		tenantID, seasonID, u1, d1, "创新先锋",
		tenantID, seasonID, u2, d2, "协作标杆",
	).Error)

	// ── 配栏目：输入故意乱序（custom=30, invisible=20, star=10），且含 1 个不可见栏目 ──
	err = svc.ConfigureSections(ctx, tenantID, pub.ID, []domain.Section{
		{Type: domain.SecCustom, Title: "自定义成稿栏目", SortOrder: 30, Visible: true},
		{Type: domain.SecEditorial, Title: "内部不可见栏目", SortOrder: 20, Visible: false},
		{Type: domain.SecStar, Title: "明星员工", SortOrder: 10, Visible: true},
	})
	require.NoError(t, err)

	// custom 栏目 id（UpsertArticle 需要挂载到具体 section）
	var customSecID int64
	require.NoError(t, testDB.Raw(
		"SELECT id FROM publication_sections WHERE publication_id=? AND type=?",
		pub.ID, domain.SecCustom,
	).Scan(&customSecID).Error)
	require.NotZero(t, customSecID)

	// ── Aggregate：仅可见且 SnapshotBacked 的 star 被写快照（custom 成稿类/不可见栏目跳过）──
	require.NoError(t, svc.Aggregate(ctx, tenantID, pub.ID))
	var snapCount int64
	require.NoError(t, testDB.Raw(
		"SELECT COUNT(*) FROM publication_snapshots WHERE publication_id=?", pub.ID,
	).Scan(&snapCount).Error)
	require.Equal(t, int64(1), snapCount, "只有 star 应产生快照")

	// ── UpsertArticle：挂到 custom 栏目 ─────────────────────────────────────────
	art := &domain.Article{
		PublicationID: &pub.ID,
		SectionID:     &customSecID,
		Title:         "我的自定义文章",
		ContentHTML:   "<p>正文</p>",
	}
	require.NoError(t, svc.UpsertArticle(ctx, tenantID, art))
	require.NotZero(t, art.ID)

	// ── Publish：status 翻 published + 写 published_at ──────────────────────────
	require.NoError(t, svc.Publish(ctx, tenantID, pub.ID))
	var status string
	require.NoError(t, testDB.Raw("SELECT status FROM publications WHERE id=?", pub.ID).Scan(&status).Error)
	require.Equal(t, "published", status)
	var publishedAtNotNull int64
	require.NoError(t, testDB.Raw(
		"SELECT COUNT(*) FROM publications WHERE id=? AND published_at IS NOT NULL", pub.ID,
	).Scan(&publishedAtNotNull).Error)
	require.Equal(t, int64(1), publishedAtNotNull, "Publish 应写入 published_at")

	// ── GetCurrent：组装最新已发布刊物 ──────────────────────────────────────────
	view, err := svc.GetCurrent(ctx, tenantID)
	require.NoError(t, err)
	require.NotNil(t, view)
	require.Equal(t, pub.ID, view.Publication.ID)
	require.Equal(t, domain.PubPublished, view.Publication.Status)

	// 可见栏目共 2 个，按 sort_order 升序：star(10) 在前，custom(30) 在后
	require.Len(t, view.Sections, 2)
	require.Equal(t, domain.SecStar, view.Sections[0].Section.Type)
	require.Equal(t, domain.SecCustom, view.Sections[1].Section.Type)
	require.Less(t, view.Sections[0].Section.SortOrder, view.Sections[1].Section.SortOrder)

	// 不可见栏目被过滤
	for _, sv := range view.Sections {
		require.NotEqual(t, "内部不可见栏目", sv.Section.Title)
		require.NotEqual(t, domain.SecEditorial, sv.Section.Type)
	}

	// star 栏目：带可反序列化的快照、无文章
	starView := findSection(t, view, domain.SecStar)
	require.NotEmpty(t, starView.Snapshot, "star 栏目应带快照")
	var starRows []domain.StarWinnerRow
	require.NoError(t, json.Unmarshal(starView.Snapshot, &starRows), "Snapshot 应可反序列化为 []StarWinnerRow")
	require.Len(t, starRows, 2)
	require.Empty(t, starView.Articles, "快照类栏目不应带文章")

	// custom 栏目：带文章、无快照
	customView := findSection(t, view, domain.SecCustom)
	require.Empty(t, customView.Snapshot, "成稿类栏目不应带快照")
	require.Len(t, customView.Articles, 1)
	require.Equal(t, art.ID, customView.Articles[0].ID)
	require.Equal(t, "我的自定义文章", customView.Articles[0].Title)
}

// TestGetDetail_DraftPreviewAndPublished 验证 GetDetail 不限状态：
//   - 未发布草稿仍能组装（admin 预览用），而同期 GetCurrent 查不到
//   - 发布后 GetDetail 正常，状态变 published
func TestGetDetail_DraftPreviewAndPublished(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	svc := newTestSvc()
	const tenantID = int64(1)

	pub, err := svc.CreateIssue(ctx, pubsvc.CreateIssueCmd{
		TenantID:   tenantID,
		Title:      "草稿预览刊",
		PeriodCode: "2026-07",
	})
	require.NoError(t, err)

	err = svc.ConfigureSections(ctx, tenantID, pub.ID, []domain.Section{
		{Type: domain.SecCustom, Title: "成稿栏目", SortOrder: 1, Visible: true},
	})
	require.NoError(t, err)

	var customSecID int64
	require.NoError(t, testDB.Raw(
		"SELECT id FROM publication_sections WHERE publication_id=? AND type=?",
		pub.ID, domain.SecCustom,
	).Scan(&customSecID).Error)

	art := &domain.Article{
		PublicationID: &pub.ID,
		SectionID:     &customSecID,
		Title:         "草稿文章",
		ContentHTML:   "<p>草稿正文</p>",
	}
	require.NoError(t, svc.UpsertArticle(ctx, tenantID, art))

	// 草稿期：GetCurrent 查不到已发布刊物，但 GetDetail 仍可组装
	_, err = svc.GetCurrent(ctx, tenantID)
	require.Error(t, err, "未发布时 GetCurrent 应查不到已发布刊物")

	draftView, err := svc.GetDetail(ctx, tenantID, pub.ID)
	require.NoError(t, err)
	require.Equal(t, domain.PubDraft, draftView.Publication.Status)
	require.Len(t, draftView.Sections, 1)
	draftSec := findSection(t, draftView, domain.SecCustom)
	require.Len(t, draftSec.Articles, 1)
	require.Equal(t, "草稿文章", draftSec.Articles[0].Title)

	// 发布后：GetDetail 正常，状态 published；GetCurrent 也能查到
	require.NoError(t, svc.Publish(ctx, tenantID, pub.ID))

	pubView, err := svc.GetDetail(ctx, tenantID, pub.ID)
	require.NoError(t, err)
	require.Equal(t, domain.PubPublished, pubView.Publication.Status)
	require.Len(t, pubView.Sections, 1)
	require.Len(t, findSection(t, pubView, domain.SecCustom).Articles, 1)

	cur, err := svc.GetCurrent(ctx, tenantID)
	require.NoError(t, err)
	require.Equal(t, pub.ID, cur.Publication.ID)
}

// TestListPublished_OnlyPublishedOrderedByPublishedAtDesc 验证 ListPublished 只返回
// published 且按 published_at 倒序；并验证 GetCurrent/GetCurrentPublished 取最新一期。
func TestListPublished_OnlyPublishedOrderedByPublishedAtDesc(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	svc := newTestSvc()
	const tenantID = int64(1)

	p1, err := svc.CreateIssue(ctx, pubsvc.CreateIssueCmd{TenantID: tenantID, Title: "一月刊", PeriodCode: "2026-01"})
	require.NoError(t, err)
	p2, err := svc.CreateIssue(ctx, pubsvc.CreateIssueCmd{TenantID: tenantID, Title: "二月刊", PeriodCode: "2026-02"})
	require.NoError(t, err)
	p3, err := svc.CreateIssue(ctx, pubsvc.CreateIssueCmd{TenantID: tenantID, Title: "草稿刊", PeriodCode: "2026-03"})
	require.NoError(t, err)

	// 发布 p1、p2；p3 保持草稿
	require.NoError(t, svc.Publish(ctx, tenantID, p1.ID))
	require.NoError(t, svc.Publish(ctx, tenantID, p2.ID))

	// 显式拉开 published_at，保证倒序断言确定（TIMESTAMP 秒级精度，svc.Publish 同秒会并列）
	require.NoError(t, testDB.Exec("UPDATE publications SET published_at=? WHERE id=?", "2026-05-01 10:00:00", p1.ID).Error)
	require.NoError(t, testDB.Exec("UPDATE publications SET published_at=? WHERE id=?", "2026-06-01 10:00:00", p2.ID).Error)

	list, err := svc.ListPublished(ctx, tenantID)
	require.NoError(t, err)
	require.Len(t, list, 2, "只返回 2 个已发布，草稿不计入")

	// published_at 倒序：p2(6 月) 在前，p1(5 月) 在后
	require.Equal(t, p2.ID, list[0].ID)
	require.Equal(t, p1.ID, list[1].ID)
	for _, p := range list {
		require.Equal(t, domain.PubPublished, p.Status)
		require.NotEqual(t, p3.ID, p.ID, "草稿刊不应出现在已发布列表")
	}

	// GetCurrent 取最新一期（GetCurrentPublished 按 published_at DESC 取首条）
	cur, err := svc.GetCurrent(ctx, tenantID)
	require.NoError(t, err)
	require.Equal(t, p2.ID, cur.Publication.ID)
}

// TestUpsertArticle_CreateDefaultsThenUpdate 验证 UpsertArticle 两条路径：
//   - 新建(ID==0)：service 默认填充 status=draft、source_type=manual、tenant_id，并回填自增 ID
//   - 更新(ID>0)：改标题 + 状态走 UpdateArticle，不新增行、不改 source_type
func TestUpsertArticle_CreateDefaultsThenUpdate(t *testing.T) {
	truncateAll(t)
	ctx := context.Background()
	svc := newTestSvc()
	const tenantID = int64(1)

	// ── 新建（ID==0）──────────────────────────────────────────────────────────
	art := &domain.Article{
		Title:       "初稿",
		ContentHTML: "<p>初稿正文</p>",
	}
	require.NoError(t, svc.UpsertArticle(ctx, tenantID, art))
	require.NotZero(t, art.ID, "新建后应回填自增 ID")
	require.Equal(t, domain.ArticleDraft, art.Status, "新建默认 status=draft")
	require.Equal(t, domain.ArticleManual, art.SourceType, "新建默认 source_type=manual")
	require.Equal(t, tenantID, art.TenantID, "新建应回填 tenant_id")

	var created struct {
		Status     string
		SourceType string
		Title      string
		TenantID   int64
	}
	require.NoError(t, testDB.Raw(
		"SELECT status, source_type, title, tenant_id FROM publication_articles WHERE id=?", art.ID,
	).Scan(&created).Error)
	require.Equal(t, "draft", created.Status)
	require.Equal(t, "manual", created.SourceType)
	require.Equal(t, "初稿", created.Title)
	require.Equal(t, tenantID, created.TenantID)

	// ── 更新（ID>0）：改标题 + 状态 ─────────────────────────────────────────────
	art.Title = "终稿"
	art.Status = domain.ArticlePublished
	require.NoError(t, svc.UpsertArticle(ctx, tenantID, art))

	var updated struct {
		Status     string
		SourceType string
		Title      string
	}
	require.NoError(t, testDB.Raw(
		"SELECT status, source_type, title FROM publication_articles WHERE id=?", art.ID,
	).Scan(&updated).Error)
	require.Equal(t, "published", updated.Status)
	require.Equal(t, "终稿", updated.Title)
	require.Equal(t, "manual", updated.SourceType, "UpdateArticle 不改 source_type")

	// 更新走 UPDATE 而非 INSERT，仍只有 1 行
	var cnt int64
	require.NoError(t, testDB.Raw("SELECT COUNT(*) FROM publication_articles").Scan(&cnt).Error)
	require.Equal(t, int64(1), cnt)
}
