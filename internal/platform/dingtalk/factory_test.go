package dingtalk

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/standardsoftware/culture_points_mall/internal/config"
)

func TestNew_SelectsByMode(t *testing.T) {
	mock := NewMock(nil, NewBus())

	got := New(config.DingTalkCfg{Mode: "mock"}, nil, mock)
	gotMock, ok := got.(*MockClient)
	require.True(t, ok)
	require.Same(t, mock, gotMock)

	gotDefault := New(config.DingTalkCfg{Mode: ""}, nil, mock)
	dm, ok := gotDefault.(*MockClient)
	require.True(t, ok)
	require.Same(t, mock, dm)

	real := New(config.DingTalkCfg{Mode: "real", AppKey: "ak"}, nil, mock)
	_, ok = real.(*RealClient)
	require.True(t, ok)
}
