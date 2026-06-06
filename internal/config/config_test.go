package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExpandEnv_DingTalk(t *testing.T) {
	t.Setenv("DINGTALK_APP_SECRET", "secret-xyz")
	t.Setenv("DINGTALK_ROBOT_SECRET", "robot-abc")
	c := &Config{}
	c.DingTalk.AppKey = "ak-plain"
	c.DingTalk.AppSecret = "${DINGTALK_APP_SECRET}"
	c.DingTalk.Robots = []RobotCfg{{ID: "g1", Webhook: "https://x?access_token=t", Secret: "${DINGTALK_ROBOT_SECRET}"}}
	expandEnv(c)
	require.Equal(t, "ak-plain", c.DingTalk.AppKey)
	require.Equal(t, "secret-xyz", c.DingTalk.AppSecret)
	require.Equal(t, "robot-abc", c.DingTalk.Robots[0].Secret)
}

func TestLoadExample(t *testing.T) {
	cfg, err := Load("../../configs")
	require.NoError(t, err)
	require.Equal(t, 8080, cfg.Server.Port)
	require.Equal(t, "mock", cfg.DingTalk.Mode)
	require.Equal(t, "claude", cfg.LLM.Provider)
	require.Equal(t, "claude-sonnet-4-7", cfg.LLM.Claude.Model)
}
