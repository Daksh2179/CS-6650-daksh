package main

import (
	"context"
	"log"
	"os"

	"final-mastery/handlers"
	"final-mastery/store"
	"final-mastery/worker"

	"github.com/gin-gonic/gin"
)

func main() {
	ctx := context.Background()

	// --- Store clients ---
	dynamoClient, err := store.NewDynamoClient(ctx)
	if err != nil {
		log.Fatalf("failed to init dynamo: %v", err)
	}

	redisClient, err := store.NewRedisClient()
	if err != nil {
		log.Fatalf("failed to init redis: %v", err)
	}

	s3Client, err := store.NewS3Client(ctx)
	if err != nil {
		log.Fatalf("failed to init s3: %v", err)
	}

	bucket := os.Getenv("S3_BUCKET")
	if bucket == "" {
		log.Fatal("S3_BUCKET env var required")
	}

	// --- Worker pool ---
	pool := worker.NewPool(dynamoClient, s3Client, bucket, 50)
	pool.Start()

	// --- Handlers ---
	albumHandler := handlers.NewAlbumHandler(dynamoClient)
	photoHandler := handlers.NewPhotoHandler(dynamoClient, redisClient, s3Client, bucket, pool)

	// --- Router ---
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/health", handlers.Health)

	r.PUT("/albums/:album_id", albumHandler.PutAlbum)
	r.GET("/albums/:album_id", albumHandler.GetAlbum)
	r.GET("/albums", albumHandler.ListAlbums)

	r.POST("/albums/:album_id/photos", photoHandler.UploadPhoto)
	r.GET("/albums/:album_id/photos/:photo_id", photoHandler.GetPhoto)
	r.DELETE("/albums/:album_id/photos/:photo_id", photoHandler.DeletePhoto)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("starting server on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}