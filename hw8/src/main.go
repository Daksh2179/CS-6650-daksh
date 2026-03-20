package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
	// Initialize MySQL
	db, err := InitMySQL()
	if err != nil {
		log.Fatalf("Failed to connect to MySQL: %v", err)
	}
	defer db.Close()

	// Initialize DynamoDB
	dynaClient, err := InitDynamo()
	if err != nil {
		log.Fatalf("Failed to initialize DynamoDB: %v", err)
	}

	r := gin.Default()

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// MySQL routes - POST /shopping-carts
	// Creates a new shopping cart with customer info, returns cart ID
	r.POST("/shopping-carts", CreateCartMySQL(db))

	// GET /shopping-carts/:id
	// Retrieves full cart with all items using JOIN
	r.GET("/shopping-carts/:id", GetCartMySQL(db))

	// POST /shopping-carts/:id/items
	// Adds or updates an item in an existing cart
	r.POST("/shopping-carts/:id/items", AddItemMySQL(db))

	// DynamoDB routes (same API spec, different backend)
	r.POST("/dynamo/shopping-carts", CreateCartDynamo(dynaClient))
	r.GET("/dynamo/shopping-carts/:id", GetCartDynamo(dynaClient))
	r.POST("/dynamo/shopping-carts/:id/items", AddItemDynamo(dynaClient))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	r.Run(":" + port)
}