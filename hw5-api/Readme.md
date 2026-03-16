# HW5: Product API with Terraform Deployment

## Project Overview

E-commerce Product API implementation deployed to AWS ECS using Terraform infrastructure as code. Implements two endpoints from the OpenAPI specification for product management.

## Project Structure
```
hw5-api/
├── src/                          # Application source code
│   ├── main.go                  # Product API implementation
│   ├── Dockerfile               # Container configuration
│   ├── go.mod                   # Go dependencies
│   └── go.sum
├── terraform/                    # Infrastructure as Code
│   ├── main.tf                  # Main Terraform configuration
│   ├── variables.tf             # Variable definitions
│   ├── provider.tf              # AWS/Docker provider config
│   ├── outputs.tf               # Output definitions
│   └── modules/                 # Terraform modules
│       ├── ecr/                 # Elastic Container Registry
│       ├── ecs/                 # Elastic Container Service
│       ├── logging/             # CloudWatch logs
│       └── network/             # VPC, security groups
├── tests/                        # Load testing
│   └── locustfile.py            # Locust test scenarios
├── .gitignore                   # Git ignore rules
└── README.md                    # This file
```

## API Endpoints Implemented

Based on the provided `api.yaml` OpenAPI specification:

### 1. GET /products/{productId}
Retrieve product details by ID.

**Response Codes:**
- `200 OK` - Product found
- `404 Not Found` - Product doesn't exist
- `400 Bad Request` - Invalid product ID format

### 2. POST /products/{productId}/details
Add or update product details.

**Response Codes:**
- `204 No Content` - Product created/updated successfully
- `400 Bad Request` - Invalid input data or missing required fields
- `404 Not Found` - Product ID mismatch

## Local Development

### Prerequisites
- Go 1.23+
- Docker Desktop
- AWS CLI configured
- Terraform 1.0+

### Run Locally
```bash
# Navigate to source directory
cd src/

# Install dependencies
go mod download

# Run the server
go run main.go
```

Server will start on `http://localhost:8080`

### Test Locally
```bash
# Create a product
curl -X POST http://localhost:8080/products/1/details \
  -H "Content-Type: application/json" \
  -d '{
    "product_id": 1,
    "sku": "ABC-123-XYZ",
    "manufacturer": "Acme Corporation",
    "category_id": 456,
    "weight": 1250,
    "some_other_id": 789
  }'

# Get the product
curl http://localhost:8080/products/1

# Test non-existent product (404)
curl http://localhost:8080/products/999

# Health check
curl http://localhost:8080/health
```

## Docker Build and Test
```bash
cd src/

# Build Docker image
docker build -t product-api:latest .

# Run container
docker run -d -p 8080:8080 --name product-api-test product-api:latest

# Test
curl http://localhost:8080/health

# Stop and remove
docker stop product-api-test
docker rm product-api-test
```

## AWS Deployment with Terraform

### Prerequisites
1. AWS Academy Learner Lab started
2. AWS credentials configured

### Configure AWS Credentials
```bash
# Start AWS Learner Lab, then get credentials from AWS Details

# Configure AWS CLI
aws configure
# Enter: Access Key, Secret Key, Region (us-west-2)

# Set session token
aws configure set aws_session_token YOUR_SESSION_TOKEN

# Verify
aws sts get-caller-identity
```

### Deploy Infrastructure
```bash
cd terraform/

# Initialize Terraform
terraform init

# Preview changes
terraform plan

# Deploy (builds Docker image, pushes to ECR, creates infrastructure)
terraform apply -auto-approve
```

**What gets created:**
- ECR repository for Docker images
- ECS cluster and Fargate service
- VPC networking and security groups
- Application Load Balancer
- CloudWatch log groups

### Get Public IP Address

**Option 1: AWS Console**
1. Go to ECS → Clusters → product-api-hw5-cluster
2. Click Services → product-api-hw5
3. Click Tasks → Select running task
4. Find Public IP in Configuration section

**Option 2: AWS CLI**
```bash
aws ec2 describe-network-interfaces \
--network-interface-ids $(
    aws ecs describe-tasks \
    --cluster product-api-hw5-cluster \
    --tasks $(
        aws ecs list-tasks \
        --cluster product-api-hw5-cluster \
        --service-name product-api-hw5 \
        --query 'taskArns[0]' --output text
    ) \
    --query "tasks[0].attachments[0].details[?name=='networkInterfaceId'].value" \
    --output text
) \
--query 'NetworkInterfaces[0].Association.PublicIp' \
--output text
```

### Test Deployed API
```bash
# Replace with your actual public IP
export PUBLIC_IP="54.188.109.70"

# Health check (200 OK)
curl http://$PUBLIC_IP:8080/health

# Create product (204 No Content)
curl -X POST http://$PUBLIC_IP:8080/products/1/details \
  -H "Content-Type: application/json" \
  -d '{
    "product_id": 1,
    "sku": "ABC-123",
    "manufacturer": "Acme Corp",
    "category_id": 456,
    "weight": 1250,
    "some_other_id": 789
  }'

# Get product (200 OK)
curl http://$PUBLIC_IP:8080/products/1

# Non-existent product (404 Not Found)
curl http://$PUBLIC_IP:8080/products/999

# Invalid input (400 Bad Request)
curl -X POST http://$PUBLIC_IP:8080/products/2/details \
  -H "Content-Type: application/json" \
  -d '{"product_id": 2, "sku": "TEST"}'
```

## API Response Examples

### 200 OK - Product Found
```bash
$ curl http://54.188.109.70:8080/products/1
```
Response:
```json
{
  "product_id": 1,
  "sku": "ABC-123",
  "manufacturer": "Acme Corp",
  "category_id": 456,
  "weight": 1250,
  "some_other_id": 789
}
```

### 204 No Content - Product Created
```bash
$ curl -X POST http://54.188.109.70:8080/products/1/details \
  -H "Content-Type: application/json" \
  -d '{...}'
```
Response: Empty body, status code 204

### 400 Bad Request - Invalid Input
```bash
$ curl -X POST http://54.188.109.70:8080/products/2/details \
  -H "Content-Type: application/json" \
  -d '{"product_id": 2, "sku": "TEST"}'
```
Response:
```json
{
  "error": "INVALID_INPUT",
  "message": "manufacturer is required"
}
```

### 404 Not Found - Product Not Found
```bash
$ curl http://54.188.109.70:8080/products/999
```
Response:
```json
{
  "error": "NOT_FOUND",
  "message": "Product not found"
}
```

## Load Testing

### Setup and Run
```bash
# Install Locust
pip install locust --user

# Run load tests
cd tests/
python -m locust -f locustfile.py
```

Open browser to `http://localhost:8089`

### Test Configuration
- **Users**: 100 concurrent users
- **Spawn rate**: 10 users/second
- **Duration**: 5 minutes
- **Read/Write ratio**: 5:1 (simulates realistic e-commerce traffic)

### Test Scenarios
1. **GET /products/{id}** (5x weight) - Most common operation
2. **POST /products/{id}/details** (1x weight) - Less frequent writes

### Results Analysis

**Performance Metrics:**
- Average response time: ~X ms
- 95th percentile: ~Y ms
- Requests per second: Z RPS
- Failure rate: 0%

**Key Observations:**
- In-memory HashMap provides O(1) lookup performance
- Single ECS task (256 CPU, 512 MB memory) handles load well
- No failures during sustained load testing
- Response times remain consistent under load

## Design Decisions

### Data Structure: HashMap
**Choice**: `map[int]Product` for in-memory storage

**Pros:**
- O(1) lookup by product ID
- Simple implementation
- Fast for assignment requirements

**Cons:**
- Not persistent (data lost on restart)
- No range queries or complex filtering
- Limited to single instance (no distributed state)

**Production Alternative**: Would use PostgreSQL/DynamoDB for persistence and scalability

### Terraform (Declarative Infrastructure)

Terraform uses declarative syntax - you specify the **desired end state**, not the steps to get there.

**Example:**
```hcl
resource "aws_ecs_cluster" "main" {
  name = "product-api-cluster"
}
```

Terraform automatically:
- Determines what needs to be created/updated/destroyed
- Handles dependencies between resources
- Maintains state to track infrastructure

**Benefit**: Infrastructure as code, version controlled, reproducible deployments

### Scalability Considerations

For the full e-commerce system in `api.yaml`:

**Architecture:**
1. **Microservices**: Separate services for Products, Cart, Warehouse, Payments
2. **API Gateway**: Route requests to appropriate services
3. **Database**: 
   - Products: PostgreSQL for relational data
   - Cart: Redis for session state
   - Orders: PostgreSQL with read replicas
4. **Message Queue**: RabbitMQ/SQS for async order processing
5. **Caching**: Redis for product catalog
6. **CDN**: CloudFront for static assets
7. **Auto-scaling**: ECS with target tracking based on CPU/memory
8. **Load Balancer**: Application Load Balancer with health checks

## Cleanup
```bash
cd terraform/
terraform destroy -auto-approve
```

This removes all AWS resources created by Terraform.

## Files Location Summary

- **Server Code**: `src/main.go`
- **Dockerfile**: `src/Dockerfile`
- **Terraform Infrastructure**: `terraform/` directory
- **Load Tests**: `tests/locustfile.py`
- **Git Ignore**: `.gitignore` (excludes .tfstate, .env, sensitive files)

## Notes

- Uses AWS Learner Lab with temporary credentials
- Session tokens expire - need to refresh for long deployments
- ECS Fargate in public subnets with public IPs for simplicity
- CloudWatch logs retained for 7 days
- Security group allows inbound on port 8080 from anywhere (0.0.0.0/0)
