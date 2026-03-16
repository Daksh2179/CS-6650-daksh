output "alb_dns_name" {
  value = aws_lb.main.dns_name
}

output "sns_topic_arn" {
  value = aws_sns_topic.orders.arn
}

output "sqs_queue_url" {
  value = aws_sqs_queue.orders.url
}

output "ecr_repository_url" {
  value = data.aws_ecr_repository.hw7.repository_url
}