package v1

import (
	"log"
	"time"

	rediscache "feedsystem_ai_go/pkg/cache"
	"feedsystem_ai_go/pkg/config"
	"feedsystem_ai_go/internal/controllers"
	"feedsystem_ai_go/internal/middlewares/admin"
	rabbitmq "feedsystem_ai_go/pkg/mq"
	"feedsystem_ai_go/internal/repositories"
	"feedsystem_ai_go/internal/services"
	review "feedsystem_ai_go/internal/services/review"
	"feedsystem_ai_go/internal/services/review/agent"
	"feedsystem_ai_go/internal/worker"
	"feedsystem_ai_go/pkg/storage"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// Deps holds top-level infrastructure dependencies
type Deps struct {
	DB    *gorm.DB
	Cache *rediscache.Client
	RDB   *redis.Client
	MQ    *rabbitmq.RabbitMQ
	Cfg   config.Config
}

func SetRouter(db *gorm.DB, cache *rediscache.Client, rdb *redis.Client, rmq *rabbitmq.RabbitMQ, cfg config.Config) *gin.Engine {
	r := gin.Default()
	if err := r.SetTrustedProxies(nil); err != nil {
		log.Printf("SetTrustedProxies failed: %v", err)
	}
	if len(cfg.Server.AdminIDs) > 0 {
		admin.SetAdminIDs(cfg.Server.AdminIDs)
	}
	r.Static("/static", "./.run/uploads")

	deps := Deps{DB: db, Cache: cache, RDB: rdb, MQ: rmq, Cfg: cfg}

	// ─── Account ──────────────────────────────────────────────
	accountRepo, accountService := newAccountComponents(deps)

	// ─── Review Service (shared) ──────────────────────────────
	reviewCfg := review.ReviewConfig{
		Enabled:               cfg.Review.Enabled,
		TextModel:             cfg.Review.TextModel,
		VisionModel:           cfg.Review.VisionModel,
		SampleFrames:          cfg.Review.SampleFrames,
		FrameReviewMode:       cfg.Review.FrameReviewMode,
		ConfidenceThreshold:   cfg.Review.ConfidenceThreshold,
		ManualReviewThreshold: cfg.Review.ManualReviewThreshold,
		MaxRetries:            cfg.Review.MaxRetries,
		APIKey:                cfg.AI.APIKey,
		BaseURL:               cfg.AI.BaseURL,
		MaxVideoSizeMB:        cfg.Review.MaxVideoSizeMB,
		MaxCoverSizeMB:        cfg.Review.MaxCoverSizeMB,
		MaxVideoDurationSec:   cfg.Review.MaxVideoDurationSec,
		MinVideoDurationSec:   cfg.Review.MinVideoDurationSec,
		EnableAudioReview:     cfg.Review.EnableAudioReview,
		EnableOCRReview:       cfg.Review.EnableOCRReview,
		MaxConcurrentFrames:   cfg.Review.MaxConcurrentFrames,
		MaxConcurrentVideos:   cfg.Review.MaxConcurrentVideos,
		AgentEnabled:          cfg.Review.AgentEnabled,
		AgentMaxRounds:        cfg.Review.AgentMaxRounds,
		AgentTimeoutSec:       cfg.Review.AgentTimeoutSec,
	}
	reviewService := review.NewReviewService(reviewCfg)

	// ─── Video + Like + Comment ───────────────────────────────
	videoRepo := repositories.NewVideoRepository(db)
	popularityMQ, err := rabbitmq.NewPopularityMQ(rmq)
	if err != nil {
		log.Printf("PopularityMQ init failed (mq disabled): %v", err)
		popularityMQ = nil
	}
	videoService := services.NewVideoService(videoRepo, cache, popularityMQ)
	videoService.SetReviewService(reviewService)

	likeRepo := repositories.NewLikeRepository(db)
	likeMQ, err := rabbitmq.NewLikeMQ(rmq)
	if err != nil {
		log.Printf("LikeMQ init failed (mq disabled): %v", err)
		likeMQ = nil
	}
	likeService := services.NewLikeService(likeRepo, videoRepo, cache, likeMQ, popularityMQ)

	commentRepo := repositories.NewCommentRepository(db)
	commentMQ, err := rabbitmq.NewCommentMQ(rmq)
	if err != nil {
		log.Printf("CommentMQ init failed (mq disabled): %v", err)
		commentMQ = nil
	}
	commentService := services.NewCommentService(commentRepo, videoRepo, cache, commentMQ, popularityMQ)
	commentService.SetReviewService(reviewService)

	// ─── AI Service (used by agent + AIHandler) ───────────────
	aiService := services.NewAIService(cfg.AI, cfg.Media)

	// ─── Publishing Agent ─────────────────────────────────────
	if reviewService.IsEnabled() && reviewCfg.AgentEnabled {
		engineCfg := agent.EngineConfig{
			Model:     reviewCfg.TextModel,
			BaseURL:   reviewCfg.BaseURL,
			APIKey:    reviewCfg.APIKey,
			MaxRounds: reviewCfg.AgentMaxRounds,
			Timeout:   time.Duration(reviewCfg.AgentTimeoutSec) * time.Second,
		}
		agentDeps := agent.ToolboxDeps{
			ReviewService: reviewService,
			AIService:     aiService,
		}
		publishingAgent := agent.NewPublishingAgent(reviewService, engineCfg, agentDeps)
		videoService.SetPublishingAgent(publishingAgent)
		log.Println("[Agent] PublishingAgent started")
	}

	// ─── Social ───────────────────────────────────────────────
	socialRepo := repositories.NewSocialRepository(db)
	socialMQ, err := rabbitmq.NewSocialMQ(rmq)
	if err != nil {
		log.Printf("SocialMQ init failed (mq disabled): %v", err)
		socialMQ = nil
	}
	socialService := services.NewSocialService(socialRepo, accountRepo, socialMQ)

	// ─── Feed ─────────────────────────────────────────────────
	feedRepo := repositories.NewFeedRepository(db)
	feedService := services.NewFeedService(feedRepo, likeRepo, cache)

	// ─── Create Handlers ──────────────────────────────────────
	accountHandler := controllers.NewAccountHandler(accountService, videoRepo, socialRepo, cfg.Server.AdminIDs)
	videoHandler := controllers.NewVideoHandler(videoService, accountService)
	likeHandler := controllers.NewLikeHandler(likeService)
	commentHandler := controllers.NewCommentHandler(commentService, accountService)
	socialHandler := controllers.NewSocialHandler(socialService)
	feedHandler := controllers.NewFeedHandler(feedService)
	aiHandler := controllers.NewAIHandler(db, aiService, rdb)
	aiHandler.SetReviewService(reviewService)
	reviewHandler := controllers.NewReviewHandler(db, reviewService, videoService)

	// ─── Register Routes ──────────────────────────────────────
	registerAccountRoutes(r, accountRepo, cache, accountHandler)
	registerVideoRoutes(r, accountRepo, cache, videoHandler, likeHandler, commentHandler)
	registerFeedRoutes(r, accountRepo, cache, feedHandler)
	registerSocialRoutes(r, accountRepo, cache, socialHandler)
	registerAIRoutes(r, accountRepo, cache, aiHandler)
	registerReviewRoutes(r, accountRepo, cache, reviewHandler)

	// ─── Message + SSE ────────────────────────────────────────
	messageRepo := controllers.NewMessageRepository(db)
	messageService := controllers.NewMessageService(messageRepo)
	messageHandler := controllers.NewMessageHandler(messageService)
	registerNotificationRoutes(r, accountRepo, cache, rmq, messageHandler, db)
	registerSSEAndWorkers(r, accountRepo, cache, rmq, deps)

	// ─── Media ────────────────────────────────────────────────
	minioClient, minioErr := storage.NewMinIOClient(cfg.MinIO)
	if minioErr == nil {
		mediaService := services.NewMediaService(db, rdb, minioClient, cfg.Media)
		mediaHandler := controllers.NewMediaHandler(mediaService, rdb)
		registerMediaRoutes(r, accountRepo, cache, mediaHandler)
	} else {
		log.Printf("MinIO not available, media upload disabled: %v", minioErr)
	}

	// ─── Outbox Worker ────────────────────────────────────────
	timelineMQ, err := rabbitmq.NewTimelineMQ(rmq)
	if err != nil {
		log.Printf("timelineMQ init failed (mq disabled): %v", err)
		timelineMQ = nil
	}
	worker.StartOutboxPoller(db, timelineMQ)
	worker.StartConsumer(timelineMQ, "video.timeline.update.queue", cache)

	// ─── Post-Review Worker ───────────────────────────────────
	if reviewService.IsEnabled() && reviewCfg.AgentEnabled {
		engineCfg := agent.EngineConfig{
			Model:     reviewCfg.TextModel,
			BaseURL:   reviewCfg.BaseURL,
			APIKey:    reviewCfg.APIKey,
			MaxRounds: reviewCfg.AgentMaxRounds,
			Timeout:   time.Duration(reviewCfg.AgentTimeoutSec) * time.Second,
		}
		agentDeps := agent.ToolboxDeps{
			ReviewService: reviewService,
			AIService:     aiService,
		}
		triageAgent := agent.NewTriageAgent(engineCfg, agentDeps)
		postReviewAgent := agent.NewPostReviewAgent(engineCfg, agentDeps, triageAgent)
		reviewWorker := worker.NewReviewWorker(db, reviewService, postReviewAgent)
		reviewWorker.Start()
		log.Println("[Agent] PostReviewAgent started")
	} else if reviewService.IsEnabled() {
		reviewWorker := worker.NewReviewWorker(db, reviewService, nil)
		reviewWorker.Start()
	}

	return r
}
