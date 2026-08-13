package repositories

import (
	"context"
	"errors"
	"fmt"

	"feedsystem_ai_go/internal/models"
	"gorm.io/gorm"
)

type VideoRepository struct {
	DB *gorm.DB
}

func NewVideoRepository(db *gorm.DB) *VideoRepository {
	return &VideoRepository{DB: db}
}

func (vr *VideoRepository) CreateVideo(ctx context.Context, video *models.Video) error {
	return vr.DB.WithContext(ctx).Create(video).Error
}

func (vr *VideoRepository) CreateMsg(ctx context.Context, msg *models.OutboxMsg) error {
	return vr.DB.WithContext(ctx).Create(msg).Error
}

func (vr *VideoRepository) DeleteVideo(ctx context.Context, id uint) error {
	return vr.DB.WithContext(ctx).Delete(&models.Video{}, id).Error
}

func (vr *VideoRepository) ListByAuthorID(ctx context.Context, authorID int64) ([]models.Video, error) {
	var videos []models.Video
	if err := vr.DB.WithContext(ctx).
		Where("author_id = ?", authorID).
		Order("create_time desc").
		Limit(200).
		Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

func (vr *VideoRepository) GetByID(ctx context.Context, id uint) (*models.Video, error) {
	var video models.Video
	if err := vr.DB.WithContext(ctx).First(&video, id).Error; err != nil {
		return nil, err
	}
	return &video, nil
}

func (vr *VideoRepository) UpdateLikesCount(ctx context.Context, id uint, likesCount int64) error {
	return vr.DB.WithContext(ctx).Model(&models.Video{}).
		Where("id = ?", id).
		Update("likes_count", likesCount).Error
}

func (vr *VideoRepository) IsExist(ctx context.Context, id uint) (bool, error) {
	var video models.Video
	if err := vr.DB.WithContext(ctx).First(&video, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (vr *VideoRepository) UpdatePopularity(ctx context.Context, id uint, change int64) error {
	return vr.DB.WithContext(ctx).Model(&models.Video{}).
		Where("id = ?", id).
		Update("popularity", gorm.Expr("popularity + ?", change)).Error
}

const countMaxRetries = 5

// ChangeLikesCount updates likes_count with optimistic locking (version column).
func (vr *VideoRepository) ChangeLikesCount(ctx context.Context, id uint, change int64) error {
	return vr.changeCount(ctx, nil, id, change, 0)
}

// ChangePopularity updates popularity with optimistic locking (version column).
func (vr *VideoRepository) ChangePopularity(ctx context.Context, id uint, change int64) error {
	return vr.changeCount(ctx, nil, id, 0, change)
}

// changeCount applies an optimistic-locked increment to the video counters.
// It reads the current version, then performs a CAS (UPDATE ... WHERE version = ?)
// that bumps version atomically. On a version conflict it retries up to
// countMaxRetries times. When tx is provided the CAS runs inside that transaction.
func (vr *VideoRepository) changeCount(ctx context.Context, tx *gorm.DB, id uint, likeChange, popChange int64) error {
	db := vr.DB
	if tx != nil {
		db = tx
	}

	var lastErr error
	for attempt := 0; attempt < countMaxRetries; attempt++ {
		var v struct{ Version uint }
		if err := db.WithContext(ctx).Model(&models.Video{}).
			Where("id = ?", id).Select("version").Scan(&v).Error; err != nil {
			return err
		}

		updates := map[string]interface{}{"version": gorm.Expr("version + 1")}
		if likeChange != 0 {
			updates["likes_count"] = gorm.Expr("GREATEST(likes_count + ?, 0)", likeChange)
		}
		if popChange != 0 {
			updates["popularity"] = gorm.Expr("GREATEST(popularity + ?, 0)", popChange)
		}

		res := db.WithContext(ctx).Model(&models.Video{}).
			Where("id = ? AND version = ?", id, v.Version).
			Updates(updates)
		if res.Error != nil {
			lastErr = res.Error
			continue
		}
		if res.RowsAffected > 0 {
			return nil
		}
		// version conflict: another writer committed first, retry
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("optimistic lock conflict: exceeded %d retries updating video %d", countMaxRetries, id)
}

func (vr *VideoRepository) CountByAuthor(ctx context.Context, authorID uint) (int64, error) {
	var count int64
	if err := vr.DB.WithContext(ctx).Model(&models.Video{}).Where("author_id = ?", authorID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (vr *VideoRepository) TotalLikesByAuthor(ctx context.Context, authorID uint) (int64, error) {
	var total int64
	if err := vr.DB.WithContext(ctx).Model(&models.Video{}).Where("author_id = ?", authorID).Select("COALESCE(SUM(likes_count), 0)").Scan(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}
