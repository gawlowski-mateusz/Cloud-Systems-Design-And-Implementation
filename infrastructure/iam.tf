data "aws_caller_identity" "current" {}

# AWS Academy Learner Lab forbids creating IAM roles, so the pre-provisioned
# LabRole is reused everywhere it would normally be a dedicated role: ECS task +
# execution role, and the Lambda execution role. Its trust policy already allows
# lambda.amazonaws.com and ecs-tasks.amazonaws.com, and it carries broad
# managed permissions (DynamoDB, SQS, S3, CloudWatch Logs).
data "aws_iam_role" "lab_role" {
  name = var.lab_role_name
}
