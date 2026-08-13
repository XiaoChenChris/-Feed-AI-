package repositories

import (
	"context"
	"time"

	"feedsystem_ai_go/internal/models"
	"gorm.io/gorm"
)

type FeedRepository struct {
	DB *gorm.DB
}

func NewFeedRepository(db *gorm.DB) *FeedRepository {
	return &FeedRepository{DB: db}
}

func (repo *FeedRepository) ListLatest(ctx context.Context, limit int, latestBefore time.Time) ([]*models.Video, error) {
	var videos []*models.Video
	query := repo.DB.WithContext(ctx).Model(&models.Video{}).
		Where("review_status = ?", "approved").
		Order("create_time DESC")
	if !latestBefore.IsZero() {
		query = query.Where("create_time < ?", latestBefore)
	}
	if err := query.Limit(limit).Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

func (repo *FeedRepository) ListLikesCountWithCursor(ctx context.Context, limit int, cursor *models.LikesCountCursor) ([]*models.Video, error) {
	var videos []*models.Video
	query := repo.DB.WithContext(ctx).Model(&models.Video{}).
		Where("review_status = ?", "approved").
		Order("likes_count DESC, id DESC")

	if cursor != nil {
		query = query.Where(
			"(likes_count < ?) OR (likes_count = ? AND id < ?)",
			cursor.LikesCount,
			cursor.LikesCount, cursor.ID,
		)
	}

	if err := query.Limit(limit).Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

func (repo *FeedRepository) ListByFollowing(ctx context.Context, limit int, viewerAccountID uint, latestBefore time.Time) ([]*models.Video, error) {
	var videos []*models.Video
	query := repo.DB.WithContext(ctx).Model(&models.Video{}).
		Where("review_status = ?", "approved").
		Order("create_time DESC")
	if viewerAccountID > 0 {
		followingSubQuery := repo.DB.WithContext(ctx).
			Model(&models.Social{}).
			Select("vlogger_id").
			Where("follower_id = ?", viewerAccountID)
		query = query.Where("author_id IN (?)", followingSubQuery)
	}
	if !latestBefore.IsZero() {
		query = query.Where("create_time < ?", latestBefore)
	}
	if err := query.Limit(limit).Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

func (repo *FeedRepository) ListByPopularity(ctx context.Context, limit int, popularityBefore int64, timeBefore time.Time, idBefore uint) ([]*models.Video, error) {
	var videos []*models.Video
	query := repo.DB.WithContext(ctx).Model(&models.Video{}).
		Where("review_status = ?", "approved").
		Order("popularity DESC, create_time DESC, id DESC")

	if !timeBefore.IsZero() && idBefore > 0 {
		query = query.Where(
			"(popularity < ?) OR (popularity = ? AND create_time < ?) OR (popularity = ? AND create_time = ? AND id < ?)",
			popularityBefore,
			popularityBefore, timeBefore,
			popularityBefore, timeBefore, idBefore,
		)
	}

	if err := query.Limit(limit).Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

func (repo *FeedRepository) GetByIDs(ctx context.Context, ids []uint) ([]*models.Video, error) {
	var videos []*models.Video
	if len(ids) == 0 {
		return videos, nil
	}
	if err := repo.DB.WithContext(ctx).Model(&models.Video{}).
		Where("id IN ? AND review_status = ?", ids, "approved").Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

func (repo *FeedRepository) ListByTag(ctx context.Context, tagName string, limit int) ([]*models.Video, error) {
	var videos []*models.Video
	err := repo.DB.WithContext(ctx).Model(&models.Video{}).Table("videos").
		Joins("JOIN video_tags ON video_tags.video_id = videos.id").
		Joins("JOIN tags ON tags.id = video_tags.tag_id").
		Where("tags.name = ? AND videos.review_status = ?", tagName, "approved").
		Order("videos.create_time desc").
		Limit(limit).
		Find(&videos).Error
	return videos, err
}

// ListByTags returns approved videos matching any of the given tags.
func (repo *FeedRepository) ListByTags(ctx context.Context, tagNames []string, limit int) ([]*models.Video, error) {
	if len(tagNames) == 0 {
		return nil, nil
	}
	var videos []*models.Video
	err := repo.DB.WithContext(ctx).Model(&models.Video{}).Table("videos").
		Joins("JOIN video_tags ON video_tags.video_id = videos.id").
		Joins("JOIN tags ON tags.id = video_tags.tag_id").
		Where("tags.name IN ? AND videos.review_status = ?", tagNames, "approved").
		Order("videos.create_time desc").
		Limit(limit).
		Find(&videos).Error
	return videos, err
}

// GetCategoryByIDs returns the category for each video id. For videos whose
// Category column is empty, it falls back to the video's first tag name.
func (repo *FeedRepository) GetCategoryByIDs(ctx context.Context, ids []uint) (map[uint]string, error) {
	result := make(map[uint]string)
	if len(ids) == 0 {
		return result, nil
	}
	type idCat struct {
		ID       uint
		Category string
	}
	var rows []idCat
	if err := repo.DB.WithContext(ctx).Model(&models.Video{}).
		Select("id, category").
		Where("id IN ?", ids).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	var needFallback []uint
	for _, r := range rows {
		if r.Category != "" {
			result[r.ID] = r.Category
		} else {
			needFallback = append(needFallback, r.ID)
		}
	}
	if len(needFallback) == 0 {
		return result, nil
	}
	type firstTag struct {
		VideoID uint
		Name    string
	}
	var tags []firstTag
	if err := repo.DB.WithContext(ctx).
		Table("video_tags").
		Select("video_tags.video_id AS video_id, tags.name AS name").
		Joins("JOIN tags ON tags.id = video_tags.tag_id").
		Where("video_tags.video_id IN ?", needFallback).
		Order("video_tags.id ASC").
		Find(&tags).Error; err != nil {
		return nil, err
	}
	for _, t := range tags {
		if _, ok := result[t.VideoID]; !ok {
			result[t.VideoID] = t.Name
		}
	}
	return result, nil
}
