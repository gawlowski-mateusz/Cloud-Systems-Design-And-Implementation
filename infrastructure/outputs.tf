output "alb_dns_name" {
  description = "Application Load Balancer DNS name (use as API_BASE_URL)"
  value       = aws_lb.main.dns_name
}

output "frontend_url" {
  description = "Public website URL for the frontend bucket"
  value       = "http://${aws_s3_bucket.frontend.bucket}.s3-website-${var.aws_region}.amazonaws.com"
}

output "cognito_user_pool_id" {
  description = "Cognito user pool id"
  value       = aws_cognito_user_pool.app.id
}

output "cognito_client_id" {
  description = "Cognito user pool client id"
  value       = aws_cognito_user_pool_client.app.id
}

output "rds_endpoint" {
  description = "PostgreSQL RDS endpoint"
  value       = aws_db_instance.postgres.address
}

output "media_bucket" {
  description = "S3 media bucket name"
  value       = aws_s3_bucket.media.bucket
}

output "sns_topic_arn" {
  description = "SNS topic ARN for app events"
  value       = aws_sns_topic.app_events.arn
}

output "ecs_cluster_name" {
  description = "ECS cluster name"
  value       = aws_ecs_cluster.main.name
}

output "ecr_repositories" {
  description = "ECR repository URLs per service"
  value       = { for k, v in aws_ecr_repository.service : k => v.repository_url }
}

output "service_names" {
  description = "ECS service names"
  value       = { for k, v in aws_ecs_service.app : k => v.name }
}
