package reservations

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"neurosciolar/backend/internal/notifications"
	"neurosciolar/backend/internal/sharedauth"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	db        *sql.DB
	publisher *notifications.Publisher
}

type createRequest struct {
	HallID    string `json:"hallId"`
	Date      string `json:"date"`
	StartTime string `json:"start"`
	EndTime   string `json:"end"`
	Attendees int    `json:"attendees"`
	Purpose   string `json:"purpose"`
}

type response struct {
	ID        int64  `json:"id"`
	HallID    string `json:"hallId"`
	Date      string `json:"date"`
	StartTime string `json:"start"`
	EndTime   string `json:"end"`
	Attendees int    `json:"attendees"`
	Purpose   string `json:"purpose"`
}

func NewHandler(db *sql.DB, publisher *notifications.Publisher) *Handler {
	return &Handler{db: db, publisher: publisher}
}

func (h *Handler) Create(c *gin.Context) {
	userSub, ok := sharedauth.UserSub(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	req.HallID = strings.TrimSpace(req.HallID)
	req.Purpose = strings.TrimSpace(req.Purpose)

	reservationDate, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date format"})
		return
	}
	start, err := time.Parse("15:04", req.StartTime)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid start time format"})
		return
	}
	end, err := time.Parse("15:04", req.EndTime)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid end time format"})
		return
	}
	if req.HallID == "" || req.Purpose == "" || req.Attendees < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "hallId, purpose and attendees are required"})
		return
	}
	if !start.Before(end) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "end time must be after start time"})
		return
	}

	const conflictCheck = `
		SELECT EXISTS (
			SELECT 1 FROM reservations
			WHERE hall_id = $1
			  AND reservation_date = $2
			  AND start_time < $4
			  AND end_time > $3
		)`
	var hasConflict bool
	if err := h.db.QueryRow(conflictCheck, req.HallID, reservationDate, req.StartTime, req.EndTime).Scan(&hasConflict); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to validate reservation"})
		return
	}
	if hasConflict {
		c.JSON(http.StatusConflict, gin.H{"error": "selected hall is already reserved for this time slot"})
		return
	}

	const insert = `
		INSERT INTO reservations (user_sub, hall_id, reservation_date, start_time, end_time, attendees, purpose)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`
	var reservationID int64
	if err := h.db.QueryRow(insert, userSub, req.HallID, reservationDate, req.StartTime, req.EndTime, req.Attendees, req.Purpose).Scan(&reservationID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create reservation"})
		return
	}

	resp := response{
		ID:        reservationID,
		HallID:    req.HallID,
		Date:      reservationDate.Format("2006-01-02"),
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
		Attendees: req.Attendees,
		Purpose:   req.Purpose,
	}

	if h.publisher != nil {
		payload, _ := json.Marshal(resp)
		go func(sub, body string) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = h.publisher.Publish(ctx, notifications.Event{
				EventType: "ReservationCreated",
				UserSub:   sub,
				Payload:   body,
			})
		}(userSub, string(payload))
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":     "reservation created successfully",
		"reservation": resp,
	})
}

func (h *Handler) ListMine(c *gin.Context) {
	userSub, ok := sharedauth.UserSub(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	const query = `
		SELECT id, hall_id, reservation_date, start_time::text, end_time::text, attendees, purpose
		FROM reservations
		WHERE user_sub = $1
		ORDER BY reservation_date, start_time`
	rows, err := h.db.Query(query, userSub)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load reservations"})
		return
	}
	defer rows.Close()

	out := make([]response, 0)
	for rows.Next() {
		var item response
		var reservationDate time.Time
		if err := rows.Scan(&item.ID, &item.HallID, &reservationDate, &item.StartTime, &item.EndTime, &item.Attendees, &item.Purpose); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse reservations"})
			return
		}
		item.Date = reservationDate.Format("2006-01-02")
		item.StartTime = strings.TrimSuffix(item.StartTime, ":00")
		item.EndTime = strings.TrimSuffix(item.EndTime, ":00")
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load reservations"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"reservations": out})
}

func (h *Handler) GetOne(c *gin.Context) {
	userSub, ok := sharedauth.UserSub(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	idRaw := strings.TrimSpace(c.Param("id"))
	id, err := strconv.ParseInt(idRaw, 10, 64)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid reservation id"})
		return
	}

	const query = `
		SELECT id, hall_id, reservation_date, start_time::text, end_time::text, attendees, purpose
		FROM reservations
		WHERE id = $1 AND user_sub = $2`
	var item response
	var reservationDate time.Time
	if err := h.db.QueryRow(query, id, userSub).Scan(&item.ID, &item.HallID, &reservationDate, &item.StartTime, &item.EndTime, &item.Attendees, &item.Purpose); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "reservation not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load reservation"})
		return
	}
	item.Date = reservationDate.Format("2006-01-02")
	item.StartTime = strings.TrimSuffix(item.StartTime, ":00")
	item.EndTime = strings.TrimSuffix(item.EndTime, ":00")
	c.JSON(http.StatusOK, gin.H{"reservation": item})
}
