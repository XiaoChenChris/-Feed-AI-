package repositories

import (
	"context"
	"errors"

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

func (vr *VideoRepository) ChangeLikesCount(ctx context.Context, id uint, change int64) error {
	return vr.DB.WithContext(ctx).Model(&models.Video{}).
		Where("id = ?", id).
		UpdateColumn("likes_count", gorm.Expr("GREATEST(likes_count + ?, 0)", change)).Error
}

func (vr *VideoRepository) ChangePopularity(ctx context.Context, id uint, change int64) error {
	return vr.DB.WithContext(ctx).Model(&models.Video{}).
		Where("id = ?", id).
		UpdateColumn("popularity", gorm.Expr("GREATEST(popularity + ?, 0)", change)).Error
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
