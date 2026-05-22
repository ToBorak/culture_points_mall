package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestJWTRoundTrip(t *testing.T) {
	s := &Signer{Secret: []byte("test"), TTL: time.Hour}
	tok, err := s.Issue(42, 1, []string{"hr"})
	require.NoError(t, err)
	c, err := s.Parse(tok)
	require.NoError(t, err)
	require.Equal(t, int64(42), c.UserID)
	require.Equal(t, int64(1), c.TenantID)
	require.Equal(t, []string{"hr"}, c.Roles)
}
