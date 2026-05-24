package notifications

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"neurosciolar/backend/internal/dynamostore"
	"neurosciolar/backend/internal/sharedauth"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	store     *dynamostore.NotificationStore
	publisher *Publisher
	http      *http.Client
}

type broadcastRequest struct {
	EventType string `json:"eventType"`
	Payload   string `json:"payload"`
}

func NewHandler(store *dynamostore.NotificationStore, publisher *Publisher) *Handler {
	return &Handler{
		store:     store,
		publisher: publisher,
		http:      &http.Client{Timeout: 10 * time.Second},
	}
}

func (h *Handler) ListMine(c *gin.Context) {
	userSub, ok := sharedauth.UserSub(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	items, err := h.store.ListByUser(c.Request.Context(), userSub, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load notifications"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"notifications": items})
}

func (h *Handler) Broadcast(c *gin.Context) {
	if _, ok := sharedauth.UserSub(c); !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req broadcastRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	req.EventType = strings.TrimSpace(req.EventType)
	if req.EventType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "eventType is required"})
		return
	}
	if err := h.publisher.Publish(c.Request.Context(), Event{
		EventType: req.EventType,
		UserSub:   dynamostore.BroadcastUserSub,
		Payload:   req.Payload,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to publish event"})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"message": "event published"})
}

type snsEnvelope struct {
	Type             string `json:"Type"`
	MessageID        string `json:"MessageId"`
	Token            string `json:"Token"`
	TopicArn         string `json:"TopicArn"`
	Message          string `json:"Message"`
	SubscribeURL     string `json:"SubscribeURL"`
	Timestamp        string `json:"Timestamp"`
	SignatureVersion string `json:"SignatureVersion"`
	Signature        string `json:"Signature"`
	MessageAttrs     map[string]struct {
		Type  string `json:"Type"`
		Value string `json:"Value"`
	} `json:"MessageAttributes"`
}

func (h *Handler) Subscribe(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}
	defer c.Request.Body.Close()

	var env snsEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sns payload"})
		return
	}

	switch env.Type {
	case "SubscriptionConfirmation":
		if env.SubscribeURL == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "missing subscribe url"})
			return
		}
		resp, err := h.http.Get(env.SubscribeURL)
		if err != nil {
			log.Printf("sns subscribe confirmation failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to confirm subscription"})
			return
		}
		resp.Body.Close()
		log.Printf("sns subscription confirmed: %s", env.TopicArn)
		c.JSON(http.StatusOK, gin.H{"status": "confirmed"})
	case "Notification":
		eventType := ""
		userSub := ""
		if attr, ok := env.MessageAttrs["event_type"]; ok {
			eventType = attr.Value
		}
		if attr, ok := env.MessageAttrs["user_sub"]; ok {
			userSub = attr.Value
		}
		if userSub == "" || userSub == "broadcast" {
			userSub = dynamostore.BroadcastUserSub
		}
		err := h.store.Put(c.Request.Context(), dynamostore.Notification{
			UserSub:   userSub,
			EventType: eventType,
			Payload:   env.Message,
		})
		if err != nil {
			log.Printf("failed to store notification: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store notification"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "stored"})
	default:
		log.Printf("unhandled sns envelope type: %s", env.Type)
		c.JSON(http.StatusOK, gin.H{"status": "ignored"})
	}
}
