package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"neurosciolar/backend/internal/awsclients"
	"neurosciolar/backend/internal/dynamostore"
	"neurosciolar/backend/internal/notifications"
	"neurosciolar/backend/internal/sharedauth"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := awsclients.LoadConfig(context.Background())
	if err != nil {
		log.Fatalf("failed to load aws config: %v", err)
	}
	dynamoClient := dynamodb.NewFromConfig(cfg)
	snsClient := sns.NewFromConfig(cfg)

	table := requireEnv("DYNAMO_NOTIFICATIONS_TABLE")
	topicARN := strings.TrimSpace(os.Getenv("SNS_TOPIC_ARN"))

	store := dynamostore.NewNotificationStore(dynamoClient, table)
	publisher := notifications.NewPublisher(snsClient, topicARN)
	handler := notifications.NewHandler(store, publisher)

	validator, err := sharedauth.NewValidator()
	if err != nil {
		log.Fatalf("failed to init cognito validator: %v", err)
	}

	r := newRouter()
	r.GET("/notifications/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "notifications"})
	})
	// SNS HTTPS subscription endpoint — unauthenticated by design (signed by SNS).
	r.POST("/notifications/sns", handler.Subscribe)

	authed := r.Group("/notifications")
	authed.Use(validator.Middleware())
	authed.GET("", handler.ListMine)
	authed.POST("", handler.Broadcast)

	port := envOr("PORT", "8080")
	log.Printf("notifications service listening on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func newRouter() *gin.Engine {
	r := gin.Default()
	r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length", "Content-Type"},
		AllowCredentials: false,
	}))
	return r
}

func requireEnv(key string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		log.Fatalf("environment variable %s is required", key)
	}
	return v
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
