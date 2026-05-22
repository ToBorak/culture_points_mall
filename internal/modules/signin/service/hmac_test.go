package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHMACDoubleWindow(t *testing.T) {
	secret := "test-secret"
	now := time.Unix(1_700_000_000, 0)
	cur := now.Unix() / 60
	codeNow := CodeFor(42, cur, secret)
	codePrev := CodeFor(42, cur-1, secret)

	require.True(t, ValidCode(42, codeNow, 60, secret, now))
	require.True(t, ValidCode(42, codePrev, 60, secret, now))
	require.False(t, ValidCode(42, "wrong", 60, secret, now))
	require.False(t, ValidCode(99, codeNow, 60, secret, now))
}
