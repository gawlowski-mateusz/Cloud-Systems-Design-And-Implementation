package storage

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Store struct {
	client *s3.Client
	bucket string
}

func NewS3Store(region, bucket string) (*S3Store, error) {
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load aws config: %w", err)
	}

	return &S3Store{
		client: s3.NewFromConfig(cfg),
		bucket: bucket,
	}, nil
}

func (s *S3Store) Upload(ctx context.Context, key string, body io.Reader, contentType string) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("failed to upload file to S3: %w", err)
	}

	return nil
}

func (s *S3Store) Download(ctx context.Context, key string) (io.ReadCloser, string, error) {
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to read file from S3: %w", err)
	}

	contentType := "application/octet-stream"
	if result.ContentType != nil && *result.ContentType != "" {
		contentType = *result.ContentType
	}

	return result.Body, contentType, nil
}
