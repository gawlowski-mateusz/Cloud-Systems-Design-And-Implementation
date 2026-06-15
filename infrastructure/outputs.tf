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

output "reservations_api_url" {
  description = "API Gateway base URL for the reservation Lambdas (frontend RESERVATIONS_API_URL)"
  value       = aws_apigatewayv2_stage.default.invoke_url
}

output "media_bucket" {
  description = "S3 media bucket name"
  value       = aws_s3_bucket.media.bucket
}

output "sqs_queue_url" {
  description = "Main application events queue URL"
  value       = aws_sqs_queue.app_events.url
}

output "sqs_dlq_url" {
  description = "Dead Letter Queue URL"
  value       = aws_sqs_queue.app_events_dlq.url
}

output "notification_topic_arn" {
  description = "SNS topic that emails notification subscribers"
  value       = aws_sns_topic.notifications.arn
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
