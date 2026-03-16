package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
)

type MapperResponse struct {
	OutputURL string `json:"output_url"`
	Message   string `json:"message"`
	WordCount int    `json:"word_count"`
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
	router.GET("/map", mapHandler)
	router.Run(":8080")
}

func mapHandler(c *gin.Context) {
	s3URL := c.Query("s3_url")
	if s3URL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "s3_url parameter required"})
		return
	}

	// Parse S3 URL
	parts := strings.TrimPrefix(s3URL, "s3://")
	urlParts := strings.SplitN(parts, "/", 2)
	if len(urlParts) != 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid S3 URL format"})
		return
	}
	bucket := urlParts[0]
	key := urlParts[1]

	// Download chunk from S3
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read chunk"})
		return
	}

	// Count words
	wordCounts := countWords(string(content))

	// Convert to JSON
	jsonData, err := json.Marshal(wordCounts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create JSON"})
		return
	}

	// Generate unique output filename from input chunk name
	chunkName := strings.TrimSuffix(strings.Split(key, "/")[len(strings.Split(key, "/"))-1], ".txt")
	outputKey := fmt.Sprintf("mapped/%s_mapped.json", chunkName)

	// Upload to S3
	_, err = s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(outputKey),
		Body:        bytes.NewReader(jsonData),
		ContentType: aws.String("application/json"),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload results"})
		return
	}

	outputURL := fmt.Sprintf("s3://%s/%s", bucket, outputKey)

	c.JSON(http.StatusOK, MapperResponse{
		OutputURL: outputURL,
		Message:   "Mapping complete",
		WordCount: len(wordCounts),
	})
}

func countWords(text string) map[string]int {
	// Convert to lowercase and split by non-word characters
	text = strings.ToLower(text)
	reg := regexp.MustCompile(`[a-z]+`)
	words := reg.FindAllString(text, -1)

	counts := make(map[string]int)
	for _, word := range words {
		if len(word) > 0 {
			counts[word]++
		}
	}
	return counts
}