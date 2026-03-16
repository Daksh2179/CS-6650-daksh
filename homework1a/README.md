# Web Service with Go and Gin

A RESTful API built with Go and the Gin framework for managing vintage jazz album records.

## Description
This project implements a simple REST API that handles CRUD operations for album data. Built as part of CS6650 Lab 1 to learn Go programming and RESTful API design.

## API Endpoints
- `GET /albums` - Returns all albums
- `GET /albums/:id` - Returns a specific album by ID
- `POST /albums` - Adds a new album

## Setup & Run

### Local Development
```bash
# Initialize module
go mod init example/web-service-gin

# Install dependencies
go get .

# Run server
go run .
```

Server runs on `http://localhost:8080`

### Google Cloud Platform Deployment
1. Deploy to GCP VM (Ubuntu 22.04)
2. Update `main.go` to use `0.0.0.0:8080` instead of `localhost:8080`
3. Configure firewall rule for port 8080
4. Run server on VM

## Testing
```bash
# Get all albums
curl http://localhost:8080/albums

# Get specific album
curl http://localhost:8080/albums/2

# Add new album
curl http://localhost:8080/albums \
  --header "Content-Type: application/json" \
  --request "POST" \
  --data '{"id": "4","title": "Example Album","artist": "Example Artist","price": 29.99}'
```

## Technologies
- Go 1.23.0
- Gin Web Framework
- Google Cloud Platform
