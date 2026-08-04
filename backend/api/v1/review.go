package v1

import (
	"feedsystem_ai_go/internal/controllers"
	"feedsystem_ai_go/internal/middlewares/admin"
	"feedsystem_ai_go/internal/middlewares/jwt"
	"feedsystem_ai_go/internal/repositories"

	rediscache "feedsystem_ai_go/pkg/cache"

	"github.com/gin-gonic/gin"
)

func registerReviewRoutes(r *gin.Engine, accountRepo *repositories.AccountRepository, cache *rediscache.Client, h *controllers.ReviewHandler) {
	reviewGroup := r.Group("/review")
	reviewGroup.Use(jwt.JWTAuth(accountRepo, cache))
	{
		reviewGroup.GET("/config", h.GetReviewConfig)
		reviewGroup.GET("/status/:videoId", h.GetVideoReviewStatus)
		reviewGroup.POST("/resubmit", h.ReSubmitVideo)
	}

	// Admin-only review endpoints
	adminReviewGroup := r.Group("/review")
	adminReviewGroup.Use(jwt.JWTAuth(accountRepo, cache))
	adminReviewGroup.Use(admin.RequireAdmin())
	{
		adminReviewGroup.POST("/config", h.UpdateReviewConfig)
		adminReviewGroup.POST("/video/:id/approve", h.ApproveVideo)
		adminReviewGroup.POST("/video/:id/reject", h.RejectVideo)
		adminReviewGroup.GET("/pending", h.GetPendingVideos)
	}
}