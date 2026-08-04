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

func registerSocialRoutes(r *gin.Engine, accountRepo *repositories.AccountRepository, cache *rediscache.Client, h *controllers.SocialHandler) {
	socialLimiter := ratelimit.Limit(cache, "social_write", 20, time.Minute, ratelimit.KeyByAccount)

	socialGroup := r.Group("/social")
	protected := socialGroup.Group("")
	protected.Use(jwt.JWTAuth(accountRepo, cache))
	{
		protected.POST("/follow", socialLimiter, h.Follow)
		protected.POST("/unfollow", socialLimiter, h.Unfollow)
		protected.POST("/getAllFollowers", h.GetAllFollowers)
		protected.POST("/getAllVloggers", h.GetAllVloggers)
		protected.POST("/getCounts", h.GetCounts)
	}
}