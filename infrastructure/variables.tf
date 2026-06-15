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

variable "lambda_log_retention_days" {
  description = "Retention for the reservation Lambda log groups"
  type        = number
  default     = 3
}

variable "sqs_max_receive_count" {
  description = "Deliveries attempted before a message is moved to the Dead Letter Queue"
  type        = number
  default     = 5
}

variable "notification_email" {
  description = <<-EOT
    Address subscribed to the notification SNS topic (use the email you log in
    with). After apply, confirm the subscription via the link SNS sends, otherwise
    no emails are delivered.
  EOT
  type        = string
}
