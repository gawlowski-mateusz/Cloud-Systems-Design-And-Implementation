package cognitoauth

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"

	"neurosciolar/backend/internal/dynamostore"
	"neurosciolar/backend/internal/sharedauth"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	client     *cognitoidentityprovider.Client
	clientID   string
	userPoolID string
	profiles   *dynamostore.ProfileStore
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

func NewHandler(client *cognitoidentityprovider.Client, profiles *dynamostore.ProfileStore) *Handler {
	return &Handler{
		client:     client,
		clientID:   strings.TrimSpace(os.Getenv("COGNITO_CLIENT_ID")),
		userPoolID: strings.TrimSpace(os.Getenv("COGNITO_USER_POOL_ID")),
		profiles:   profiles,
	}
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

	signUp, err := h.client.SignUp(c.Request.Context(), &cognitoidentityprovider.SignUpInput{
		ClientId: aws.String(h.clientID),
		Username: aws.String(req.Email),
		Password: aws.String(req.Password),
		UserAttributes: []types.AttributeType{
			{Name: aws.String("email"), Value: aws.String(req.Email)},
			{Name: aws.String("name"), Value: aws.String(req.FullName)},
		},
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": cognitoError(err)})
		return
	}

	if _, err := h.client.AdminConfirmSignUp(c.Request.Context(), &cognitoidentityprovider.AdminConfirmSignUpInput{
		UserPoolId: aws.String(h.userPoolID),
		Username:   aws.String(req.Email),
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to auto-confirm user"})
		return
	}

	userSub := ""
	if signUp.UserSub != nil {
		userSub = *signUp.UserSub
	}

	if userSub != "" {
		_ = h.profiles.Put(c.Request.Context(), dynamostore.Profile{
			UserSub:  userSub,
			Email:    req.Email,
			FullName: req.FullName,
		})
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "user registered successfully",
		"user": gin.H{
			"userSub":  userSub,
			"email":    req.Email,
			"fullName": req.FullName,
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

	authOut, err := h.client.InitiateAuth(c.Request.Context(), &cognitoidentityprovider.InitiateAuthInput{
		AuthFlow: types.AuthFlowTypeUserPasswordAuth,
		ClientId: aws.String(h.clientID),
		AuthParameters: map[string]string{
			"USERNAME": req.Email,
			"PASSWORD": req.Password,
		},
	})
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": cognitoError(err)})
		return
	}
	if authOut.AuthenticationResult == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication challenge not supported"})
		return
	}

	idToken := aws.ToString(authOut.AuthenticationResult.IdToken)
	accessToken := aws.ToString(authOut.AuthenticationResult.AccessToken)
	refreshToken := aws.ToString(authOut.AuthenticationResult.RefreshToken)

	profile := lookupProfileByEmail(c.Request.Context(), h, req.Email)

	c.JSON(http.StatusOK, gin.H{
		"message":      "login successful",
		"idToken":      idToken,
		"accessToken":  accessToken,
		"refreshToken": refreshToken,
		"user": gin.H{
			"userSub":  profile.UserSub,
			"email":    profile.Email,
			"fullName": profile.FullName,
		},
	})
}

func (h *Handler) Me(c *gin.Context) {
	userSub, ok := sharedauth.UserSub(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	profile, err := h.profiles.Get(c.Request.Context(), userSub)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load profile"})
		return
	}
	if profile.UserSub == "" {
		profile.UserSub = userSub
		profile.Email = sharedauth.Email(c)
	}
	c.JSON(http.StatusOK, gin.H{"user": profile})
}

func lookupProfileByEmail(ctx context.Context, h *Handler, email string) dynamostore.Profile {
	out, err := h.client.AdminGetUser(ctx, &cognitoidentityprovider.AdminGetUserInput{
		UserPoolId: aws.String(h.userPoolID),
		Username:   aws.String(email),
	})
	if err != nil {
		return dynamostore.Profile{Email: email}
	}
	profile := dynamostore.Profile{Email: email}
	for _, attr := range out.UserAttributes {
		switch aws.ToString(attr.Name) {
		case "sub":
			profile.UserSub = aws.ToString(attr.Value)
		case "name":
			profile.FullName = aws.ToString(attr.Value)
		case "email":
			profile.Email = aws.ToString(attr.Value)
		}
	}
	if profile.UserSub != "" {
		if existing, _ := h.profiles.Get(ctx, profile.UserSub); existing.UserSub != "" {
			return existing
		}
		_ = h.profiles.Put(ctx, profile)
	}
	return profile
}

func cognitoError(err error) string {
	if err == nil {
		return ""
	}
	text := err.Error()
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "usernameexists"), strings.Contains(lower, "already exists"):
		return "user with this email already exists"
	case strings.Contains(lower, "notauthorizedexception"), strings.Contains(lower, "incorrect username"):
		return "invalid email or password"
	case strings.Contains(lower, "invalidpassword"):
		return "password does not meet complexity requirements"
	case strings.Contains(lower, "invalidparameter"):
		return "invalid input"
	}
	var apiErr interface{ ErrorMessage() string }
	if errors.As(err, &apiErr) {
		return apiErr.ErrorMessage()
	}
	return "authentication request failed"
}
