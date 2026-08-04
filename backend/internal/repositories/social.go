package repositories

import (
	"context"

	"feedsystem_ai_go/internal/models"
	"gorm.io/gorm"
)

type SocialRepository struct {
	DB *gorm.DB
}

func NewSocialRepository(db *gorm.DB) *SocialRepository {
	return &SocialRepository{DB: db}
}

func (r *SocialRepository) Follow(ctx context.Context, social *models.Social) error {
	return r.DB.WithContext(ctx).Create(social).Error
}

func (r *SocialRepository) Unfollow(ctx context.Context, social *models.Social) error {
	return r.DB.WithContext(ctx).
		Where("follower_id = ? AND vlogger_id = ?", social.FollowerID, social.VloggerID).
		Delete(&models.Social{}).Error
}

func (r *SocialRepository) GetAllFollowers(ctx context.Context, vloggerID uint) ([]*models.Account, error) {
	var relations []models.Social
	if err := r.DB.WithContext(ctx).
		Model(&models.Social{}).
		Where("vlogger_id = ?", vloggerID).
		Limit(200).
		Find(&relations).Error; err != nil {
		return nil, err
	}

	followerIDs := make([]uint, 0, len(relations))
	for _, rel := range relations {
		followerIDs = append(followerIDs, rel.FollowerID)
	}
	if len(followerIDs) == 0 {
		return []*models.Account{}, nil
	}

	var followers []*models.Account
	if err := r.DB.WithContext(ctx).
		Model(&models.Account{}).
		Where("id IN ?", followerIDs).
		Find(&followers).Error; err != nil {
		return nil, err
	}
	return followers, nil
}

func (r *SocialRepository) GetAllVloggers(ctx context.Context, followerID uint) ([]*models.Account, error) {
	var relations []models.Social
	if err := r.DB.WithContext(ctx).
		Model(&models.Social{}).
		Where("follower_id = ?", followerID).
		Limit(200).
		Find(&relations).Error; err != nil {
		return nil, err
	}

	vloggerIDs := make([]uint, 0, len(relations))
	for _, rel := range relations {
		vloggerIDs = append(vloggerIDs, rel.VloggerID)
	}
	if len(vloggerIDs) == 0 {
		return []*models.Account{}, nil
	}

	var vloggers []*models.Account
	if err := r.DB.WithContext(ctx).
		Model(&models.Account{}).
		Where("id IN ?", vloggerIDs).
		Find(&vloggers).Error; err != nil {
		return nil, err
	}
	return vloggers, nil
}

func (r *SocialRepository) IsFollowed(ctx context.Context, social *models.Social) (bool, error) {
	var count int64
	if err := r.DB.WithContext(ctx).
		Model(&models.Social{}).
		Where("follower_id = ? AND vlogger_id = ?", social.FollowerID, social.VloggerID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *SocialRepository) CountFollowers(ctx context.Context, vloggerID uint) (int64, error) {
	var count int64
	if err := r.DB.WithContext(ctx).Model(&models.Social{}).Where("vlogger_id = ?", vloggerID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *SocialRepository) CountVloggers(ctx context.Context, followerID uint) (int64, error) {
	var count int64
	if err := r.DB.WithContext(ctx).Model(&models.Social{}).Where("follower_id = ?", followerID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
