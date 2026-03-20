output "alb_dns" {
  description = "ALB DNS name - use this as your API base URL"
  value       = "http://${aws_lb.main.dns_name}"
}

output "ecr_url" {
  description = "ECR repository URL for pushing Docker image"
  value       = aws_ecr_repository.hw8.repository_url
}

output "rds_endpoint" {
  description = "RDS MySQL endpoint"
  value       = aws_db_instance.mysql.address
}

output "dynamodb_table" {
  description = "DynamoDB table name"
  value       = aws_dynamodb_table.carts.name
}

output "ecs_cluster" {
  description = "ECS cluster name"
  value       = aws_ecs_cluster.main.name
}

output "ecs_service" {
  description = "ECS service name"
  value       = aws_ecs_service.main.name
}