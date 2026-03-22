package auth

import (
	"database/sql"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	db *sql.DB
}

type registerRequest struct {
	FullName string `json:"fullName"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginClaims struct {
	UserID   int64  `json:"userId"`
	FullName string `json:"fullName"`
	Email    string `json:"email"`
	jwt.RegisteredClaims
}

func NewHandler(db *sql.DB) *Handler {
	return &Handler{db: db}
}

func (h *Handler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	req.FullName = strings.TrimSpace(req.FullName)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	if req.FullName == "" || req.Email == "" || len(req.Password) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "fullName, email and password (min 8 chars) are required"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process password"})
		return
	}

	var userID int64
	err = h.db.QueryRow(
		`INSERT INTO users (full_name, email, password_hash) VALUES ($1, $2, $3) RETURNING id`,
		req.FullName,
		req.Email,
		string(hashedPassword),
	).Scan(&userID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			c.JSON(http.StatusConflict, gin.H{"error": "user with this email already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "user registered successfully",
		"user": gin.H{
			"id":       userID,
			"fullName": req.FullName,
			"email":    req.Email,
		},
	})
}

func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email and password are required"})
		return
	}

	var userID int64
	var fullName string
	var email string
	var passwordHash string

	err := h.db.QueryRow(
		`SELECT id, full_name, email, password_hash FROM users WHERE email = $1`,
		req.Email,
	).Scan(&userID, &fullName, &email, &passwordHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to authenticate user"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}

	token, err := h.generateToken(userID, fullName, email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "login successful",
		"token":   token,
		"user": gin.H{
			"id":       userID,
			"fullName": fullName,
			"email":    email,
		},
	})
}

func (h *Handler) generateToken(userID int64, fullName, email string) (string, error) {
	now := time.Now()
	claims := loginClaims{
		UserID:   userID,
		FullName: fullName,
		Email:    email,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "conference-reservation-backend",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
			Subject:   email,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "dev-secret-change-me"
	}

	return token.SignedString([]byte(secret))
}
