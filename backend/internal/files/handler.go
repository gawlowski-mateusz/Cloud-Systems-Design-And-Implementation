package files

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"neurosciolar/backend/internal/dynamostore"
	"neurosciolar/backend/internal/events"
	"neurosciolar/backend/internal/sharedauth"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
)

const maxFileSize = 25 * 1024 * 1024

type Handler struct {
	s3        *s3.Client
	bucket    string
	store     *dynamostore.FileMetadataStore
	publisher *events.Publisher
}

func NewHandler(s3Client *s3.Client, bucket string, store *dynamostore.FileMetadataStore, publisher *events.Publisher) *Handler {
	return &Handler{s3: s3Client, bucket: bucket, store: store, publisher: publisher}
}

func (h *Handler) Upload(c *gin.Context) {
	userSub, ok := sharedauth.UserSub(c)
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
	if fileHeader.Size > maxFileSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file too large (max 25 MB)"})
		return
	}

	originalName := strings.TrimSpace(fileHeader.Filename)
	if originalName == "" {
		originalName = "uploaded-file"
	}
	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = mime.TypeByExtension(filepath.Ext(originalName))
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	opened, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read uploaded file"})
		return
	}
	defer opened.Close()

	fileID := newFileID()
	objectKey := buildObjectKey(userSub, fileID, originalName)

	if _, err := h.s3.PutObject(c.Request.Context(), &s3.PutObjectInput{
		Bucket:      aws.String(h.bucket),
		Key:         aws.String(objectKey),
		Body:        opened,
		ContentType: aws.String(contentType),
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to upload file to S3"})
		return
	}

	meta := dynamostore.FileMetadata{
		UserSub:      userSub,
		FileID:       fileID,
		OriginalName: originalName,
		ContentType:  contentType,
		SizeBytes:    fileHeader.Size,
		ObjectKey:    objectKey,
	}
	if err := h.store.Put(c.Request.Context(), meta); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist file metadata"})
		return
	}

	if h.publisher != nil {
		payload, _ := json.Marshal(gin.H{
			"fileId":       meta.FileID,
			"originalName": meta.OriginalName,
			"sizeBytes":    meta.SizeBytes,
		})
		go func(sub, body string) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = h.publisher.Publish(ctx, events.Event{
				EventType: "FileUploaded",
				UserSub:   sub,
				Payload:   body,
			})
		}(userSub, string(payload))
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "file uploaded successfully",
		"file":    meta,
	})
}

func (h *Handler) ListMine(c *gin.Context) {
	userSub, ok := sharedauth.UserSub(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	items, err := h.store.ListByUser(c.Request.Context(), userSub)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load files"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"files": items})
}

func (h *Handler) Download(c *gin.Context) {
	userSub, ok := sharedauth.UserSub(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	fileID := strings.TrimSpace(c.Param("id"))
	if fileID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file id"})
		return
	}

	meta, found, err := h.store.Get(c.Request.Context(), userSub, fileID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load metadata"})
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	out, err := h.s3.GetObject(c.Request.Context(), &s3.GetObjectInput{
		Bucket: aws.String(h.bucket),
		Key:    aws.String(meta.ObjectKey),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch file"})
		return
	}
	defer out.Body.Close()

	contentType := meta.ContentType
	if contentType == "" && out.ContentType != nil {
		contentType = *out.ContentType
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", meta.OriginalName))
	c.DataFromReader(http.StatusOK, meta.SizeBytes, contentType, out.Body, nil)
}

func newFileID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), hex.EncodeToString(buf))
}

func buildObjectKey(userSub, fileID, fileName string) string {
	safeName := strings.ReplaceAll(fileName, " ", "-")
	safeName = strings.ReplaceAll(safeName, "/", "-")
	safeName = strings.ReplaceAll(safeName, "\\", "-")
	if safeName == "" {
		safeName = "file"
	}
	return fmt.Sprintf("users/%s/%s-%s", userSub, fileID, safeName)
}
