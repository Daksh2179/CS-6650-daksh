package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// DynaCart is the DynamoDB document structure
// Design: single table, cartId as partition key
// Items are embedded as a list attribute inside the cart document
// This avoids joins and maps naturally to NoSQL access patterns
type DynaCart struct {
	CartID     string      `dynamodbav:"cartId"`
	CustomerID string      `dynamodbav:"customerId"`
	Status     string      `dynamodbav:"status"`
	CreatedAt  string      `dynamodbav:"createdAt"`
	Items      []DynaItem  `dynamodbav:"items"`
}

type DynaItem struct {
	ID        string  `dynamodbav:"id"`
	ProductID string  `dynamodbav:"productId"`
	Quantity  int     `dynamodbav:"quantity"`
	Price     float64 `dynamodbav:"price"`
}

var dynamoTable string

// InitDynamo creates DynamoDB client using ECS task role credentials
func InitDynamo() (*dynamodb.Client, error) {
	dynamoTable = os.Getenv("DYNAMO_TABLE")
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = "us-west-2"
	}

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, err
	}

	client := dynamodb.NewFromConfig(cfg)
	log.Printf("DynamoDB client initialized, table: %s", dynamoTable)
	return client, nil
}

// POST /dynamo/shopping-carts
func CreateCartDynamo(client *dynamodb.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			CustomerID string `json:"customer_id" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "customer_id is required"})
			return
		}

		cart := DynaCart{
			CartID:     uuid.New().String(),
			CustomerID: req.CustomerID,
			Status:     "active",
			CreatedAt:  time.Now().UTC().Format(time.RFC3339),
			Items:      []DynaItem{},
		}

		item, err := attributevalue.MarshalMap(cart)
		if err != nil {
			log.Printf("CreateCartDynamo marshal error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "marshal failed"})
			return
		}

		_, err = client.PutItem(c.Request.Context(), &dynamodb.PutItemInput{
			TableName: aws.String(dynamoTable),
			Item:      item,
		})
		if err != nil {
			log.Printf("CreateCartDynamo PutItem error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create cart"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"id":          cart.CartID,
			"customer_id": cart.CustomerID,
			"status":      cart.Status,
			"created_at":  cart.CreatedAt,
			"items":       cart.Items,
		})
	}
}

// GET /dynamo/shopping-carts/:id
func GetCartDynamo(client *dynamodb.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		cartID := c.Param("id")

		result, err := client.GetItem(c.Request.Context(), &dynamodb.GetItemInput{
			TableName: aws.String(dynamoTable),
			Key: map[string]types.AttributeValue{
				"cartId": &types.AttributeValueMemberS{Value: cartID},
			},
		})
		if err != nil {
			log.Printf("GetCartDynamo GetItem error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve cart"})
			return
		}

		if result.Item == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "cart not found"})
			return
		}

		var cart DynaCart
		if err := attributevalue.UnmarshalMap(result.Item, &cart); err != nil {
			log.Printf("GetCartDynamo unmarshal error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unmarshal failed"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"id":          cart.CartID,
			"customer_id": cart.CustomerID,
			"status":      cart.Status,
			"created_at":  cart.CreatedAt,
			"items":       cart.Items,
		})
	}
}

// POST /dynamo/shopping-carts/:id/items
func AddItemDynamo(client *dynamodb.Client) gin.HandlerFunc {
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

		newItem := DynaItem{
			ID:        uuid.New().String(),
			ProductID: req.ProductID,
			Quantity:  req.Quantity,
			Price:     req.Price,
		}

		itemAV, err := attributevalue.MarshalMap(newItem)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "marshal failed"})
			return
		}

		// Use UpdateItem to append item to the items list atomically
		_, err = client.UpdateItem(c.Request.Context(), &dynamodb.UpdateItemInput{
			TableName: aws.String(dynamoTable),
			Key: map[string]types.AttributeValue{
				"cartId": &types.AttributeValueMemberS{Value: cartID},
			},
			UpdateExpression:    aws.String("SET #items = list_append(if_not_exists(#items, :empty), :newItem)"),
			ConditionExpression: aws.String("attribute_exists(cartId)"),
			ExpressionAttributeNames: map[string]string{
				"#items": "items",
			},
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":newItem": &types.AttributeValueMemberL{
					Value: []types.AttributeValue{
						&types.AttributeValueMemberM{Value: itemAV},
					},
				},
				":empty": &types.AttributeValueMemberL{Value: []types.AttributeValue{}},
			},
		})
		if err != nil {
			log.Printf("AddItemDynamo UpdateItem error: %v", err)
			c.JSON(http.StatusNotFound, gin.H{"error": "cart not found or update failed"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"id":         newItem.ID,
			"cart_id":    cartID,
			"product_id": newItem.ProductID,
			"quantity":   newItem.Quantity,
			"price":      newItem.Price,
		})
	}
}