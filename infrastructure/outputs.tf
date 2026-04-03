output "backend_url" {
  value = aws_elastic_beanstalk_environment.backend.endpoint_url
}

output "frontend_url" {
  value = aws_elastic_beanstalk_environment.frontend.endpoint_url
}

output "rds_address" {
  value = aws_db_instance.postgres.address
}

output "media_bucket" {
  value = aws_s3_bucket.media.bucket
}

output "cognito_user_pool_id" {
  value = aws_cognito_user_pool.app.id
}

output "cognito_client_id" {
  value = aws_cognito_user_pool_client.app.id
}
