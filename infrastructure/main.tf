terraform {
  required_version = ">= 1.6.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.95"
    }
  }
}

provider "aws" {
  region = var.aws_region
}

data "aws_caller_identity" "current" {}

resource "aws_default_vpc" "default" {}

data "aws_subnets" "default_vpc" {
  filter {
    name   = "vpc-id"
    values = [aws_default_vpc.default.id]
  }
}

data "aws_elastic_beanstalk_solution_stack" "docker" {
  most_recent = true
  name_regex  = "^64bit Amazon Linux 2023.*running Docker$"
}

locals {
  default_vpc_subnet_ids = sort(data.aws_subnets.default_vpc.ids)
}

resource "aws_s3_bucket" "media" {
  bucket = var.media_bucket_name
}

resource "aws_s3_bucket_public_access_block" "media" {
  bucket = aws_s3_bucket.media.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket_versioning" "media" {
  bucket = aws_s3_bucket.media.id

  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_security_group" "eb" {
  name        = "${var.project_name}-eb-sg"
  description = "Security group for Elastic Beanstalk instances"
  vpc_id      = aws_default_vpc.default.id

  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_security_group" "rds" {
  name        = "${var.project_name}-rds-sg"
  description = "Security group for PostgreSQL RDS"
  vpc_id      = aws_default_vpc.default.id

  ingress {
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [aws_security_group.eb.id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_db_subnet_group" "main" {
  name       = "${var.project_name}-db-subnets"
  subnet_ids = local.default_vpc_subnet_ids
}

resource "aws_db_instance" "postgres" {
  identifier             = "${var.project_name}-postgres"
  engine                 = "postgres"
  engine_version         = "16"
  instance_class         = var.db_instance_class
  storage_type           = "gp3"
  allocated_storage      = 20
  username               = var.db_username
  password               = var.db_password
  db_name                = var.db_name
  db_subnet_group_name   = aws_db_subnet_group.main.name
  vpc_security_group_ids = [aws_security_group.rds.id]
  publicly_accessible    = false
  skip_final_snapshot    = true
  backup_retention_period = 0
}

resource "aws_cognito_user_pool" "app" {
  count = var.enable_cognito ? 1 : 0

  name = "${var.project_name}-user-pool"

  auto_verified_attributes = ["email"]
}

resource "aws_cognito_user_pool_client" "app" {
  count = var.enable_cognito ? 1 : 0

  name         = "${var.project_name}-client"
  user_pool_id = aws_cognito_user_pool.app[0].id

  explicit_auth_flows = [
    "ALLOW_USER_PASSWORD_AUTH",
    "ALLOW_USER_SRP_AUTH",
    "ALLOW_REFRESH_TOKEN_AUTH"
  ]

  generate_secret = false
}

resource "aws_elastic_beanstalk_application" "backend" {
  name = "${var.project_name}-backend"
}

resource "aws_elastic_beanstalk_application" "frontend" {
  name = "${var.project_name}-frontend"
}

resource "aws_s3_bucket" "eb_versions" {
  bucket = "${var.project_name}-eb-versions-${var.aws_region}-${data.aws_caller_identity.current.account_id}"
}

resource "aws_s3_bucket_public_access_block" "eb_versions" {
  bucket = aws_s3_bucket.eb_versions.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_elastic_beanstalk_application_version" "backend" {
  name        = var.backend_version_label
  application = aws_elastic_beanstalk_application.backend.name
  bucket      = aws_s3_bucket.eb_versions.bucket
  key         = var.backend_source_bundle_key
}

resource "aws_elastic_beanstalk_application_version" "frontend" {
  name        = var.frontend_version_label
  application = aws_elastic_beanstalk_application.frontend.name
  bucket      = aws_s3_bucket.eb_versions.bucket
  key         = var.frontend_source_bundle_key
}

resource "aws_elastic_beanstalk_environment" "backend" {
  name                = "${var.project_name}-backend-env-v4"
  application         = aws_elastic_beanstalk_application.backend.name
  solution_stack_name = data.aws_elastic_beanstalk_solution_stack.docker.name
  version_label       = aws_elastic_beanstalk_application_version.backend.name
  wait_for_ready_timeout = "45m"
  depends_on          = [aws_security_group.eb, aws_security_group.rds]

  setting {
    namespace = "aws:elasticbeanstalk:command"
    name      = "Timeout"
    value     = "1800"
  }

  setting {
    namespace = "aws:elasticbeanstalk:application"
    name      = "Application Healthcheck URL"
    value     = "/health"
  }

  setting {
    namespace = "aws:elasticbeanstalk:cloudwatch:logs"
    name      = "StreamLogs"
    value     = "true"
  }

  setting {
    namespace = "aws:elasticbeanstalk:cloudwatch:logs"
    name      = "RetentionInDays"
    value     = "14"
  }

  setting {
    namespace = "aws:elasticbeanstalk:cloudwatch:logs"
    name      = "DeleteOnTerminate"
    value     = "false"
  }

  setting {
    namespace = "aws:elasticbeanstalk:hostmanager"
    name      = "LogPublicationControl"
    value     = "true"
  }

  setting {
    namespace = "aws:autoscaling:launchconfiguration"
    name      = "IamInstanceProfile"
    value     = var.eb_instance_profile_name
  }

  setting {
    namespace = "aws:ec2:vpc"
    name      = "VPCId"
    value     = aws_default_vpc.default.id
  }

  setting {
    namespace = "aws:ec2:vpc"
    name      = "Subnets"
    value     = join(",", local.default_vpc_subnet_ids)
  }

  setting {
    namespace = "aws:elasticbeanstalk:environment"
    name      = "EnvironmentType"
    value     = "SingleInstance"
  }

  setting {
    namespace = "aws:autoscaling:asg"
    name      = "MinSize"
    value     = "1"
  }

  setting {
    namespace = "aws:autoscaling:asg"
    name      = "MaxSize"
    value     = "1"
  }

  setting {
    namespace = "aws:autoscaling:launchconfiguration"
    name      = "SecurityGroups"
    value     = aws_security_group.eb.id
  }

  setting {
    namespace = "aws:elasticbeanstalk:application:environment"
    name      = "DB_HOST"
    value     = aws_db_instance.postgres.address
  }

  setting {
    namespace = "aws:elasticbeanstalk:application:environment"
    name      = "DB_PORT"
    value     = "5432"
  }

  setting {
    namespace = "aws:elasticbeanstalk:application:environment"
    name      = "DB_USER"
    value     = var.db_username
  }

  setting {
    namespace = "aws:elasticbeanstalk:application:environment"
    name      = "DB_PASSWORD"
    value     = var.db_password
  }

  setting {
    namespace = "aws:elasticbeanstalk:application:environment"
    name      = "DB_NAME"
    value     = var.db_name
  }

  setting {
    namespace = "aws:elasticbeanstalk:application:environment"
    name      = "DB_SSLMODE"
    value     = "require"
  }

  setting {
    namespace = "aws:elasticbeanstalk:application:environment"
    name      = "AUTH_PROVIDER"
    value     = var.enable_cognito ? "cognito" : "local"
  }

  dynamic "setting" {
    for_each = var.enable_cognito ? [1] : []
    content {
      namespace = "aws:elasticbeanstalk:application:environment"
      name      = "COGNITO_REGION"
      value     = var.aws_region
    }
  }

  dynamic "setting" {
    for_each = var.enable_cognito ? [1] : []
    content {
      namespace = "aws:elasticbeanstalk:application:environment"
      name      = "COGNITO_CLIENT_ID"
      value     = aws_cognito_user_pool_client.app[0].id
    }
  }

  dynamic "setting" {
    for_each = var.enable_cognito ? [1] : []
    content {
      namespace = "aws:elasticbeanstalk:application:environment"
      name      = "COGNITO_USER_POOL_ID"
      value     = aws_cognito_user_pool.app[0].id
    }
  }

  setting {
    namespace = "aws:elasticbeanstalk:application:environment"
    name      = "FILE_STORAGE_PROVIDER"
    value     = "s3"
  }

  setting {
    namespace = "aws:elasticbeanstalk:application:environment"
    name      = "S3_BUCKET"
    value     = aws_s3_bucket.media.bucket
  }

  setting {
    namespace = "aws:elasticbeanstalk:application:environment"
    name      = "AWS_REGION"
    value     = var.aws_region
  }

  setting {
    namespace = "aws:autoscaling:launchconfiguration"
    name      = "InstanceType"
    value     = "t2.micro"
  }
}

resource "aws_elastic_beanstalk_environment" "frontend" {
  name                = "${var.project_name}-frontend-env"
  application         = aws_elastic_beanstalk_application.frontend.name
  solution_stack_name = data.aws_elastic_beanstalk_solution_stack.docker.name
  version_label       = aws_elastic_beanstalk_application_version.frontend.name
  wait_for_ready_timeout = "45m"
  depends_on          = [aws_security_group.eb]

  setting {
    namespace = "aws:autoscaling:launchconfiguration"
    name      = "IamInstanceProfile"
    value     = var.eb_instance_profile_name
  }

  setting {
    namespace = "aws:elasticbeanstalk:cloudwatch:logs"
    name      = "StreamLogs"
    value     = "true"
  }

  setting {
    namespace = "aws:elasticbeanstalk:cloudwatch:logs"
    name      = "RetentionInDays"
    value     = "14"
  }

  setting {
    namespace = "aws:elasticbeanstalk:cloudwatch:logs"
    name      = "DeleteOnTerminate"
    value     = "false"
  }

  setting {
    namespace = "aws:elasticbeanstalk:hostmanager"
    name      = "LogPublicationControl"
    value     = "true"
  }

  setting {
    namespace = "aws:ec2:vpc"
    name      = "VPCId"
    value     = aws_default_vpc.default.id
  }

  setting {
    namespace = "aws:ec2:vpc"
    name      = "Subnets"
    value     = join(",", local.default_vpc_subnet_ids)
  }

  setting {
    namespace = "aws:elasticbeanstalk:environment"
    name      = "EnvironmentType"
    value     = "SingleInstance"
  }

  setting {
    namespace = "aws:autoscaling:asg"
    name      = "MinSize"
    value     = "1"
  }

  setting {
    namespace = "aws:autoscaling:asg"
    name      = "MaxSize"
    value     = "1"
  }

  setting {
    namespace = "aws:autoscaling:launchconfiguration"
    name      = "SecurityGroups"
    value     = aws_security_group.eb.id
  }

  setting {
    namespace = "aws:autoscaling:launchconfiguration"
    name      = "InstanceType"
    value     = "t2.micro"
  }
}

resource "aws_cloudwatch_log_group" "backend" {
  count = var.manage_cloudwatch_log_groups ? 1 : 0

  name              = "/aws/elasticbeanstalk/${var.project_name}-backend"
  retention_in_days = 3
}

resource "aws_cloudwatch_log_group" "frontend" {
  count = var.manage_cloudwatch_log_groups ? 1 : 0

  name              = "/aws/elasticbeanstalk/${var.project_name}-frontend"
  retention_in_days = 3
}
