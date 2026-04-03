output "lf_leader_ip" {
  description = "Leader-Follower leader public IP"
  value       = "Check ECS console for task public IP"
}

output "ecr_lf_url" {
  value = aws_ecr_repository.kv_lf.repository_url
}

output "ecr_ll_url" {
  value = aws_ecr_repository.kv_ll.repository_url
}