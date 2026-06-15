# Notification emails are delivered through SNS. SES is unusable in the Learner
# Lab (the role is denied ses:VerifyEmailIdentity, so no identity can be verified
# and the sandbox refuses to send), whereas SNS email subscriptions work.
resource "aws_sns_topic" "notifications" {
  name = "${var.project_name}-notification-emails"
}

# Email subscriptions cannot be auto-confirmed by Terraform: the address owner
# must click the link SNS sends after apply before any email is delivered.
resource "aws_sns_topic_subscription" "email" {
  topic_arn = aws_sns_topic.notifications.arn
  protocol  = "email"
  endpoint  = var.notification_email
}
