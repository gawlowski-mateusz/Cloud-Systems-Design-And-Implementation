variable "aws_region" {
  description = "AWS region for infrastructure"
  type        = string
  default     = "us-east-1"

  validation {
    condition     = var.aws_region == "us-east-1"
    error_message = "This Terraform setup is locked to Learner Lab and must run in us-east-1."
  }
}

variable "enable_cognito" {
  description = "Whether to create and use Cognito resources"
  type        = bool
  default     = false

  validation {
    condition     = var.enable_cognito == false
    error_message = "Learner Lab policy blocks Cognito APIs, so enable_cognito must stay false."
  }
}

variable "manage_cloudwatch_log_groups" {
  description = "Whether Terraform should manage dedicated CloudWatch log groups"
  type        = bool
  default     = false

  validation {
    condition     = var.manage_cloudwatch_log_groups == false
    error_message = "Learner Lab policy blocks CloudWatch Logs Describe APIs, so manage_cloudwatch_log_groups must stay false."
  }
}

variable "eb_instance_profile_name" {
  description = "Pre-existing EC2 instance profile used by Elastic Beanstalk in Learner Lab"
  type        = string
  default     = "LabInstanceProfile"

  validation {
    condition     = var.eb_instance_profile_name == "LabInstanceProfile"
    error_message = "This setup is locked to Learner Lab and must use the LabInstanceProfile instance profile."
  }
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
