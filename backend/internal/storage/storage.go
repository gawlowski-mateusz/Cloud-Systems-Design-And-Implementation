package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type FileStore interface {
	Upload(ctx context.Context, key string, body io.Reader, contentType string) error
	Download(ctx context.Context, key string) (io.ReadCloser, string, error)
}

func NewFromEnv() (FileStore, error) {
	provider := strings.ToLower(strings.TrimSpace(os.Getenv("FILE_STORAGE_PROVIDER")))
	if provider == "" {
		provider = "local"
	}

	switch provider {
	case "s3":
		bucket := strings.TrimSpace(os.Getenv("S3_BUCKET"))
		region := strings.TrimSpace(os.Getenv("AWS_REGION"))
		if bucket == "" || region == "" {
			return nil, fmt.Errorf("S3_BUCKET and AWS_REGION must be set when FILE_STORAGE_PROVIDER=s3")
		}
		return NewS3Store(region, bucket)
	case "local":
		baseDir := strings.TrimSpace(os.Getenv("MEDIA_LOCAL_DIR"))
		if baseDir == "" {
			baseDir = "uploads"
		}
		if !filepath.IsAbs(baseDir) {
			baseDir = filepath.Clean(baseDir)
		}
		return NewLocalStore(baseDir)
	default:
		return nil, fmt.Errorf("unsupported FILE_STORAGE_PROVIDER: %s", provider)
	}
}
