package v1

import (
	"feedsystem_ai_go/internal/controllers"
	"feedsystem_ai_go/internal/middlewares/jwt"
	"feedsystem_ai_go/internal/repositories"

	rediscache "feedsystem_ai_go/pkg/cache"

	"github.com/gin-gonic/gin"
)

func registerMediaRoutes(r *gin.Engine, accountRepo *repositories.AccountRepository, cache *rediscache.Client, h *controllers.MediaHandler) {
	mediaGroup := r.Group("/media")
	mediaGroup.Use(jwt.JWTAuth(accountRepo, cache))
	{
		mediaGroup.POST("/init-upload", h.InitUpload)
		mediaGroup.POST("/upload", h.Upload)
		mediaGroup.POST("/upload-chunk", h.UploadChunk)
		mediaGroup.POST("/complete-upload", h.CompleteChunkUpload)
		mediaGroup.GET("/list", h.List)
		mediaGroup.DELETE("/delete", h.Delete)
		mediaGroup.POST("/check-duplicate", h.CheckDuplicate)
	}
}