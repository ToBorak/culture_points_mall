//go:build integration

package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/standardsoftware/culture_points_mall/internal/config"
	"github.com/standardsoftware/culture_points_mall/internal/migrate"
	"github.com/standardsoftware/culture_points_mall/internal/platform/dingtalk"
)

var authDB *gorm.DB

func TestMain(m *testing.M) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("dockertest pool: %v", err)
	}
	res, err := pool.Run("mysql", "8.4.4", []string{
		"MYSQL_ROOT_PASSWORD=root", "MYSQL_DATABASE=cpm_test",
	})
	if err != nil {
		log.Fatalf("dockertest run: %v", err)
	}
	dsn := fmt.Sprintf("root:root@tcp(localhost:%s)/cpm_test?parseTime=true&charset=utf8mb4", res.GetPort("3306/tcp"))
	if err := pool.Retry(func() error {
		db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err != nil {
			return err
		}
		authDB = db
		sqlDB, _ := db.DB()
		return sqlDB.Ping()
	}); err != nil {
		log.Fatalf("db connect: %v", err)
	}
	r := &migrate.Runner{DB: authDB, Dir: "../../migrations"}
	if err := r.Up(); err != nil {
		log.Fatalf("migrate up: %v", err)
	}
	code := m.Run()
	_ = pool.Purge(res)
	os.Exit(code)
}

// fakeDing 仅实现 dingtalk.Client 中本测试用到的方法，其余返回零值。
type fakeDing struct{ user dingtalk.User }

func (f fakeDing) GetUserByCode(context.Context, string) (dingtalk.User, error) { return f.user, nil }
func (f fakeDing) CreateCalendarEvent(context.Context, dingtalk.CalendarRequest) (string, error) {
	return "", nil
}
func (f fakeDing) ListCalendarResponses(context.Context, string) ([]dingtalk.Response, error) {
	return nil, nil
}
func (f fakeDing) QueryMeetingRooms(context.Context, string) ([]dingtalk.MeetingRoom, error) {
	return nil, nil
}
func (f fakeDing) SendWorkNotice(context.Context, []string, dingtalk.Card) error { return nil }
func (f fakeDing) SendInteractiveCard(context.Context, string, string, map[string]any) (dingtalk.CardInstance, error) {
	return dingtalk.CardInstance{}, nil
}
func (f fakeDing) BotBroadcast(context.Context, string, dingtalk.Card) error { return nil }
func (f fakeDing) StartOAProcess(context.Context, dingtalk.ApprovalRequest) (string, error) {
	return "", nil
}

func newAuthCfg() *config.Config {
	c := &config.Config{}
	c.JWT.Secret = "test-secret"
	c.JWT.TTLHours = 1
	c.DingTalk.Mode = "real"
	c.Seed.DefaultTenantID = 1
	return c
}

func TestDingLogin_AdminBySysFlag_PersistsAndIssuesRole(t *testing.T) {
	require.NoError(t, authDB.Exec("TRUNCATE users").Error)
	cfg := newAuthCfg()
	h := NewHandler(authDB, cfg, fakeDing{user: dingtalk.User{
		DingUserID: "u-admin", Name: "管理员", AvatarURL: "http://a/1.png", UnionID: "un-admin", IsAdmin: true,
	}})
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h.Register(r.Group("/"))
	srv := httptest.NewServer(r)
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{"code": "x"})
	resp, err := http.Post(srv.URL+"/auth/dingtalk/login", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	var lr loginResp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&lr))
	require.NotEmpty(t, lr.Token)

	var row struct {
		UnionID string
		IsAdmin int
	}
	require.NoError(t, authDB.Raw("SELECT union_id, is_admin FROM users WHERE ding_user_id=?", "u-admin").Scan(&row).Error)
	require.Equal(t, "un-admin", row.UnionID)
	require.Equal(t, 1, row.IsAdmin)

	signer := &Signer{Secret: []byte(cfg.JWT.Secret), TTL: time.Hour}
	claims, err := signer.Parse(lr.Token)
	require.NoError(t, err)
	require.Contains(t, claims.Roles, "admin")
}

func TestDingLogin_NonAdminByAllowlist(t *testing.T) {
	require.NoError(t, authDB.Exec("TRUNCATE users").Error)
	cfg := newAuthCfg()
	cfg.DingTalk.AdminUserIDs = []string{"u-white"}
	h := NewHandler(authDB, cfg, fakeDing{user: dingtalk.User{
		DingUserID: "u-white", Name: "白名单", UnionID: "un-w", IsAdmin: false,
	}})
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h.Register(r.Group("/"))
	srv := httptest.NewServer(r)
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{"code": "x"})
	resp, err := http.Post(srv.URL+"/auth/dingtalk/login", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	var lr loginResp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&lr))

	var row struct {
		UnionID string
		IsAdmin int
	}
	require.NoError(t, authDB.Raw("SELECT union_id, is_admin FROM users WHERE ding_user_id=?", "u-white").Scan(&row).Error)
	require.Equal(t, "un-w", row.UnionID)
	require.Equal(t, 0, row.IsAdmin)

	signer := &Signer{Secret: []byte(cfg.JWT.Secret), TTL: time.Hour}
	claims, err := signer.Parse(lr.Token)
	require.NoError(t, err)
	require.Contains(t, claims.Roles, "admin")
}

func TestDingLogin_ExistingUserSyncsProfileFromDingTalk(t *testing.T) {
	require.NoError(t, authDB.Exec("TRUNCATE users").Error)
	require.NoError(t, authDB.Exec(
		"INSERT INTO users (tenant_id, ding_user_id, name, avatar_url, union_id, is_admin) VALUES (1, 'u-sync', '旧名字', 'http://old/avatar.png', 'old-union', 0)",
	).Error)
	cfg := newAuthCfg()
	h := NewHandler(authDB, cfg, fakeDing{user: dingtalk.User{
		DingUserID: "u-sync", Name: "钉钉真实姓名", AvatarURL: "http://real/avatar.png", UnionID: "real-union", IsAdmin: true,
	}})
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h.Register(r.Group("/"))
	srv := httptest.NewServer(r)
	defer srv.Close()

	body, _ := json.Marshal(map[string]string{"code": "x"})
	resp, err := http.Post(srv.URL+"/auth/dingtalk/login", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	require.Equal(t, 200, resp.StatusCode)
	var lr loginResp
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&lr))
	require.Equal(t, "钉钉真实姓名", lr.Name)

	var row struct {
		Name      string
		AvatarURL string
		UnionID   string
		IsAdmin   int
	}
	require.NoError(t, authDB.Raw("SELECT name, avatar_url, union_id, is_admin FROM users WHERE ding_user_id=?", "u-sync").Scan(&row).Error)
	require.Equal(t, "钉钉真实姓名", row.Name)
	require.Equal(t, "http://real/avatar.png", row.AvatarURL)
	require.Equal(t, "real-union", row.UnionID)
	require.Equal(t, 1, row.IsAdmin)
}

func TestAdminGroup_RoleGate(t *testing.T) {
	require.NoError(t, authDB.Exec("TRUNCATE users").Error)
	authDB.Exec("INSERT INTO users (id, tenant_id, ding_user_id, name, is_admin) VALUES (1,1,'a','管理',1),(2,1,'b','普通',0)")
	cfg := newAuthCfg()
	signer := &Signer{Secret: []byte(cfg.JWT.Secret), TTL: time.Hour}
	adminTok, _ := signer.Issue(1, 1, []string{"admin"})
	userTok, _ := signer.Issue(2, 1, nil)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	admin := r.Group("/", RequireJWTWithUser(signer, authDB), RequireRole("admin"))
	admin.GET("/admin/ping", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })
	srv := httptest.NewServer(r)
	defer srv.Close()

	get := func(tok string) int {
		req, _ := http.NewRequest("GET", srv.URL+"/admin/ping", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		return resp.StatusCode
	}
	require.Equal(t, 200, get(adminTok))
	require.Equal(t, 403, get(userTok))
}
