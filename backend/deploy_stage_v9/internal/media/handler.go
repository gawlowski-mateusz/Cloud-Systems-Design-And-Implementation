package media

import (
	"database/sql"
	"fmt"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"neurosciolar/backend/internal/auth"
	"neurosciolar/backend/internal/storage"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	db    *sql.DB
	store storage.FileStore
}

type mediaItem struct {
	ID          int64  `json:"id"`
	FileName    string `json:"fileName"`
	ContentType string `json:"contentType"`
	SizeBytes   int64  `json:"sizeBytes"`
	CreatedAt   string `json:"createdAt"`
}

func NewHandler(db *sql.DB, store storage.FileStore) *Handler {
	return &Handler{db: db, store: store}
}

func (h *Handler) Upload(c *gin.Context) {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required in multipart form field 'file'"})
		return
	}

	if fileHeader.Size <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is empty"})
		return
	}

	if fileHeader.Size > 25*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file too large (max 25 MB)"})
		return
	}

	originalName := strings.TrimSpace(fileHeader.Filename)
	if originalName == "" {
		originalName = "uploaded-file"
	}

	opened, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read uploaded file"})
		return
	}
	defer opened.Close()

	key := buildObjectKey(userID, originalName)
	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(originalName))
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	if err := h.store.Upload(c.Request.Context(), key, opened, contentType); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store file"})
		return
	}

	const query = `
		INSERT INTO media_files (user_id, original_name, content_type, size_bytes, object_key)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`

	var mediaID int64
	var createdAt time.Time
	if err := h.db.QueryRow(query, userID, originalName, contentType, fileHeader.Size, key).Scan(&mediaID, &createdAt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist file metadata"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "file uploaded successfully",
		"file": mediaItem{
			ID:          mediaID,
			FileName:    originalName,
			ContentType: contentType,
			SizeBytes:   fileHeader.Size,
			CreatedAt:   createdAt.UTC().Format(time.RFC3339),
		},
	})
}

func (h *Handler) ListMine(c *gin.Context) {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	const query = `
		SELECT id, original_name, content_type, size_bytes, created_at
		FROM media_files
		WHERE user_id = $1
		ORDER BY created_at DESC`

	rows, err := h.db.Query(query, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load files"})
		return
	}
	defer rows.Close()

	items := make([]mediaItem, 0)
	for rows.Next() {
		var item mediaItem
		var createdAt time.Time
		if err := rows.Scan(&item.ID, &item.FileName, &item.ContentType, &item.SizeBytes, &createdAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse files"})
			return
		}
		item.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load files"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"files": items})
}

func (h *Handler) Download(c *gin.Context) {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	idRaw := strings.TrimSpace(c.Param("id"))
	mediaID, err := strconv.ParseInt(idRaw, 10, 64)
	if err != nil || mediaID < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid media id"})
		return
	}

	const query = `
		SELECT original_name, content_type, object_key
		FROM media_files
		WHERE id = $1 AND user_id = $2`

	var originalName string
	var contentType string
	var objectKey string
	if err := h.db.QueryRow(query, mediaID, userID).Scan(&originalName, &contentType, &objectKey); err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load file metadata"})
		return
	}

	reader, detectedType, err := h.store.Download(c.Request.Context(), objectKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch file content"})
		return
	}
	defer reader.Close()

	if contentType == "" {
		contentType = detectedType
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", originalName))
	c.DataFromReader(http.StatusOK, -1, contentType, reader, nil)
}

func buildObjectKey(userID int64, fileName string) string {
	safeName := strings.ReplaceAll(fileName, " ", "-")
	safeName = strings.ReplaceAll(safeName, "/", "-")
	safeName = strings.ReplaceAll(safeName, "\\", "-")
	if safeName == "" {
		safeName = "file"
	}

	return fmt.Sprintf("users/%d/%d-%s", userID, time.Now().UnixNano(), safeName)
}
