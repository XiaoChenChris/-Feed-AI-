package repositories

import (
	"context"
	"errors"

	"feedsystem_ai_go/internal/models"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

type LikeRepository struct {
	DB *gorm.DB
}

func NewLikeRepository(db *gorm.DB) *LikeRepository {
	return &LikeRepository{DB: db}
}

func (r *LikeRepository) Like(ctx context.Context, like *models.Like) error {
	return r.DB.WithContext(ctx).Create(like).Error
}

func (r *LikeRepository) Unlike(ctx context.Context, like *models.Like) error {
	return r.DB.WithContext(ctx).
		Where("video_id = ? AND account_id = ?", like.VideoID, like.AccountID).
		Delete(&models.Like{}).Error
}

func (r *LikeRepository) LikeIgnoreDuplicate(ctx context.Context, like *models.Like) (created bool, err error) {
	if like == nil || like.VideoID == 0 || like.AccountID == 0 {
		return false, nil
	}
	err = r.DB.WithContext(ctx).Create(like).Error
	if err == nil {
		return true, nil
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return false, nil
	}
	return false, err
}

func (r *LikeRepository) DeleteByVideoAndAccount(ctx context.Context, videoID, accountID uint) (deleted bool, err error) {
	if videoID == 0 || accountID == 0 {
		return false, nil
	}
	res := r.DB.WithContext(ctx).
		Where("video_id = ? AND account_id = ?", videoID, accountID).
		Delete(&models.Like{})
	return res.RowsAffected > 0, res.Error
}

func (r *LikeRepository) IsLiked(ctx context.Context, videoID, accountID uint) (bool, error) {
	var count int64
	err := r.DB.WithContext(ctx).Model(&models.Like{}).
		Where("video_id = ? AND account_id = ?", videoID, accountID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *LikeRepository) BatchGetLiked(ctx context.Context, videoIDs []uint, accountID uint) (map[uint]bool, error) {
	likeMap := make(map[uint]bool)
	if len(videoIDs) == 0 {
		return likeMap, nil
	}
	if accountID == 0 {
		return likeMap, nil
	}
	var likes []models.Like
	err := r.DB.WithContext(ctx).Model(&models.Like{}).
		Where("video_id IN ? AND account_id = ?", videoIDs, accountID).
		Find(&likes).Error
	if err != nil {
		return nil, err
	}
	for _, like := range likes {
		likeMap[like.VideoID] = true
	}
	return likeMap, nil
}

func (r *LikeRepository) ListLikedVideos(ctx context.Context, accountID uint) ([]models.Video, error) {
	var videos []models.Video
	if accountID == 0 {
		return videos, nil
	}
	err := r.DB.WithContext(ctx).
		Model(&models.Video{}).
		Joins("JOIN likes ON likes.video_id = videos.id").
		Where("likes.account_id = ?", accountID).
		Order("likes.created_at desc").
		Limit(200).
		Find(&videos).Error
	if err != nil {
		return nil, err
	}
	return videos, nil
}

// GetLikedVideoTags returns the distinct tag names of the account's recently
// liked videos (most recent first), capped at limit entries.
func (r *LikeRepository) GetLikedVideoTags(ctx context.Context, accountID uint, limit int) ([]string, error) {
	if accountID == 0 || limit <= 0 {
		return nil, nil
	}
	var names []string
	err := r.DB.WithContext(ctx).
		Table("likes").
		Select("tags.name").
		Joins("JOIN video_tags ON video_tags.video_id = likes.video_id").
		Joins("JOIN tags ON tags.id = video_tags.tag_id").
		Where("likes.account_id = ?", accountID).
		Order("likes.created_at DESC").
		Limit(limit * 3).
		Pluck("tags.name", &names).Error
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var out []string
	for _, n := range names {
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}
