package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/standardsoftware/culture_points_mall/internal/modules/activities/domain"
)

type GormRepo struct{ DB *gorm.DB }

func New(db *gorm.DB) *GormRepo { return &GormRepo{DB: db} }

func (r *GormRepo) Create(ctx context.Context, a *domain.Activity) error {
	return r.DB.WithContext(ctx).Create(a).Error
}

func (r *GormRepo) GetByID(ctx context.Context, tenantID, id int64) (*domain.Activity, error) {
	var a domain.Activity
	if err := r.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&a).Error; err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *GormRepo) ListByTenant(ctx context.Context, tenantID int64, status domain.Status, limit int) ([]domain.Activity, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	// MySQL 没有 NULLS LAST，用 IS NULL trick
	q := r.DB.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("start_at IS NULL, start_at DESC, id DESC").
		Limit(limit)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var rows []domain.Activity
	err := q.Find(&rows).Error
	return rows, err
}

func (r *GormRepo) UpdateStatus(ctx context.Context, id int64, status domain.Status) error {
	return r.DB.WithContext(ctx).
		Model(&domain.Activity{}).
		Where("id = ?", id).
		Update("status", status).
		Error
}

// ---- 报名（activity_enrollments） ----

// Enroll 幂等报名：已存在则不动（保留 checked_in 等状态）。
func (r *GormRepo) Enroll(ctx context.Context, activityID, userID int64) error {
	e := &domain.Enrollment{ActivityID: activityID, UserID: userID, Status: domain.EnrollEnrolled}
	return r.DB.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "activity_id"}, {Name: "user_id"}},
			DoNothing: true,
		}).
		Create(e).Error
}

// SetCalendarEventID 记录报名时自动创建的钉钉日程事件ID（取消报名时据此删除）。
func (r *GormRepo) SetCalendarEventID(ctx context.Context, activityID, userID int64, eventID string) error {
	return r.DB.WithContext(ctx).
		Model(&domain.Enrollment{}).
		Where("activity_id = ? AND user_id = ?", activityID, userID).
		Update("calendar_event_id", eventID).Error
}

// Unenroll 取消报名：直接删行，允许之后重新报名。
func (r *GormRepo) Unenroll(ctx context.Context, activityID, userID int64) error {
	return r.DB.WithContext(ctx).
		Where("activity_id = ? AND user_id = ?", activityID, userID).
		Delete(&domain.Enrollment{}).Error
}

// MarkCheckedIn 签到成功时调用：没有报名记录则自动补一条并置为 checked_in。
func (r *GormRepo) MarkCheckedIn(ctx context.Context, activityID, userID int64) error {
	e := &domain.Enrollment{ActivityID: activityID, UserID: userID, Status: domain.EnrollCheckedIn}
	return r.DB.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "activity_id"}, {Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"status"}),
		}).
		Create(e).Error
}

// GetEnrollment 返回某用户在某活动的报名记录，无则返回 (nil, nil)。
func (r *GormRepo) GetEnrollment(ctx context.Context, activityID, userID int64) (*domain.Enrollment, error) {
	var e domain.Enrollment
	err := r.DB.WithContext(ctx).
		Where("activity_id = ? AND user_id = ?", activityID, userID).
		First(&e).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// CountEnrolled 统计有效报名人数（缺席不计）。
func (r *GormRepo) CountEnrolled(ctx context.Context, activityID int64) (int64, error) {
	var n int64
	err := r.DB.WithContext(ctx).Model(&domain.Enrollment{}).
		Where("activity_id = ? AND status <> ?", activityID, domain.EnrollAbsent).
		Count(&n).Error
	return n, err
}

// CountsByActivityIDs 批量统计列表内各活动的有效报名人数。
func (r *GormRepo) CountsByActivityIDs(ctx context.Context, ids []int64) (map[int64]int64, error) {
	out := map[int64]int64{}
	if len(ids) == 0 {
		return out, nil
	}
	type row struct {
		ActivityID int64
		N          int64
	}
	var rows []row
	err := r.DB.WithContext(ctx).Model(&domain.Enrollment{}).
		Select("activity_id, COUNT(*) AS n").
		Where("activity_id IN ? AND status <> ?", ids, domain.EnrollAbsent).
		Group("activity_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, rec := range rows {
		out[rec.ActivityID] = rec.N
	}
	return out, nil
}

// EnrollmentsByUserForActivities 批量返回某用户在列表内各活动的报名记录。
func (r *GormRepo) EnrollmentsByUserForActivities(ctx context.Context, userID int64, ids []int64) (map[int64]domain.Enrollment, error) {
	out := map[int64]domain.Enrollment{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []domain.Enrollment
	err := r.DB.WithContext(ctx).
		Where("user_id = ? AND activity_id IN ?", userID, ids).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, e := range rows {
		out[e.ActivityID] = e
	}
	return out, nil
}

// Delete 删除活动记录（用于「撤销发布活动」回撤）。
func (r *GormRepo) Delete(ctx context.Context, tenantID, id int64) error {
	return r.DB.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&domain.Activity{}).Error
}
