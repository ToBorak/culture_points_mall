package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/standardsoftware/culture_points_mall/internal/modules/users/domain"
)

type GormRepo struct{ DB *gorm.DB }

func New(db *gorm.DB) *GormRepo { return &GormRepo{DB: db} }

func (r *GormRepo) GetByID(ctx context.Context, tenantID, id int64) (*domain.User, error) {
	var u domain.User
	if err := r.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// UserWithDept 带部门名的成员行（供后台"部门+员工"树状选择器）。
type UserWithDept struct {
	ID         int64
	DingUserID string
	Name       string
	DeptID     *int64
	DeptName   string
}

// ListWithDept 列出本租户成员并带上部门名（无部门的排在最后）。
func (r *GormRepo) ListWithDept(ctx context.Context, tenantID int64) ([]UserWithDept, error) {
	var rows []UserWithDept
	err := r.DB.WithContext(ctx).Raw(
		`SELECT u.id, u.ding_user_id, u.name, u.dept_id, COALESCE(d.name, '') AS dept_name
		 FROM users u
		 LEFT JOIN departments d ON d.id = u.dept_id AND d.tenant_id = u.tenant_id
		 WHERE u.tenant_id = ?
		 ORDER BY (u.dept_id IS NULL), u.dept_id, u.id`, tenantID).Scan(&rows).Error
	return rows, err
}

func (r *GormRepo) ListByDept(ctx context.Context, tenantID, deptID int64) ([]domain.User, error) {
	var rows []domain.User
	err := r.DB.WithContext(ctx).Where("tenant_id = ? AND dept_id = ?", tenantID, deptID).Find(&rows).Error
	return rows, err
}

func (r *GormRepo) ListByTenant(ctx context.Context, tenantID int64, limit int) ([]domain.User, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	var rows []domain.User
	err := r.DB.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("id DESC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}
