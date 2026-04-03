package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	db              *sql.DB
	authProvider    string
	cognitoClient   *cognitoidentityprovider.Client
	cognitoClientID string
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
	h := &Handler{db: db}

	authProvider := strings.ToLower(strings.TrimSpace(os.Getenv("AUTH_PROVIDER")))
	if authProvider == "" {
		authProvider = "local"
	}
	h.authProvider = authProvider

	if authProvider == "cognito" {
		region := strings.TrimSpace(os.Getenv("COGNITO_REGION"))
		clientID := strings.TrimSpace(os.Getenv("COGNITO_CLIENT_ID"))
		if region != "" && clientID != "" {
			cfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(region))
			if err == nil {
				h.cognitoClient = cognitoidentityprovider.NewFromConfig(cfg)
				h.cognitoClientID = clientID
			}
		}
	}

	return h
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

	if h.authProvider == "cognito" {
		if err := h.registerWithCognito(c.Request.Context(), req.FullName, req.Email, req.Password); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		userID, err := h.createOrGetLocalUser(req.FullName, req.Email, "COGNITO")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create local user profile"})
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

	if h.authProvider == "cognito" {
		fullName, err := h.loginWithCognito(c.Request.Context(), req.Email, req.Password)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		if fullName == "" {
			fullName = strings.Split(req.Email, "@")[0]
		}

		userID, err := h.createOrGetLocalUser(fullName, req.Email, "COGNITO")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to initialize local user"})
			return
		}

		token, err := h.generateToken(userID, fullName, req.Email)
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
				"email":    req.Email,
			},
		})
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

func (h *Handler) registerWithCognito(ctx context.Context, fullName, email, password string) error {
	if h.cognitoClient == nil || h.cognitoClientID == "" {
		return fmt.Errorf("cognito is not configured on the backend")
	}

	_, err := h.cognitoClient.SignUp(ctx, &cognitoidentityprovider.SignUpInput{
		ClientId: &h.cognitoClientID,
		Username: &email,
		Password: &password,
		UserAttributes: []types.AttributeType{
			{Name: strPtr("email"), Value: strPtr(email)},
			{Name: strPtr("name"), Value: strPtr(fullName)},
		},
	})
	if err != nil {
		errText := strings.ToLower(err.Error())
		if strings.Contains(errText, "exists") || strings.Contains(errText, "already") {
			return fmt.Errorf("user with this email already exists")
		}
		return fmt.Errorf("failed to register user in cognito")
	}

	return nil
}

func (h *Handler) loginWithCognito(ctx context.Context, email, password string) (string, error) {
	if h.cognitoClient == nil || h.cognitoClientID == "" {
		return "", fmt.Errorf("cognito is not configured on the backend")
	}

	_, err := h.cognitoClient.InitiateAuth(ctx, &cognitoidentityprovider.InitiateAuthInput{
		AuthFlow: types.AuthFlowTypeUserPasswordAuth,
		ClientId: &h.cognitoClientID,
		AuthParameters: map[string]string{
			"USERNAME": email,
			"PASSWORD": password,
		},
	})
	if err != nil {
		return "", fmt.Errorf("invalid email or password")
	}

	return "", nil
}

func (h *Handler) createOrGetLocalUser(fullName, email, passwordHash string) (int64, error) {
	var userID int64
	err := h.db.QueryRow(`SELECT id FROM users WHERE email = $1`, email).Scan(&userID)
	if err == nil {
		if _, updateErr := h.db.Exec(`UPDATE users SET full_name = $1 WHERE id = $2`, fullName, userID); updateErr != nil {
			return 0, updateErr
		}
		return userID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}

	err = h.db.QueryRow(
		`INSERT INTO users (full_name, email, password_hash) VALUES ($1, $2, $3) RETURNING id`,
		fullName,
		email,
		passwordHash,
	).Scan(&userID)

	return userID, err
}

func strPtr(value string) *string {
	return &value
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
