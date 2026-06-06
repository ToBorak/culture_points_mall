package service

import (
	"context"

	"gorm.io/gorm"
)

type Service struct{ DB *gorm.DB }

func New(db *gorm.DB) *Service { return &Service{DB: db} }

type Entry struct {
	Rank      int    `json:"rank"`
	UserID    int64  `json:"userId"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl"`
	DeptName  string `json:"deptName"`
	Score     int    `json:"score"`
}

type ListParams struct {
	TenantID    int64
	Scope       string
	DimensionID int64
	Limit       int
}

func (s *Service) List(ctx context.Context, p ListParams) ([]Entry, error) {
	if p.Limit <= 0 || p.Limit > 200 {
		p.Limit = 50
	}
	var rows []Entry
	switch {
	case p.Scope == "dim" && p.DimensionID > 0:
		err := s.DB.WithContext(ctx).Raw(`
			SELECT u.id AS user_id, u.name, u.avatar_url, COALESCE(d.name,'') AS dept_name, s.total_score AS score
			FROM user_dimension_scores s
			JOIN users u ON u.id = s.user_id AND u.tenant_id = s.tenant_id
			LEFT JOIN departments d ON d.id = u.dept_id AND d.tenant_id = u.tenant_id
			WHERE s.tenant_id = ? AND s.dimension_id = ?
			ORDER BY s.total_score DESC, u.id ASC
			LIMIT ?
		`, p.TenantID, p.DimensionID, p.Limit).Scan(&rows).Error
		if err != nil {
			return nil, err
		}
	case p.Scope == "dept":
		err := s.DB.WithContext(ctx).Raw(`
			SELECT
				d.id AS user_id, d.name AS name, '' AS avatar_url, '' AS dept_name,
				COALESCE(SUM(s.total_score), 0) AS score
			FROM departments d
			LEFT JOIN users u ON u.dept_id = d.id AND u.tenant_id = d.tenant_id
			LEFT JOIN user_dimension_scores s ON s.user_id = u.id
			WHERE d.tenant_id = ?
			GROUP BY d.id, d.name
			ORDER BY score DESC
			LIMIT ?
		`, p.TenantID, p.Limit).Scan(&rows).Error
		if err != nil {
			return nil, err
		}
	default:
		err := s.DB.WithContext(ctx).Raw(`
			SELECT u.id AS user_id, u.name, u.avatar_url, COALESCE(d.name,'') AS dept_name,
				COALESCE(SUM(s.total_score), 0) AS score
			FROM users u
			LEFT JOIN user_dimension_scores s ON s.user_id = u.id
			LEFT JOIN departments d ON d.id = u.dept_id AND d.tenant_id = u.tenant_id
			WHERE u.tenant_id = ?
			GROUP BY u.id, u.name, u.avatar_url, d.name
			ORDER BY score DESC, u.id ASC
			LIMIT ?
		`, p.TenantID, p.Limit).Scan(&rows).Error
		if err != nil {
			return nil, err
		}
	}
	for i := range rows {
		if rows[i].Rank == 0 {
			rows[i].Rank = i + 1
		}
	}
	return rows, nil
}
