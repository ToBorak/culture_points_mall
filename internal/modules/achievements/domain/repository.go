package domain

import "context"

type Repository interface {
	ListBadges(ctx context.Context, tenantID int64) ([]Badge, error)
	ListUserBadgeIDs(ctx context.Context, userID int64) ([]int64, error)
	AwardBadge(ctx context.Context, userID, badgeID int64) error
}
