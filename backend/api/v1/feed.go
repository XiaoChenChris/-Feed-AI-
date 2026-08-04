package v1

import (
	"feedsystem_ai_go/internal/controllers"
	"feedsystem_ai_go/internal/middlewares/jwt"
	"feedsystem_ai_go/internal/repositories"

	rediscache "feedsystem_ai_go/pkg/cache"

	"github.com/gin-gonic/gin"
)

func registerFeedRoutes(r *gin.Engine, accountRepo *repositories.AccountRepository, cache *rediscache.Client, h *controllers.FeedHandler) {
	feedGroup := r.Group("/feed")
	feedGroup.Use(jwt.SoftJWTAuth(accountRepo, cache))
	{
		feedGroup.POST("/listLatest", h.ListLatest)
		feedGroup.POST("/listLikesCount", h.ListLikesCount)
		feedGroup.POST("/listByPopularity", h.ListByPopularity)
		feedGroup.POST("/listByTag", h.ListByTag)
	}
	protected := feedGroup.Group("")
	protected.Use(jwt.JWTAuth(accountRepo, cache))
	{
		protected.POST("/listByFollowing", h.ListByFollowing)
	}
}