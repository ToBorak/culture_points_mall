package router

import (
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/standardsoftware/culture_points_mall/internal/auth"
	"github.com/standardsoftware/culture_points_mall/internal/config"
	"github.com/standardsoftware/culture_points_mall/internal/platform/dingtalk"

	pointsh "github.com/standardsoftware/culture_points_mall/internal/modules/points/handler"
	pointsrepo "github.com/standardsoftware/culture_points_mall/internal/modules/points/repository"
	pointssvc "github.com/standardsoftware/culture_points_mall/internal/modules/points/service"
	valuesh "github.com/standardsoftware/culture_points_mall/internal/modules/values/handler"
	valuesrepo "github.com/standardsoftware/culture_points_mall/internal/modules/values/repository"
	valuessvc "github.com/standardsoftware/culture_points_mall/internal/modules/values/service"
)

type Deps struct {
	DB       *gorm.DB
	Cfg      *config.Config
	DingMock *dingtalk.MockClient
	DingBus  *dingtalk.Bus
}

func Build(deps Deps) *gin.Engine {
	r := gin.Default()
	r.GET("/healthz", func(c *gin.Context) { c.JSON(200, gin.H{"ok": true}) })

	valuesRepo := valuesrepo.New(deps.DB)
	valuesSvc := valuessvc.New(valuesRepo)
	pointsRepo := pointsrepo.New(deps.DB)
	pointsSvc := pointssvc.New(deps.DB, pointsRepo, valuesSvc)

	signer := &auth.Signer{Secret: []byte(deps.Cfg.JWT.Secret), TTL: time.Duration(deps.Cfg.JWT.TTLHours) * time.Hour}

	// 受保护组
	authed := r.Group("/", auth.RequireJWT(signer))
	pointsh.New(pointsSvc).Register(authed)

	// 开放组（含 admin 演示，正式生产应再加 admin role 校验）
	open := r.Group("/")
	valuesh.New(valuesSvc).Register(open)
	dingtalk.NewMockHandler(deps.DB, deps.DingBus).Register(open)

	return r
}
