package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
)

type SplitterResponse struct {
	ChunkURLs []string `json:"chunk_urls"`
	Message   string   `json:"message"`
}

var s3Client *s3.Client

func main() {
	// Initialize AWS SDK
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-west-2"))
	if err != nil {
		panic(err)
	}
	s3Client = s3.NewFromConfig(cfg)

	router := gin.Default()
	router.GET("/split", splitHandler)
	router.Run(":8080")
}

func splitHandler(c *gin.Context) {
	s3URL := c.Query("s3_url")
	if s3URL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "s3_url parameter required"})
		return
	}

	// Parse S3 URL: s3://bucket-name/input/hamlet.txt
	parts := strings.TrimPrefix(s3URL, "s3://")
	urlParts := strings.SplitN(parts, "/", 2)
	if len(urlParts) != 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid S3 URL format"})
		return
	}
	bucket := urlParts[0]
	key := urlParts[1]

	// Download file from S3
	result, err := s3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to download: %v", err)})
		return
	}
	defer result.Body.Close()

	content, err := io.ReadAll(result.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read file"})
		return
	}

	// Split into 3 chunks
	text := string(content)
	chunkSize := len(text) / 3
	chunks := []string{
		text[:chunkSize],
		text[chunkSize : 2*chunkSize],
		text[2*chunkSize:],
	}

	// Upload chunks to S3
	var chunkURLs []string
	for i, chunk := range chunks {
		chunkKey := fmt.Sprintf("chunks/chunk_%d.txt", i+1)
		_, err := s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(chunkKey),
			Body:   bytes.NewReader([]byte(chunk)),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to upload chunk %d", i+1)})
			return
		}
		chunkURLs = append(chunkURLs, fmt.Sprintf("s3://%s/%s", bucket, chunkKey))
	}

	c.JSON(http.StatusOK, SplitterResponse{
		ChunkURLs: chunkURLs,
		Message:   "File split successfully",
	})
}