# Homework 1B - AWS EC2 Deployment

## Overview
Deployed Go-Gin web service on AWS EC2 and performed load testing.

## Files
- `main.go` - Go web service (modified for 0.0.0.0:8080)
- `load_test.py` - Python performance testing script
- `go.mod`, `go.sum` - Go dependencies

## Deployment Details
- **Instance Type:** t2.micro
- **OS:** Amazon Linux 2023  
- **Region:** us-west-2 (Oregon)
- **Port:** 8080

## Performance Testing Results
- Total requests: 436 in 30 seconds
- Average response time: 68.24ms
- Median: 63.27ms
- 95th percentile: 84.54ms
- 99th percentile: 204.06ms
- Max: 312.08ms

## Testing Commands
```bash
# Test endpoint
curl http://<EC2-PUBLIC-IP>:8080/albums

# Run load test
python load_test.py
```
