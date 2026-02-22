package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Product struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Brand       string `json:"brand"`
}

type SearchResponse struct {
	Products   []Product `json:"products"`
	TotalFound int       `json:"total_found"`
	SearchTime string    `json:"search_time"`
}

var productStore sync.Map
var productCount = 100000

var brands = []string{"Alpha", "Beta", "Gamma", "Delta", "Epsilon", "Zeta", "Eta", "Theta"}
var categories = []string{"Electronics", "Books", "Home", "Sports", "Clothing", "Toys", "Food", "Garden"}
var descriptions = []string{
	"High quality product for everyday use",
	"Premium grade item with excellent durability",
	"Budget-friendly option with great value",
	"Professional grade equipment",
	"Compact and portable design",
}

func generateProducts() {
	log.Println("Generating 100,000 products...")
	for i := 1; i <= productCount; i++ {
		brand := brands[i%len(brands)]
		category := categories[i%len(categories)]
		desc := descriptions[i%len(descriptions)]
		p := Product{
			ID:          i,
			Name:        fmt.Sprintf("Product %s %d", brand, i),
			Category:    category,
			Description: desc,
			Brand:       brand,
		}
		productStore.Store(i, p)
	}
	log.Println("Done generating products.")
}

func searchProducts(query string) ([]Product, int) {
	query = strings.ToLower(query)
	var results []Product
	totalFound := 0
	checked := 0

	// Simulate fixed-cost computation (burns CPU like an AI model would)
	x := 0.0
	for i := 0; i < 50000000; i++ {
		x += float64(i) * 0.0001
	}
	_ = x

	// Check exactly 100 products then stop
	productStore.Range(func(key, value interface{}) bool {
		if checked >= 100 {
			return false
		}
		checked++

		p := value.(Product)
		if strings.Contains(strings.ToLower(p.Name), query) ||
			strings.Contains(strings.ToLower(p.Category), query) {
			totalFound++
			if len(results) < 20 {
				results = append(results, p)
			}
		}
		return true
	})

	return results, totalFound
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "missing query parameter q", http.StatusBadRequest)
		return
	}

	products, total := searchProducts(query)
	elapsed := time.Since(start)

	resp := SearchResponse{
		Products:   products,
		TotalFound: total,
		SearchTime: elapsed.String(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func main() {
	generateProducts()

	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/products/search", searchHandler)

	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
