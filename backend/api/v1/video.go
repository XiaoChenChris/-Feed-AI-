package v1

import (
	"feedsystem_ai_go/internal/controllers"
	"feedsystem_ai_go/internal/middlewares/jwt"
	"feedsystem_ai_go/internal/middlewares/ratelimit"
	"feedsystem_ai_go/internal/repositories"
	"time"

	rediscache "feedsystem_ai_go/pkg/cache"

	"github.com/gin-gonic/gin"
)

func registerVideoRoutes(r *gin.Engine, accountRepo *repositories.AccountRepository, cache *rediscache.Client,
	vh *controllers.VideoHandler, lh *controllers.LikeHandler, ch *controllers.CommentHandler) {

	likeLimiter := ratelimit.Limit(cache, "like_write", 30, time.Minute, ratelimit.KeyByAccount)
	commentLimiter := ratelimit.Limit(cache, "comment_write", 10, time.Minute, ratelimit.KeyByAccount)

	// video routes
	videoGroup := r.Group("/video")
	{
		videoGroup.POST("/listByAuthorID", vh.ListByAuthorID)
		videoGroup.POST("/getDetail", vh.GetDetail)
	}
	protectedVideo := videoGroup.Group("")
	protectedVideo.Use(jwt.JWTAuth(accountRepo, cache))
	{
		protectedVideo.POST("/uploadVideo", vh.UploadVideo)
		protectedVideo.POST("/uploadCover", vh.UploadCover)
		protectedVideo.POST("/publish", vh.PublishVideo)
		protectedVideo.POST("/delete", vh.DeleteVideo)
	}

	// like routes
	likeGroup := r.Group("/like")
	protectedLike := likeGroup.Group("")
	protectedLike.Use(jwt.JWTAuth(accountRepo, cache))
	{
		protectedLike.POST("/like", likeLimiter, lh.Like)
		protectedLike.POST("/unlike", likeLimiter, lh.Unlike)
		protectedLike.POST("/isLiked", lh.IsLiked)
		protectedLike.POST("/listMyLikedVideos", lh.ListMyLikedVideos)
	}

	// comment routes
	commentGroup := r.Group("/comment")
	{
		commentGroup.POST("/listAll", ch.GetAllComments)
	}
	protectedComment := commentGroup.Group("")
	protectedComment.Use(jwt.JWTAuth(accountRepo, cache))
	{
		protectedComment.POST("/publish", commentLimiter, ch.PublishComment)
		protectedComment.POST("/delete", commentLimiter, ch.DeleteComment)
	}
}