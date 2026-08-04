package v1

import (
	"feedsystem_ai_go/internal/controllers"
	"feedsystem_ai_go/internal/middlewares/jwt"
	"feedsystem_ai_go/internal/repositories"

	rediscache "feedsystem_ai_go/pkg/cache"

	"github.com/gin-gonic/gin"
)

func registerAIRoutes(r *gin.Engine, accountRepo *repositories.AccountRepository, cache *rediscache.Client, h *controllers.AIHandler) {
	aiGroup := r.Group("/ai")
	aiGroup.Use(jwt.JWTAuth(accountRepo, cache))
	{
		aiGroup.POST("/analyze", h.TriggerAnalysis)
		aiGroup.POST("/transcribe", h.TranscribeOnly)
		aiGroup.POST("/summarize", h.SummarizeText)
		aiGroup.GET("/status/:id", h.GetAnalysisStatus)
		aiGroup.GET("/audio/:id", h.DownloadAudio)
		aiGroup.GET("/config", h.GetConfig)
		aiGroup.POST("/config", h.UpdateConfig)
	}
}