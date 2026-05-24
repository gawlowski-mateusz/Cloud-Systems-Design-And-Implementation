package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"neurosciolar/backend/internal/awsclients"
	"neurosciolar/backend/internal/cognitoauth"
	"neurosciolar/backend/internal/dynamostore"
	"neurosciolar/backend/internal/sharedauth"

	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg, err := awsclients.LoadConfig(context.Background())
	if err != nil {
		log.Fatalf("failed to load aws config: %v", err)
	}
	cognitoClient := cognitoidentityprovider.NewFromConfig(cfg)
	dynamoClient := dynamodb.NewFromConfig(cfg)

	profilesTable := requireEnv("DYNAMO_PROFILES_TABLE")
	profileStore := dynamostore.NewProfileStore(dynamoClient, profilesTable)

	authHandler := cognitoauth.NewHandler(cognitoClient, profileStore)

	validator, err := sharedauth.NewValidator()
	if err != nil {
		log.Fatalf("failed to init cognito validator: %v", err)
	}

	r := newRouter()
	r.GET("/auth/health", healthHandler)
	r.POST("/auth/register", authHandler.Register)
	r.POST("/auth/login", authHandler.Login)
	authed := r.Group("/auth")
	authed.Use(validator.Middleware())
	authed.GET("/me", authHandler.Me)

	port := envOr("PORT", "8080")
	log.Printf("auth service listening on :%s", port)
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

func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "auth"})
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
