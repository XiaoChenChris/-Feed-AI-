package v1

import (
	"context"
	"log"

	"feedsystem_ai_go/internal/controllers"
	"feedsystem_ai_go/internal/middlewares/jwt"
	"feedsystem_ai_go/internal/repositories"
	"feedsystem_ai_go/internal/worker"

	rabbitmq "feedsystem_ai_go/pkg/mq"
	rediscache "feedsystem_ai_go/pkg/cache"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func registerNotificationRoutes(r *gin.Engine, accountRepo *repositories.AccountRepository, cache *rediscache.Client,
	rmq *rabbitmq.RabbitMQ, messageHandler *controllers.MessageHandler, db *gorm.DB) {

	// message routes
	messageGroup := r.Group("/message")
	protected := messageGroup.Group("")
	protected.Use(jwt.JWTAuth(accountRepo, cache))
	{
		protected.POST("/send", messageHandler.Send)
		protected.POST("/list", messageHandler.List)
	}
}

func registerSSEAndWorkers(r *gin.Engine, accountRepo *repositories.AccountRepository, cache *rediscache.Client,
	rmq *rabbitmq.RabbitMQ, deps Deps) {

	// SSE notification
	if rmq != nil && rmq.Ch != nil {
		rmq.DeclareTopic("like.events", "notification.like", "like.like")
		rmq.DeclareTopic("comment.events", "notification.comment", "comment.publish")
		rmq.DeclareTopic("social.events", "notification.social", "social.follow")
	}
	sseHub := worker.NewSSEHub(deps.DB)
	notifGroup := r.Group("/notification")
	notifGroup.Use(sseHub.SSERequireAuth())
	sseHub.RegisterRoutes(r, notifGroup)

	go func() {
		if rmq != nil && rmq.Ch != nil {
			hub := sseHub
			ctx := context.Background()
			go func() {
				w := worker.NewNotificationWorker(rmq.Ch, deps.DB, "notification.like", hub)
				if err := w.Run(ctx); err != nil {
					log.Printf("notification-like worker: %v", err)
				}
			}()
			go func() {
				w := worker.NewNotificationWorker(rmq.Ch, deps.DB, "notification.comment", hub)
				if err := w.Run(ctx); err != nil {
					log.Printf("notification-comment worker: %v", err)
				}
			}()
			go func() {
				w := worker.NewNotificationWorker(rmq.Ch, deps.DB, "notification.social", hub)
				if err := w.Run(ctx); err != nil {
					log.Printf("notification-social worker: %v", err)
				}
			}()
		} else {
			log.Printf("Notification SSE disabled (MQ not available)")
		}
	}()
}