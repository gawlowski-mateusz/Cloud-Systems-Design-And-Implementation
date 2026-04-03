package main

import (
	"log"
	"net/http"
	"os"

	"neurosciolar/backend/internal/auth"
	"neurosciolar/backend/internal/database"
	"neurosciolar/backend/internal/media"
	"neurosciolar/backend/internal/reservation"
	"neurosciolar/backend/internal/storage"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	db, err := database.Connect()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := database.EnsureSchema(db); err != nil {
		log.Fatalf("Failed to initialize database schema: %v", err)
	}

	r := gin.Default()
	r.Use(cors.Default())

	authHandler := auth.NewHandler(db)
	reservationHandler := reservation.NewHandler(db)
	fileStore, err := storage.NewFromEnv()
	if err != nil {
		log.Fatalf("Failed to initialize file storage: %v", err)
	}
	mediaHandler := media.NewHandler(db, fileStore)

	api := r.Group("/api")
	{
		authGroup := api.Group("/auth")
		{
			authGroup.POST("/register", authHandler.Register)
			authGroup.POST("/login", authHandler.Login)
		}

		reservationGroup := api.Group("/reservations")
		reservationGroup.Use(auth.RequireAuth())
		{
			reservationGroup.GET("", reservationHandler.ListMine)
			reservationGroup.POST("", reservationHandler.Create)
		}

		mediaGroup := api.Group("/media")
		mediaGroup.Use(auth.RequireAuth())
		{
			mediaGroup.GET("", mediaHandler.ListMine)
			mediaGroup.GET("/:id", mediaHandler.Download)
			mediaGroup.POST("", mediaHandler.Upload)
		}
	}

	r.GET("/health", func(c *gin.Context) {
		if err := db.Ping(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "unhealthy",
				"db":     "disconnected",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status": "healthy",
			"db":     "connected",
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
