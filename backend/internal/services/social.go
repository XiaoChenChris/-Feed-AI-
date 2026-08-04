package services

import (
	"context"
	"errors"

	"feedsystem_ai_go/internal/models"
	rabbitmq "feedsystem_ai_go/pkg/mq"
	"feedsystem_ai_go/internal/repositories"
)

type SocialService struct {
	repo        *repositories.SocialRepository
	accountRepo *repositories.AccountRepository
	socialMQ    *rabbitmq.SocialMQ
}

func NewSocialService(repo *repositories.SocialRepository, accountRepo *repositories.AccountRepository, socialMQ *rabbitmq.SocialMQ) *SocialService {
	return &SocialService{repo: repo, accountRepo: accountRepo, socialMQ: socialMQ}
}

func (s *SocialService) Follow(ctx context.Context, social *models.Social) error {
	_, err := s.accountRepo.FindByID(ctx, social.FollowerID)
	if err != nil {
		return err
	}
	_, err = s.accountRepo.FindByID(ctx, social.VloggerID)
	if err != nil {
		return err
	}
	if social.FollowerID == social.VloggerID {
		return errors.New("can not follow self")
	}
	isFollowed, err := s.repo.IsFollowed(ctx, social)
	if err != nil {
		return err
	}
	if isFollowed {
		return errors.New("already followed")
	}
	if s.socialMQ != nil {
		s.socialMQ.Follow(ctx, social.FollowerID, social.VloggerID)
	}
	return s.repo.Follow(ctx, social)
}

func (s *SocialService) Unfollow(ctx context.Context, social *models.Social) error {
	_, err := s.accountRepo.FindByID(ctx, social.FollowerID)
	if err != nil {
		return err
	}
	_, err = s.accountRepo.FindByID(ctx, social.VloggerID)
	if err != nil {
		return err
	}
	isFollowed, err := s.repo.IsFollowed(ctx, social)
	if err != nil {
		return err
	}
	if !isFollowed {
		return errors.New("not followed")
	}
	if s.socialMQ != nil {
		s.socialMQ.UnFollow(ctx, social.FollowerID, social.VloggerID)
	}
	return s.repo.Unfollow(ctx, social)
}

func (s *SocialService) GetAllFollowers(ctx context.Context, vloggerID uint) ([]*models.Account, error) {
	_, err := s.accountRepo.FindByID(ctx, vloggerID)
	if err != nil {
		return nil, err
	}
	return s.repo.GetAllFollowers(ctx, vloggerID)
}

func (s *SocialService) GetAllVloggers(ctx context.Context, followerID uint) ([]*models.Account, error) {
	_, err := s.accountRepo.FindByID(ctx, followerID)
	if err != nil {
		return nil, err
	}
	return s.repo.GetAllVloggers(ctx, followerID)
}

func (s *SocialService) CountFollowers(ctx context.Context, vloggerID uint) (int64, error) {
	return s.repo.CountFollowers(ctx, vloggerID)
}

func (s *SocialService) CountVloggers(ctx context.Context, followerID uint) (int64, error) {
	return s.repo.CountVloggers(ctx, followerID)
}

func (s *SocialService) IsFollowed(ctx context.Context, social *models.Social) (bool, error) {
	_, err := s.accountRepo.FindByID(ctx, social.FollowerID)
	if err != nil {
		return false, err
	}
	_, err = s.accountRepo.FindByID(ctx, social.VloggerID)
	if err != nil {
		return false, err
	}
	return s.repo.IsFollowed(ctx, social)
}
