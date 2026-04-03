package storage

import (
	"context"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
)

type LocalStore struct {
	baseDir string
}

func NewLocalStore(baseDir string) (*LocalStore, error) {
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create local media directory: %w", err)
	}
	return &LocalStore{baseDir: baseDir}, nil
}

func (s *LocalStore) Upload(_ context.Context, key string, body io.Reader, _ string) error {
	fullPath := filepath.Join(s.baseDir, filepath.Clean(key))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return fmt.Errorf("failed to create media path: %w", err)
	}

	f, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create local media file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, body); err != nil {
		return fmt.Errorf("failed to write local media file: %w", err)
	}

	return nil
}

func (s *LocalStore) Download(_ context.Context, key string) (io.ReadCloser, string, error) {
	fullPath := filepath.Join(s.baseDir, filepath.Clean(key))
	f, err := os.Open(fullPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open local media file: %w", err)
	}

	contentType := mime.TypeByExtension(filepath.Ext(fullPath))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	return f, contentType, nil
}
