package store

import (
	"bytes"
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Client struct {
	client *s3.Client
	region string
}

func NewS3Client(ctx context.Context) (*S3Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	return &S3Client{
		client: s3.NewFromConfig(cfg),
		region: cfg.Region,
	}, nil
}

func (s *S3Client) Upload(ctx context.Context, bucket, albumID, photoID string, data []byte, contentType string) (string, error) {
	key := fmt.Sprintf("albums/%s/photos/%s", albumID, photoID)

	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("s3 put: %w", err)
	}

	url := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", bucket, s.region, key)
	return url, nil
}

func (s *S3Client) Delete(ctx context.Context, bucket, albumID, photoID string) error {
	key := fmt.Sprintf("albums/%s/photos/%s", albumID, photoID)
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return err
}