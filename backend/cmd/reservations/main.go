package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"neurosciolar/backend/internal/awsclients"
	"neurosciolar/backend/internal/notifications"
	"neurosciolar/backend/internal/rdsdb"
	"neurosciolar/backend/internal/reservations"
	"neurosciolar/backend/internal/sharedauth"

	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	db, err := rdsdb.Connect()
	if err != nil {
		log.Fatalf("failed to connect to db: %v", err)
	}
	defer db.Close()
	if err := rdsdb.EnsureReservationsSchema(db); err != nil {
		log.Fatalf("failed to ensure schema: %v", err)
	}

	cfg, err := awsclients.LoadConfig(context.Background())
	if err != nil {
		log.Fatalf("failed to load aws config: %v", err)
	}
	snsClient := sns.NewFromConfig(cfg)
	topicARN := strings.TrimSpace(os.Getenv("SNS_TOPIC_ARN"))
	publisher := notifications.NewPublisher(snsClient, topicARN)

	handler := reservations.NewHandler(db, publisher)

	validator, err := sharedauth.NewValidator()
	if err != nil {
		log.Fatalf("failed to init cognito validator: %v", err)
	}

	r := newRouter()
	r.GET("/reservations/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "reservations"})
	})
	authed := r.Group("/reservations")
	authed.Use(validator.Middleware())
	authed.GET("", handler.ListMine)
	authed.GET("/:id", handler.GetOne)
	authed.POST("", handler.Create)

	port := envOr("PORT", "8080")
	log.Printf("reservations service listening on :%s", port)
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

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
