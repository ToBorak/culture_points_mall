package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadExample(t *testing.T) {
	cfg, err := Load("../../configs")
	require.NoError(t, err)
	require.Equal(t, 8080, cfg.Server.Port)
	require.Equal(t, "mock", cfg.DingTalk.Mode)
	require.Equal(t, "claude", cfg.LLM.Provider)
	require.Equal(t, "claude-sonnet-4-7", cfg.LLM.Claude.Model)
}
