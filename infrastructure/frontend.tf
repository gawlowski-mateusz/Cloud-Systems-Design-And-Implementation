resource "aws_s3_bucket" "frontend" {
  bucket        = "${var.project_name}-frontend-${data.aws_caller_identity.current.account_id}"
  force_destroy = true
}

resource "aws_s3_bucket_public_access_block" "frontend" {
  bucket = aws_s3_bucket.frontend.id

  block_public_acls       = false
  block_public_policy     = false
  ignore_public_acls      = false
  restrict_public_buckets = false
}

resource "aws_s3_bucket_website_configuration" "frontend" {
  bucket = aws_s3_bucket.frontend.id

  index_document {
    suffix = "index.html"
  }

  error_document {
    key = "index.html"
  }
}

resource "aws_s3_bucket_ownership_controls" "frontend" {
  bucket = aws_s3_bucket.frontend.id

  rule {
    object_ownership = "BucketOwnerEnforced"
  }
}

resource "aws_s3_bucket_policy" "frontend_public" {
  bucket = aws_s3_bucket.frontend.id
  depends_on = [
    aws_s3_bucket_public_access_block.frontend,
    aws_s3_bucket_ownership_controls.frontend,
  ]

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid       = "PublicReadGetObject"
        Effect    = "Allow"
        Principal = "*"
        Action    = "s3:GetObject"
        Resource  = "${aws_s3_bucket.frontend.arn}/*"
      }
    ]
  })
}

locals {
  frontend_dir = "${path.module}/../frontend"
  frontend_static_files = {
    "index.html" = "text/html"
    "app.js"     = "application/javascript"
    "styles.css" = "text/css"
  }
}

resource "aws_s3_object" "frontend_static" {
  for_each = local.frontend_static_files

  bucket       = aws_s3_bucket.frontend.id
  key          = each.key
  source       = "${local.frontend_dir}/${each.key}"
  etag         = filemd5("${local.frontend_dir}/${each.key}")
  content_type = each.value

  depends_on = [aws_s3_bucket_policy.frontend_public]
}

resource "aws_s3_object" "frontend_config" {
  bucket       = aws_s3_bucket.frontend.id
  key          = "config.js"
  content_type = "application/javascript"

  content = templatefile("${path.module}/templates/config.js.tftpl", {
    api_base_url         = "http://${aws_lb.main.dns_name}"
    cognito_user_pool_id = aws_cognito_user_pool.app.id
    cognito_client_id    = aws_cognito_user_pool_client.app.id
    aws_region           = var.aws_region
  })

  depends_on = [aws_s3_bucket_policy.frontend_public]
}
