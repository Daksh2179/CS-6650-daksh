package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
)

// Cart represents a shopping cart
type Cart struct {
	ID         string     `json:"id"`
	CustomerID string     `json:"customer_id"`
	Status     string     `json:"status"`
	CreatedAt  time.Time  `json:"created_at"`
	Items      []CartItem `json:"items"`
}

// CartItem represents a single item in a cart
type CartItem struct {
	ID        string  `json:"id"`
	CartID    string  `json:"cart_id"`
	ProductID string  `json:"product_id"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
}

// InitMySQL connects to RDS and creates schema if not exists
func InitMySQL() (*sql.DB, error) {
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	pass := os.Getenv("DB_PASSWORD")
	name := os.Getenv("DB_NAME")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", user, pass, host, port, name)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	// Connection pool settings for 100 concurrent sessions
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Retry connection up to 10 times (RDS may take a moment)
	for i := 0; i < 10; i++ {
		if err = db.Ping(); err == nil {
			break
		}
		log.Printf("MySQL not ready, retrying in 5s... (%d/10)", i+1)
		time.Sleep(5 * time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("could not connect to MySQL after retries: %w", err)
	}

	if err = createSchema(db); err != nil {
		return nil, err
	}

	log.Println("MySQL connected and schema ready")
	return db, nil
}

// createSchema creates tables if they don't exist
// Design decisions:
//   - Two tables: carts (header) + cart_items (line items)
//   - cart_items has FK to carts for referential integrity
//   - Index on cart_items.cart_id for fast JOIN on GET
//   - Index on carts.customer_id for customer history queries
func createSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS carts (
			id          VARCHAR(36)  PRIMARY KEY,
			customer_id VARCHAR(36)  NOT NULL,
			status      VARCHAR(20)  NOT NULL DEFAULT 'active',
			created_at  DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_customer (customer_id)
		) ENGINE=InnoDB
	`)
	if err != nil {
		return fmt.Errorf("create carts table: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS cart_items (
			id         VARCHAR(36)    PRIMARY KEY,
			cart_id    VARCHAR(36)    NOT NULL,
			product_id VARCHAR(36)    NOT NULL,
			quantity   INT            NOT NULL DEFAULT 1,
			price      DECIMAL(10,2)  NOT NULL DEFAULT 0.00,
			FOREIGN KEY (cart_id) REFERENCES carts(id) ON DELETE CASCADE,
			INDEX idx_cart (cart_id)
		) ENGINE=InnoDB
	`)
	if err != nil {
		return fmt.Errorf("create cart_items table: %w", err)
	}

	return nil
}

// POST /shopping-carts
// Request: { "customer_id": "123" }
// Response: { "id": "uuid", "customer_id": "123", "status": "active", "items": [] }
func CreateCartMySQL(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			CustomerID string `json:"customer_id" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "customer_id is required"})
			return
		}

		cartID := uuid.New().String()
		_, err := db.ExecContext(c.Request.Context(),
			"INSERT INTO carts (id, customer_id, status) VALUES (?, ?, 'active')",
			cartID, req.CustomerID,
		)
		if err != nil {
			log.Printf("CreateCart error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create cart"})
			return
		}

		c.JSON(http.StatusCreated, Cart{
			ID:         cartID,
			CustomerID: req.CustomerID,
			Status:     "active",
			CreatedAt:  time.Now(),
			Items:      []CartItem{},
		})
	}
}

// GET /shopping-carts/:id
// Response: full cart with all items
func GetCartMySQL(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		cartID := c.Param("id")

		// Get cart header
		var cart Cart
		err := db.QueryRowContext(c.Request.Context(),
			"SELECT id, customer_id, status, created_at FROM carts WHERE id = ?",
			cartID,
		).Scan(&cart.ID, &cart.CustomerID, &cart.Status, &cart.CreatedAt)

		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "cart not found"})
			return
		}
		if err != nil {
			log.Printf("GetCart error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve cart"})
			return
		}

		// Get cart items via JOIN
		rows, err := db.QueryContext(c.Request.Context(),
			"SELECT id, cart_id, product_id, quantity, price FROM cart_items WHERE cart_id = ?",
			cartID,
		)
		if err != nil {
			log.Printf("GetCart items error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve items"})
			return
		}
		defer rows.Close()

		cart.Items = []CartItem{}
		for rows.Next() {
			var item CartItem
			if err := rows.Scan(&item.ID, &item.CartID, &item.ProductID, &item.Quantity, &item.Price); err != nil {
				log.Printf("GetCart scan error: %v", err)
				continue
			}
			cart.Items = append(cart.Items, item)
		}

		c.JSON(http.StatusOK, cart)
	}
}

// POST /shopping-carts/:id/items
// Request: { "product_id": "abc", "quantity": 2, "price": 9.99 }
// Response: the created/updated item
func AddItemMySQL(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		cartID := c.Param("id")

		var req struct {
			ProductID string  `json:"product_id" binding:"required"`
			Quantity  int     `json:"quantity" binding:"required,min=1"`
			Price     float64 `json:"price" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Verify cart exists
		var exists int
		err := db.QueryRowContext(c.Request.Context(),
			"SELECT COUNT(*) FROM carts WHERE id = ?", cartID,
		).Scan(&exists)
		if err != nil || exists == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "cart not found"})
			return
		}

		// Use transaction for safe insert/update
		tx, err := db.BeginTx(c.Request.Context(), nil)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "transaction failed"})
			return
		}
		defer tx.Rollback()

		// Check if item already exists for this product in this cart
		var itemID string
		err = tx.QueryRowContext(c.Request.Context(),
			"SELECT id FROM cart_items WHERE cart_id = ? AND product_id = ?",
			cartID, req.ProductID,
		).Scan(&itemID)

		if err == sql.ErrNoRows {
			// New item
			itemID = uuid.New().String()
			_, err = tx.ExecContext(c.Request.Context(),
				"INSERT INTO cart_items (id, cart_id, product_id, quantity, price) VALUES (?, ?, ?, ?, ?)",
				itemID, cartID, req.ProductID, req.Quantity, req.Price,
			)
		} else if err == nil {
			// Update existing item quantity
			_, err = tx.ExecContext(c.Request.Context(),
				"UPDATE cart_items SET quantity = ?, price = ? WHERE id = ?",
				req.Quantity, req.Price, itemID,
			)
		}

		if err != nil {
			log.Printf("AddItem error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add item"})
			return
		}

		if err = tx.Commit(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "commit failed"})
			return
		}

		c.JSON(http.StatusCreated, CartItem{
			ID:        itemID,
			CartID:    cartID,
			ProductID: req.ProductID,
			Quantity:  req.Quantity,
			Price:     req.Price,
		})
	}
}