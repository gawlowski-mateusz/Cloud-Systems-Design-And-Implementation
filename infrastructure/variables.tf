variable "aws_region" {
  description = "AWS region for infrastructure"
  type        = string
  default     = "us-east-1"

  validation {
    condition     = var.aws_region == "us-east-1"
    error_message = "This setup is locked to AWS Academy Learner Lab and must run in us-east-1."
  }
}

variable "project_name" {
  description = "Project slug used in AWS resource names"
  type        = string
  default     = "conference-app"
}

variable "lab_role_name" {
  description = "Pre-existing IAM role provided by AWS Academy Learner Lab"
  type        = string
  default     = "LabRole"
}

variable "image_tag" {
  description = "Container image tag deployed to ECS"
  type        = string
  default     = "latest"
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
  description = "PostgreSQL admin password (min 8 chars)"
  type        = string
  sensitive   = true
}

variable "service_desired_count" {
  description = "Desired number of running tasks per ECS service"
  type        = number
  default     = 2
}

variable "service_min_capacity" {
  description = "Min capacity for ECS service auto-scaling"
  type        = number
  default     = 2
}

variable "service_max_capacity" {
  description = "Max capacity for ECS service auto-scaling"
  type        = number
  default     = 4
}

variable "task_cpu" {
  description = "Fargate task CPU units (256 = 0.25 vCPU)"
  type        = string
  default     = "256"
}

variable "task_memory" {
  description = "Fargate task memory in MiB"
  type        = string
  default     = "512"
}

variable "enable_sns_subscription" {
  description = "Create the SNS->ALB HTTP subscription. Set to true only after ECS services are running (runningCount=2), so SNS can auto-confirm by POSTing to /notifications/sns."
  type        = bool
  default     = false
}
