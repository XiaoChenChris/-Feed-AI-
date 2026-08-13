package services

import (
	"context"
	"errors"
	"time"

	rediscache "feedsystem_ai_go/pkg/cache"
	"feedsystem_ai_go/internal/models"
	rabbitmq "feedsystem_ai_go/pkg/mq"
	"feedsystem_ai_go/internal/repositories"
)

type LikeService struct {
	repo         *repositories.LikeRepository
	VideoRepo    *repositories.VideoRepository
	cache        *rediscache.Client
	likeMQ       *rabbitmq.LikeMQ
	popularityMQ *rabbitmq.PopularityMQ
}

func NewLikeService(repo *repositories.LikeRepository, videoRepo *repositories.VideoRepository, cache *rediscache.Client, likeMQ *rabbitmq.LikeMQ, popularityMQ *rabbitmq.PopularityMQ) *LikeService {
	return &LikeService{repo: repo, VideoRepo: videoRepo, cache: cache, likeMQ: likeMQ, popularityMQ: popularityMQ}
}

func (s *LikeService) Like(ctx context.Context, like *models.Like) error {
	if like == nil {
		return errors.New("like is nil")
	}
	if like.VideoID == 0 || like.AccountID == 0 {
		return errors.New("video_id and account_id are required")
	}

	if s.VideoRepo != nil {
		ok, err := s.VideoRepo.IsExist(ctx, like.VideoID)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("video not found")
		}
	}

	isLiked, err := s.repo.IsLiked(ctx, like.VideoID, like.AccountID)
	if err != nil {
		return err
	}
	if isLiked {
		return errors.New("user has liked this video")
	}

	like.CreatedAt = time.Now()
	mysqlEnqueued := false
	redisEnqueued := false
	if s.likeMQ != nil {
		if err := s.likeMQ.Like(ctx, like.AccountID, like.VideoID); err == nil {
			mysqlEnqueued = true
		}
	}
	if s.popularityMQ != nil {
		if err := s.popularityMQ.Update(ctx, like.VideoID, 1); err == nil {
			redisEnqueued = true
		}
	}
	if mysqlEnqueued && redisEnqueued {
		return nil
	}

	// Fallback: direct MySQL write when like MQ publish fails.
	if !mysqlEnqueued {
		created, err := s.repo.LikeIgnoreDuplicate(ctx, like)
		if err != nil {
			return err
		}
		if !created {
			return errors.New("user has liked this video")
		}
		// 乐观锁更新计数（version 字段 CAS + 重试）
		if err := s.VideoRepo.ChangeLikesCount(ctx, like.VideoID, 1); err != nil {
			return err
		}
		if err := s.VideoRepo.ChangePopularity(ctx, like.VideoID, 1); err != nil {
			return err
		}
	}

	// Fallback: direct Redis update when popularity MQ publish fails.
	if !redisEnqueued {
		UpdatePopularityCache(ctx, s.cache, like.VideoID, 1)
	}
	return nil
}

func (s *LikeService) Unlike(ctx context.Context, like *models.Like) error {
	if like == nil {
		return errors.New("like is nil")
	}
	if like.VideoID == 0 || like.AccountID == 0 {
		return errors.New("video_id and account_id are required")
	}

	if s.VideoRepo != nil {
		ok, err := s.VideoRepo.IsExist(ctx, like.VideoID)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("video not found")
		}
	}

	isLiked, err := s.repo.IsLiked(ctx, like.VideoID, like.AccountID)
	if err != nil {
		return err
	}
	if !isLiked {
		return errors.New("user has not liked this video")
	}

	mysqlEnqueued := false
	redisEnqueued := false
	if s.likeMQ != nil {
		if err := s.likeMQ.Unlike(ctx, like.AccountID, like.VideoID); err == nil {
			mysqlEnqueued = true
		}
	}
	if s.popularityMQ != nil {
		if err := s.popularityMQ.Update(ctx, like.VideoID, -1); err == nil {
			redisEnqueued = true
		}
	}
	if mysqlEnqueued && redisEnqueued {
		return nil
	}

	// Fallback: direct MySQL write when like MQ publish fails.
	if !mysqlEnqueued {
		deleted, err := s.repo.DeleteByVideoAndAccount(ctx, like.VideoID, like.AccountID)
		if err != nil {
			return err
		}
		if !deleted {
			return errors.New("user has not liked this video")
		}
		// 乐观锁更新计数（version 字段 CAS + 重试）
		if err := s.VideoRepo.ChangeLikesCount(ctx, like.VideoID, -1); err != nil {
			return err
		}
		if err := s.VideoRepo.ChangePopularity(ctx, like.VideoID, -1); err != nil {
			return err
		}
	}

	// Fallback: direct Redis update when popularity MQ publish fails.
	if !redisEnqueued {
		UpdatePopularityCache(ctx, s.cache, like.VideoID, -1)
	}
	return nil
}

func (s *LikeService) IsLiked(ctx context.Context, videoID, accountID uint) (bool, error) {
	return s.repo.IsLiked(ctx, videoID, accountID)
}

func (s *LikeService) ListLikedVideos(ctx context.Context, accountID uint) ([]models.Video, error) {
	return s.repo.ListLikedVideos(ctx, accountID)
}
