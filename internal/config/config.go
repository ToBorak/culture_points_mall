package config

import (
	"os"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerCfg   `mapstructure:"server"`
	MCP      MCPCfg      `mapstructure:"mcp"`
	MySQL    MySQLCfg    `mapstructure:"mysql"`
	Redis    RedisCfg    `mapstructure:"redis"`
	JWT      JWTCfg      `mapstructure:"jwt"`
	DingTalk DingTalkCfg `mapstructure:"dingtalk"`
	LLM      LLMCfg      `mapstructure:"llm"`
	Signin   SigninCfg   `mapstructure:"signin"`
	Seed     SeedCfg     `mapstructure:"seed"`
	Storage  StorageCfg  `mapstructure:"storage"`
	Stars    StarsCfg    `mapstructure:"stars"`
}

type ServerCfg struct{ Port int }
type MCPCfg struct{ Port int }
type MySQLCfg struct{ DSN string }
type RedisCfg struct {
	Addr string
	DB   int
}
type JWTCfg struct {
	Secret   string
	TTLHours int `mapstructure:"ttl_hours"`
}
type RobotCfg struct {
	ID      string `mapstructure:"id"`
	Name    string `mapstructure:"name"`
	Webhook string `mapstructure:"webhook"`
	Secret  string `mapstructure:"secret"`
}
type DingTalkCfg struct {
	Mode                     string
	AppKey                   string     `mapstructure:"app_key"`
	AppSecret                string     `mapstructure:"app_secret"`
	CorpID                   string     `mapstructure:"corp_id"`
	AgentID                  int64      `mapstructure:"agent_id"`
	AdminUserIDs             []string   `mapstructure:"admin_user_ids"`
	CalendarOrganizerUnionID string     `mapstructure:"calendar_organizer_unionid"`
	Robots                   []RobotCfg `mapstructure:"robots"`
	H5BaseURL                string     `mapstructure:"h5_base_url"`
}
type LLMCfg struct {
	Provider string
	Claude   ProviderCfg
	OpenAI   ProviderCfg `mapstructure:"openai"`
	DeepSeek ProviderCfg `mapstructure:"deepseek"`
	Qwen     ProviderCfg `mapstructure:"qwen"`
}
type ProviderCfg struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
	Model   string
}
type SigninCfg struct {
	Secret        string
	WindowSeconds int `mapstructure:"window_seconds"`
}
type SeedCfg struct {
	DefaultTenantID int64 `mapstructure:"default_tenant_id"`
	WelcomeBonus    int   `mapstructure:"welcome_bonus"`
}

// StorageCfg 本地文件存储（商品/奖品图片上传）。UploadDir 为空时回退到 ./uploads。
type StorageCfg struct {
	UploadDir string `mapstructure:"upload_dir"`
}

// StarsCfg 文化星标提报评选积分规则。零值由 stars.New 兜底为默认。
type StarsCfg struct {
	NominatePoints      int `mapstructure:"nominate_points"`       // 提报每次 +2
	NominatedPoints     int `mapstructure:"nominated_points"`      // 被提名每次 +4
	WinnerPoints        int `mapstructure:"winner_points"`         // 评选上 +8
	NominateMonthlyCap  int `mapstructure:"nominate_monthly_cap"`  // 提报积分月上限 6
	NominatedMonthlyCap int `mapstructure:"nominated_monthly_cap"` // 被提名积分月上限 16
}

func Load(paths ...string) (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	for _, p := range paths {
		v.AddConfigPath(p)
	}
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := v.ReadInConfig(); err != nil {
		var nf viper.ConfigFileNotFoundError
		if !isNotFound(err, &nf) {
			return nil, err
		}
		v.SetConfigName("config.example")
		if err := v.ReadInConfig(); err != nil {
			return nil, err
		}
	}
	var c Config
	if err := v.Unmarshal(&c); err != nil {
		return nil, err
	}
	expandEnv(&c)
	if c.Storage.UploadDir == "" {
		c.Storage.UploadDir = "./uploads"
	}
	return &c, nil
}

func isNotFound(err error, target *viper.ConfigFileNotFoundError) bool {
	_, ok := err.(viper.ConfigFileNotFoundError)
	if ok {
		*target = err.(viper.ConfigFileNotFoundError)
	}
	return ok
}

func expandEnv(c *Config) {
	c.LLM.Claude.APIKey = os.ExpandEnv(c.LLM.Claude.APIKey)
	c.LLM.OpenAI.APIKey = os.ExpandEnv(c.LLM.OpenAI.APIKey)
	c.LLM.DeepSeek.APIKey = os.ExpandEnv(c.LLM.DeepSeek.APIKey)
	c.LLM.Qwen.APIKey = os.ExpandEnv(c.LLM.Qwen.APIKey)
	c.DingTalk.AppKey = os.ExpandEnv(c.DingTalk.AppKey)
	c.DingTalk.AppSecret = os.ExpandEnv(c.DingTalk.AppSecret)
	for i := range c.DingTalk.Robots {
		c.DingTalk.Robots[i].Webhook = os.ExpandEnv(c.DingTalk.Robots[i].Webhook)
		c.DingTalk.Robots[i].Secret = os.ExpandEnv(c.DingTalk.Robots[i].Secret)
	}
}
