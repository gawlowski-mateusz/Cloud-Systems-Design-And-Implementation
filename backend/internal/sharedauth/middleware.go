package sharedauth

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const ContextUserSubKey = "user_sub"
const ContextEmailKey = "user_email"

type Validator struct {
	keys     *KeySet
	issuer   string
	clientID string
}

func NewValidator() (*Validator, error) {
	region := strings.TrimSpace(os.Getenv("AWS_REGION"))
	if region == "" {
		region = "us-east-1"
	}
	pool := strings.TrimSpace(os.Getenv("COGNITO_USER_POOL_ID"))
	if pool == "" {
		return nil, errors.New("COGNITO_USER_POOL_ID is required")
	}
	clientID := strings.TrimSpace(os.Getenv("COGNITO_CLIENT_ID"))
	if clientID == "" {
		return nil, errors.New("COGNITO_CLIENT_ID is required")
	}
	issuer := fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s", region, pool)
	v := &Validator{
		keys:     NewKeySet(region, pool),
		issuer:   issuer,
		clientID: clientID,
	}
	// Best-effort warm-up; failure is non-fatal because Refresh will retry on first request.
	_ = v.keys.Refresh()
	return v, nil
}

func (v *Validator) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			return
		}
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header"})
			return
		}
		raw := strings.TrimSpace(parts[1])
		if raw == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "empty bearer token"})
			return
		}

		claims := jwt.MapClaims{}
		token, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %s", t.Method.Alg())
			}
			kid, _ := t.Header["kid"].(string)
			if kid == "" {
				return nil, errors.New("token missing kid header")
			}
			return v.keys.Key(kid)
		})
		if err != nil || token == nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		if iss, _ := claims["iss"].(string); iss != v.issuer {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token issuer"})
			return
		}
		if tokenUse, _ := claims["token_use"].(string); tokenUse != "id" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token must be an id token"})
			return
		}
		if aud, _ := claims["aud"].(string); aud != v.clientID {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token audience"})
			return
		}
		sub, _ := claims["sub"].(string)
		if sub == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token missing subject"})
			return
		}

		c.Set(ContextUserSubKey, sub)
		if email, ok := claims["email"].(string); ok {
			c.Set(ContextEmailKey, email)
		}
		c.Next()
	}
}

func UserSub(c *gin.Context) (string, bool) {
	v, ok := c.Get(ContextUserSubKey)
	if !ok {
		return "", false
	}
	sub, ok := v.(string)
	return sub, ok
}

func Email(c *gin.Context) string {
	v, ok := c.Get(ContextEmailKey)
	if !ok {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
