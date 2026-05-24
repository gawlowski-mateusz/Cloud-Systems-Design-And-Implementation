package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"neurosciolar/backend/internal/awsclients"
	"neurosciolar/backend/internal/dynamostore"
	"neurosciolar/backend/internal/files"
	"neurosciolar/backend/internal/notifications"
	"neurosciolar/backend/internal/sharedauth"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := awsclients.LoadConfig(context.Background())
	if err != nil {
		log.Fatalf("failed to load aws config: %v", err)
	}
	s3Client := s3.NewFromConfig(cfg)
	dynamoClient := dynamodb.NewFromConfig(cfg)
	snsClient := sns.NewFromConfig(cfg)

	bucket := requireEnv("S3_MEDIA_BUCKET")
	metadataTable := requireEnv("DYNAMO_FILES_TABLE")
	topicARN := strings.TrimSpace(os.Getenv("SNS_TOPIC_ARN"))

	store := dynamostore.NewFileMetadataStore(dynamoClient, metadataTable)
	publisher := notifications.NewPublisher(snsClient, topicARN)
	handler := files.NewHandler(s3Client, bucket, store, publisher)

	validator, err := sharedauth.NewValidator()
	if err != nil {
		log.Fatalf("failed to init cognito validator: %v", err)
	}

	r := newRouter()
	r.GET("/files/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "files"})
	})
	authed := r.Group("/files")
	authed.Use(validator.Middleware())
	authed.GET("", handler.ListMine)
	authed.GET("/:id", handler.Download)
	authed.POST("", handler.Upload)

	port := envOr("PORT", "8080")
	log.Printf("files service listening on :%s", port)
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
