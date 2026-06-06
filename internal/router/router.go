package router

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/standardsoftware/culture_points_mall/internal/auth"

	insightsh "github.com/standardsoftware/culture_points_mall/internal/modules/insights/handler"
	insightsservice "github.com/standardsoftware/culture_points_mall/internal/modules/insights/service"
	layoutsh "github.com/standardsoftware/culture_points_mall/internal/modules/layouts/handler"
	layoutsservice "github.com/standardsoftware/culture_points_mall/internal/modules/layouts/service"
	"github.com/standardsoftware/culture_points_mall/internal/config"
	"github.com/standardsoftware/culture_points_mall/internal/platform/dingtalk"
	"github.com/standardsoftware/culture_points_mall/internal/platform/llm"

	agenthandler "github.com/standardsoftware/culture_points_mall/internal/modules/agent/handler"
	acth "github.com/standardsoftware/culture_points_mall/internal/modules/activities/handler"
	actrepo "github.com/standardsoftware/culture_points_mall/internal/modules/activities/repository"
	actsvc "github.com/standardsoftware/culture_points_mall/internal/modules/activities/service"
	achvh "github.com/standardsoftware/culture_points_mall/internal/modules/achievements/handler"
	achvrepo "github.com/standardsoftware/culture_points_mall/internal/modules/achievements/repository"
	achvsvc "github.com/standardsoftware/culture_points_mall/internal/modules/achievements/service"
	lbh "github.com/standardsoftware/culture_points_mall/internal/modules/leaderboard/handler"
	lbsvc "github.com/standardsoftware/culture_points_mall/internal/modules/leaderboard/service"
	mallh "github.com/standardsoftware/culture_points_mall/internal/modules/mall/handler"
	mallrepo "github.com/standardsoftware/culture_points_mall/internal/modules/mall/repository"
	mallsvc "github.com/standardsoftware/culture_points_mall/internal/modules/mall/service"
	passporth "github.com/standardsoftware/culture_points_mall/internal/modules/passport/handler"
	pointsh "github.com/standardsoftware/culture_points_mall/internal/modules/points/handler"
	pointsrepo "github.com/standardsoftware/culture_points_mall/internal/modules/points/repository"
	pointssvc "github.com/standardsoftware/culture_points_mall/internal/modules/points/service"
	signinh "github.com/standardsoftware/culture_points_mall/internal/modules/signin/handler"
	signinrepo "github.com/standardsoftware/culture_points_mall/internal/modules/signin/repository"
	signinsvc "github.com/standardsoftware/culture_points_mall/internal/modules/signin/service"
	usersh "github.com/standardsoftware/culture_points_mall/internal/modules/users/handler"
	usersrepo "github.com/standardsoftware/culture_points_mall/internal/modules/users/repository"
	usersvc "github.com/standardsoftware/culture_points_mall/internal/modules/users/service"
	valuesh "github.com/standardsoftware/culture_points_mall/internal/modules/values/handler"
	valuesrepo "github.com/standardsoftware/culture_points_mall/internal/modules/values/repository"
	valuessvc "github.com/standardsoftware/culture_points_mall/internal/modules/values/service"
)

type Deps struct {
	DB           *gorm.DB
	Cfg          *config.Config
	DingMock     *dingtalk.MockClient
	DingBus      *dingtalk.Bus
	DingClient   dingtalk.Client
	LLM          llm.Client
	AgentHandler *agenthandler.Handler
	Redis        *redis.Client
}

// welcomeGranter 新用户/积分为零用户首次登录时一次性发放欢迎积分。
// 全部计入 sort_order=0 的首维度，方便盲盒消费扣减。
type welcomeGranter struct {
	points *pointssvc.Service
	values *valuessvc.Service
}

func (g *welcomeGranter) GrantWelcomeBonus(ctx context.Context, tenantID, userID int64, amount int) error {
	dims, err := g.values.GetDimensions(ctx, tenantID)
	if err != nil || len(dims) == 0 {
		return err
	}
	_, err = g.points.AddPoints(ctx, pointssvc.AddPointsCmd{
		TenantID: tenantID,
		UserID:   userID,
		Amount:   amount,
		DimCode:  dims[0].Code,
		Reason:   "新员工欢迎积分",
	})
	return err
}

func Build(deps Deps) *gin.Engine {
	r := gin.Default()
	r.GET("/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })

	valuesRepo := valuesrepo.New(deps.DB)
	valuesSvc := valuessvc.New(valuesRepo)
	pointsRepo := pointsrepo.New(deps.DB)
	pointsSvc := pointssvc.New(deps.DB, pointsRepo, valuesSvc, deps.Redis)

	signer := &auth.Signer{Secret: []byte(deps.Cfg.JWT.Secret), TTL: time.Duration(deps.Cfg.JWT.TTLHours) * time.Hour}

	achvRepo := achvrepo.New(deps.DB)
	achvSvc := achvsvc.New(&achvsvc.Wrap{Inner: achvRepo}, pointsSvc, valuesSvc)

	actRepo := actrepo.New(deps.DB)
	actSvc := actsvc.New(actRepo, valuesSvc)

	signinRepo := signinrepo.New(deps.DB)
	signinSvc := signinsvc.New(signinRepo, actSvc, pointsSvc, achvSvc, deps.Cfg.Signin.Secret, deps.Cfg.Signin.WindowSeconds)

	// 开放组：无鉴权，仅限公开端点
	open := r.Group("/")
	valuesHandler := valuesh.New(valuesSvc)
	valuesHandler.Register(open)
	auth.NewHandler(deps.DB, deps.Cfg, deps.DingClient).
		WithGranter(&welcomeGranter{points: pointsSvc, values: valuesSvc}).
		Register(open)

	// 受保护组：JWT + 用户存在性校验（DB 重置后老 token 自动失效）
	authed := r.Group("/", auth.RequireJWTWithUser(signer, deps.DB))
	actHandler := acth.New(actSvc)
	actHandler.Register(authed)
	pointsh.New(pointsSvc, valuesSvc).Register(authed)
	usersh.New(usersvc.New(usersrepo.New(deps.DB))).Register(authed)
	achvh.New(achvSvc).Register(authed)
	passporth.New(pointsSvc, achvSvc).Register(authed)
	lbh.New(lbsvc.New(deps.DB)).Register(authed)
	signinHandlerInst := signinh.New(signinSvc)
	signinHandlerInst.Register(authed)
	signinHandlerInst.RegisterWS(open)
	mallRepo := mallrepo.New(deps.DB)
	mallSvc := mallsvc.New(mallRepo, pointsSvc, valuesSvc)
	mallh.New(mallRepo, mallSvc).Register(authed)

	// AI 洞察（DNA 报告 / 教练 / 挑战 / 排行解读）
	if deps.LLM != nil {
		insightsSvc := insightsservice.New(deps.LLM, pointsSvc, valuesSvc, deps.DB, deps.Redis)
		insightsh.New(insightsSvc).Register(authed)
	}

	// 布局编排（admin 拖拽自定义 H5 首页模块顺序）
	layoutsh.New(layoutsservice.New(deps.DB)).Register(authed)

	// 后台管理组：JWT + 用户存在性 + admin 角色门禁
	admin := r.Group("/", auth.RequireJWTWithUser(signer, deps.DB), auth.RequireRole("admin"))
	valuesHandler.RegisterAdmin(admin)
	actHandler.RegisterAdmin(admin)
	signinHandlerInst.RegisterAdmin(admin)
	dingtalk.NewMockHandler(deps.DB, deps.DingBus).Register(admin)
	if deps.AgentHandler != nil {
		deps.AgentHandler.Register(admin)
	}

	return r
}
