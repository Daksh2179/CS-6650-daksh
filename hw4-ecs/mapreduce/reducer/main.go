package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
)

type ReducerResponse struct {
	OutputURL      string `json:"output_url"`
	Message        string `json:"message"`
	TotalWords     int    `json:"total_words"`
	UniqueWords    int    `json:"unique_words"`
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
	router.GET("/reduce", reduceHandler)
	router.Run(":8080")
}

func reduceHandler(c *gin.Context) {
	s3URLs := c.Query("s3_urls")
	if s3URLs == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "s3_urls parameter required (comma-separated)"})
		return
	}

	urls := strings.Split(s3URLs, ",")
	if len(urls) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No URLs provided"})
		return
	}

	// Aggregate word counts from all mappers
	finalCounts := make(map[string]int)
	var bucket string

	for _, s3URL := range urls {
		s3URL = strings.TrimSpace(s3URL)
		
		// Parse S3 URL
		parts := strings.TrimPrefix(s3URL, "s3://")
		urlParts := strings.SplitN(parts, "/", 2)
		if len(urlParts) != 2 {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid S3 URL: %s", s3URL)})
			return
		}
		bucket = urlParts[0]
		key := urlParts[1]

		// Download JSON from S3
		result, err := s3Client.GetObject(context.TODO(), &s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to download %s: %v", key, err)})
			return
		}

		content, err := io.ReadAll(result.Body)
		result.Body.Close()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read mapper result"})
			return
		}

		// Parse JSON
		var wordCounts map[string]int
		if err := json.Unmarshal(content, &wordCounts); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse JSON"})
			return
		}

		// Aggregate counts
		for word, count := range wordCounts {
			finalCounts[word] += count
		}
	}

	// Calculate total words
	totalWords := 0
	for _, count := range finalCounts {
		totalWords += count
	}

	// Convert final counts to JSON
	jsonData, err := json.MarshalIndent(finalCounts, "", "  ")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create final JSON"})
		return
	}

	// Upload final result to S3
	outputKey := "output/final_word_counts.json"
	_, err = s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(outputKey),
		Body:        bytes.NewReader(jsonData),
		ContentType: aws.String("application/json"),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload final result"})
		return
	}

	outputURL := fmt.Sprintf("s3://%s/%s", bucket, outputKey)

	c.JSON(http.StatusOK, ReducerResponse{
		OutputURL:   outputURL,
		Message:     "Reduction complete",
		TotalWords:  totalWords,
		UniqueWords: len(finalCounts),
	})
}