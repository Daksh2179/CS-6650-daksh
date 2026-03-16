package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/google/uuid"
)

// ----- Models -----

type Item struct {
	ProductID string  `json:"product_id"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
}

type Order struct {
	OrderID    string    `json:"order_id"`
	CustomerID int       `json:"customer_id"`
	Status     string    `json:"status"`
	Items      []Item    `json:"items"`
	CreatedAt  time.Time `json:"created_at"`
}

// ----- Payment simulation -----
// We use a buffered channel to truly block throughput (not just sleep)
// This limits concurrent payments to 1, simulating a slow payment processor
var paymentSlots = make(chan struct{}, 1)

func simulatePayment() {
	paymentSlots <- struct{}{}        // acquire slot
	time.Sleep(3 * time.Second)       // simulate 3s processing
	<-paymentSlots                    // release slot
}

// ----- Handlers -----

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func syncOrderHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var order Order
	if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	order.OrderID = uuid.New().String()
	order.Status = "processing"
	order.CreatedAt = time.Now()

	// This blocks -- customer waits here
	simulatePayment()

	order.Status = "completed"
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(order)
}

func asyncOrderHandler(snsClient *sns.Client, topicArn string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var order Order
		if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		order.OrderID = uuid.New().String()
		order.Status = "pending"
		order.CreatedAt = time.Now()

		msgBytes, _ := json.Marshal(order)

		_, err := snsClient.Publish(context.TODO(), &sns.PublishInput{
			TopicArn: aws.String(topicArn),
			Message:  aws.String(string(msgBytes)),
		})
		if err != nil {
			log.Printf("failed to publish to SNS: %v", err)
			http.Error(w, "failed to queue order", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted) // 202
		json.NewEncoder(w).Encode(map[string]string{
			"order_id": order.OrderID,
			"status":   "accepted",
			"message":  "order queued for processing",
		})
	}
}

// ----- SQS Worker -----

func startWorkers(sqsClient *sqs.Client, queueUrl string, numWorkers int) {
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			log.Printf("worker %d started", workerID)
			for {
				result, err := sqsClient.ReceiveMessage(context.TODO(), &sqs.ReceiveMessageInput{
					QueueUrl:            aws.String(queueUrl),
					MaxNumberOfMessages: 10,
					WaitTimeSeconds:     20, // long polling
				})
				if err != nil {
					log.Printf("worker %d: error receiving messages: %v", workerID, err)
					time.Sleep(2 * time.Second)
					continue
				}

				for _, msg := range result.Messages {
					go func(m sqstypes.Message) {
						var order Order
						// SNS wraps the message, extract the actual body
						var snsWrapper struct {
							Message string `json:"Message"`
						}
						if err := json.Unmarshal([]byte(*m.Body), &snsWrapper); err == nil && snsWrapper.Message != "" {
							json.Unmarshal([]byte(snsWrapper.Message), &order)
						} else {
							json.Unmarshal([]byte(*m.Body), &order)
						}

						log.Printf("worker %d: processing order %s", workerID, order.OrderID)
						simulatePayment()
						log.Printf("worker %d: completed order %s", workerID, order.OrderID)

						// Delete message from queue
						sqsClient.DeleteMessage(context.TODO(), &sqs.DeleteMessageInput{
							QueueUrl:      aws.String(queueUrl),
							ReceiptHandle: m.ReceiptHandle,
						})
					}(msg)
				}
			}
		}(i)
	}
	wg.Wait()
}

// ----- Main -----

func main() {
	topicArn := os.Getenv("SNS_TOPIC_ARN")
	queueUrl := os.Getenv("SQS_QUEUE_URL")
	mode := os.Getenv("APP_MODE") // "receiver" or "processor"
	numWorkersStr := os.Getenv("NUM_WORKERS")

	numWorkers := 1
	if numWorkersStr != "" {
		fmt.Sscanf(numWorkersStr, "%d", &numWorkers)
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-west-2"))
	if err != nil {
		log.Fatalf("failed to load AWS config: %v", err)
	}

	snsClient := sns.NewFromConfig(cfg)
	sqsClient := sqs.NewFromConfig(cfg)

	if mode == "processor" {
		log.Printf("starting processor with %d workers", numWorkers)
		startWorkers(sqsClient, queueUrl, numWorkers)
		return
	}

	// Default: receiver mode (HTTP server)
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/orders/sync", syncOrderHandler)
	http.HandleFunc("/orders/async", asyncOrderHandler(snsClient, topicArn))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("SNS_TOPIC_ARN=%s", topicArn)
	log.Printf("SQS_QUEUE_URL=%s", queueUrl)
	log.Printf("starting order receiver on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}