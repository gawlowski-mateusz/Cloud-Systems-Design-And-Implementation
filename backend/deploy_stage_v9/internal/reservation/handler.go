package reservation

import (
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	db *sql.DB
}

type createReservationRequest struct {
	HallID    string `json:"hallId"`
	Date      string `json:"date"`
	StartTime string `json:"start"`
	EndTime   string `json:"end"`
	Attendees int    `json:"attendees"`
	Purpose   string `json:"purpose"`
}

type reservationResponse struct {
	ID        int64  `json:"id"`
	HallID    string `json:"hallId"`
	Date      string `json:"date"`
	StartTime string `json:"start"`
	EndTime   string `json:"end"`
	Attendees int    `json:"attendees"`
	Purpose   string `json:"purpose"`
}

func NewHandler(db *sql.DB) *Handler {
	return &Handler{db: db}
}

func (h *Handler) Create(c *gin.Context) {
	userID, ok := c.Get("userID")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	uid, valid := userID.(int64)
	if !valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req createReservationRequest
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

	const insertReservation = `
		INSERT INTO reservations (user_id, hall_id, reservation_date, start_time, end_time, attendees, purpose)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`

	var reservationID int64
	if err := h.db.QueryRow(
		insertReservation,
		uid,
		req.HallID,
		reservationDate,
		req.StartTime,
		req.EndTime,
		req.Attendees,
		req.Purpose,
	).Scan(&reservationID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create reservation"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "reservation created successfully",
		"reservation": reservationResponse{
			ID:        reservationID,
			HallID:    req.HallID,
			Date:      reservationDate.Format("2006-01-02"),
			StartTime: req.StartTime,
			EndTime:   req.EndTime,
			Attendees: req.Attendees,
			Purpose:   req.Purpose,
		},
	})
}

func (h *Handler) ListMine(c *gin.Context) {
	userID, ok := c.Get("userID")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	uid, valid := userID.(int64)
	if !valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	const query = `
		SELECT id, hall_id, reservation_date, start_time::text, end_time::text, attendees, purpose
		FROM reservations
		WHERE user_id = $1
		ORDER BY reservation_date, start_time`

	rows, err := h.db.Query(query, uid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load reservations"})
		return
	}
	defer rows.Close()

	reservations := make([]reservationResponse, 0)
	for rows.Next() {
		var item reservationResponse
		var reservationDate time.Time
		if err := rows.Scan(&item.ID, &item.HallID, &reservationDate, &item.StartTime, &item.EndTime, &item.Attendees, &item.Purpose); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse reservations"})
			return
		}
		item.Date = reservationDate.Format("2006-01-02")
		item.StartTime = strings.TrimSuffix(item.StartTime, ":00")
		item.EndTime = strings.TrimSuffix(item.EndTime, ":00")
		reservations = append(reservations, item)
	}

	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load reservations"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"reservations": reservations})
}
