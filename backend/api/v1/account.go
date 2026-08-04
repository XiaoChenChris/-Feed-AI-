package v1

import (
	"time"

	"feedsystem_ai_go/internal/controllers"
	"feedsystem_ai_go/internal/middlewares/jwt"
	"feedsystem_ai_go/internal/middlewares/ratelimit"
	"feedsystem_ai_go/internal/repositories"
	"feedsystem_ai_go/internal/services"

	rediscache "feedsystem_ai_go/pkg/cache"

	"github.com/gin-gonic/gin"
)

func registerAccountRoutes(r *gin.Engine, accountRepo *repositories.AccountRepository, cache *rediscache.Client, h *controllers.AccountHandler) {
	loginLimiter := ratelimit.Limit(cache, "account_login", 10, time.Minute, ratelimit.KeyByIP)
	registerLimiter := ratelimit.Limit(cache, "account_register", 5, time.Hour, ratelimit.KeyByIP)

	accountGroup := r.Group("/account")
	{
		accountGroup.POST("/register", registerLimiter, h.CreateAccount)
		accountGroup.POST("/login", loginLimiter, h.Login)
		accountGroup.POST("/changePassword", h.ChangePassword)
		accountGroup.POST("/findByID", h.FindByID)
		accountGroup.POST("/findByUsername", h.FindByUsername)
		accountGroup.POST("/refresh", h.Refresh)
		accountGroup.POST("/getProfile", h.GetProfile)
	}
	protected := accountGroup.Group("")
	protected.Use(jwt.JWTAuth(accountRepo, cache))
	{
		protected.POST("/logout", h.Logout)
		protected.POST("/rename", h.Rename)
		protected.POST("/uploadAvatar", h.UploadAvatar)
		protected.POST("/updateProfile", h.UpdateProfile)
	}
}

// registerAccountService also wires the account service to be available
func newAccountComponents(deps Deps) (*repositories.AccountRepository, *services.AccountService) {
	accountRepo := repositories.NewAccountRepository(deps.DB)
	accountService := services.NewAccountService(accountRepo, deps.Cache)
	return accountRepo, accountService
}