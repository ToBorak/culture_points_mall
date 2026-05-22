package cpmctx

import "context"

type ctxKey int

const (
	keyTenantID ctxKey = iota + 1
	keyUserID
	keyRoles
)

func WithTenant(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, keyTenantID, id)
}
func TenantID(ctx context.Context) int64 {
	if v, ok := ctx.Value(keyTenantID).(int64); ok {
		return v
	}
	return 0
}

func WithUser(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, keyUserID, id)
}
func UserID(ctx context.Context) int64 {
	if v, ok := ctx.Value(keyUserID).(int64); ok {
		return v
	}
	return 0
}

func WithRoles(ctx context.Context, roles []string) context.Context {
	return context.WithValue(ctx, keyRoles, roles)
}
func Roles(ctx context.Context) []string {
	if v, ok := ctx.Value(keyRoles).([]string); ok {
		return v
	}
	return nil
}
