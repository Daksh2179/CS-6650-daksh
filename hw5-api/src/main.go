package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"sync"

	"github.com/gorilla/mux"
)

// Product schema matching api.yaml exactly
type Product struct {
	ProductID    int    `json:"product_id"`
	SKU          string `json:"sku"`
	Manufacturer string `json:"manufacturer"`
	CategoryID   int    `json:"category_id"`
	Weight       int    `json:"weight"`
	SomeOtherID  int    `json:"some_other_id"`
}

// Error schema matching api.yaml exactly
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// In-memory product storage
type ProductStore struct {
	mu       sync.RWMutex
	products map[int]Product
}

var store = &ProductStore{
	products: make(map[int]Product),
}

// GetProduct handles GET /products/{productId}
// Returns: 200 (success), 404 (not found), 500 (server error)
func GetProduct(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	productIDStr := vars["productId"]

	// Parse and validate productId
	productID, err := strconv.Atoi(productIDStr)
	if err != nil || productID < 1 {
		sendError(w, http.StatusBadRequest, "INVALID_INPUT", "Product ID must be a positive integer", "")
		return
	}

	// Look up product
	store.mu.RLock()
	product, exists := store.products[productID]
	store.mu.RUnlock()

	if !exists {
		sendError(w, http.StatusNotFound, "NOT_FOUND", "Product not found", "")
		return
	}

	// Return product
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(product)
}

// AddProductDetails handles POST /products/{productId}/details
// Returns: 204 (success), 400 (invalid input), 404 (not found), 500 (server error)
func AddProductDetails(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	productIDStr := vars["productId"]

	// Parse and validate productId from path
	productID, err := strconv.Atoi(productIDStr)
	if err != nil || productID < 1 {
		sendError(w, http.StatusBadRequest, "INVALID_INPUT", "Product ID must be a positive integer", "")
		return
	}

	// Parse request body
	var product Product
	if err := json.NewDecoder(r.Body).Decode(&product); err != nil {
		sendError(w, http.StatusBadRequest, "INVALID_INPUT", "Invalid JSON in request body", err.Error())
		return
	}

	// Validate required fields per api.yaml schema
	if product.ProductID == 0 {
		sendError(w, http.StatusBadRequest, "INVALID_INPUT", "product_id is required", "")
		return
	}
	if product.SKU == "" {
		sendError(w, http.StatusBadRequest, "INVALID_INPUT", "sku is required", "")
		return
	}
	if product.Manufacturer == "" {
		sendError(w, http.StatusBadRequest, "INVALID_INPUT", "manufacturer is required", "")
		return
	}
	if product.CategoryID == 0 {
		sendError(w, http.StatusBadRequest, "INVALID_INPUT", "category_id is required", "")
		return
	}
	if product.Weight < 0 {
		sendError(w, http.StatusBadRequest, "INVALID_INPUT", "weight must be non-negative", "")
		return
	}
	if product.SomeOtherID == 0 {
		sendError(w, http.StatusBadRequest, "INVALID_INPUT", "some_other_id is required", "")
		return
	}

	// Validate SKU length (1-100 chars)
	if len(product.SKU) > 100 {
		sendError(w, http.StatusBadRequest, "INVALID_INPUT", "sku must be at most 100 characters", "")
		return
	}

	// Validate manufacturer length (1-200 chars)
	if len(product.Manufacturer) > 200 {
		sendError(w, http.StatusBadRequest, "INVALID_INPUT", "manufacturer must be at most 200 characters", "")
		return
	}

	// Ensure product_id from body matches path parameter
	if product.ProductID != productID {
		sendError(w, http.StatusBadRequest, "INVALID_INPUT", "Product ID in body must match path parameter", "")
		return
	}

	// Save product
	store.mu.Lock()
	store.products[productID] = product
	store.mu.Unlock()

	// Return 204 No Content (success)
	w.WriteHeader(http.StatusNoContent)
}

// Helper function to send error responses
func sendError(w http.ResponseWriter, statusCode int, errorCode string, message string, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error:   errorCode,
		Message: message,
		Details: details,
	})
}

// Health check endpoint for ECS
func HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func main() {
	r := mux.NewRouter()

	// Product API routes (matching api.yaml exactly)
	r.HandleFunc("/products/{productId}", GetProduct).Methods("GET")
	r.HandleFunc("/products/{productId}/details", AddProductDetails).Methods("POST")

	// Health check for ALB/ECS
	r.HandleFunc("/health", HealthCheck).Methods("GET")

	port := "8080"
	log.Printf("Product API server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}