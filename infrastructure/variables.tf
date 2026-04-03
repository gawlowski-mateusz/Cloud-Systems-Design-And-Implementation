variable "aws_region" {
  description = "AWS region for infrastructure"
  type        = string
  default     = "eu-central-1"
}

variable "project_name" {
  description = "Project slug used in AWS resource names"
  type        = string
  default     = "conference-app"
}

variable "db_instance_class" {
  description = "RDS instance class"
  type        = string
  default     = "db.t3.micro"
}

variable "db_name" {
  description = "PostgreSQL database name"
  type        = string
  default     = "conference"
}

variable "db_username" {
  description = "PostgreSQL admin username"
  type        = string
}

variable "db_password" {
  description = "PostgreSQL admin password"
  type        = string
  sensitive   = true
}

variable "media_bucket_name" {
  description = "S3 bucket name for uploaded files"
  type        = string
}

variable "backend_version_label" {
  description = "Elastic Beanstalk backend version label"
  type        = string
  default     = "v1"
}

variable "frontend_version_label" {
  description = "Elastic Beanstalk frontend version label"
  type        = string
  default     = "v1"
}

variable "backend_source_bundle_key" {
  description = "S3 object key with backend deployment bundle"
  type        = string
}

variable "frontend_source_bundle_key" {
  description = "S3 object key with frontend deployment bundle"
  type        = string
}
