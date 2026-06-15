package notifications

import (
	"net/http"

	"neurosciolar/backend/internal/dynamostore"
	"neurosciolar/backend/internal/sharedauth"

	"github.com/gin-gonic/gin"
)

// Handler serves the read side of the notifications service. The write side is
// fully event-driven now: notifications are produced by the SQS Consumer, not by
// HTTP requests, so this handler only exposes the per-user history.
type Handler struct {
	store *dynamostore.NotificationStore
}

func NewHandler(store *dynamostore.NotificationStore) *Handler {
	return &Handler{store: store}
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
