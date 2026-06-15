package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"neurosciolar/backend/internal/awsclients"
	"neurosciolar/backend/internal/dynamostore"
	"neurosciolar/backend/internal/emailnotify"
	"neurosciolar/backend/internal/notifications"
	"neurosciolar/backend/internal/sharedauth"
	"neurosciolar/backend/internal/snspub"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := awsclients.LoadConfig(context.Background())
	if err != nil {
		log.Fatalf("failed to load aws config: %v", err)
	}
	dynamoClient := dynamodb.NewFromConfig(cfg)
	sqsClient := sqs.NewFromConfig(cfg)

	table := requireEnv("DYNAMO_NOTIFICATIONS_TABLE")
	queueURL := requireEnv("SQS_QUEUE_URL")

	store := dynamostore.NewNotificationStore(dynamoClient, table)
	handler := notifications.NewHandler(store)

	// Optional email delivery. When NOTIFICATIONS_TOPIC_ARN is unset the service
	// still works in pull mode (history fetched via GET /notifications).
	notifier := buildNotifier(cfg)

	// The consumer runs for the lifetime of the task; cancelled when main exits.
	consumerCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go notifications.NewConsumer(sqsClient, queueURL, store, notifier).Run(consumerCtx)

	validator, err := sharedauth.NewValidator()
	if err != nil {
		log.Fatalf("failed to init cognito validator: %v", err)
	}

	r := newRouter()
	r.GET("/notifications/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "notifications"})
	})

	authed := r.Group("/notifications")
	authed.Use(validator.Middleware())
	authed.GET("", handler.ListMine)

	port := envOr("PORT", "8080")
	log.Printf("notifications service listening on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// buildNotifier wires the SNS publisher used to email the subscribed recipient.
// Returns nil when email delivery is not configured.
func buildNotifier(cfg aws.Config) notifications.Notifier {
	topicARN := strings.TrimSpace(os.Getenv("NOTIFICATIONS_TOPIC_ARN"))
	if topicARN == "" {
		log.Printf("email notifications disabled: NOTIFICATIONS_TOPIC_ARN not set")
		return nil
	}
	return emailnotify.New(snspub.New(sns.NewFromConfig(cfg), topicARN))
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
