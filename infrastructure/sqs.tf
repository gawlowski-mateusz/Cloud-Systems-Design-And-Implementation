# Event backbone: producers (reservation Lambdas, files service) send messages
# here and the notifications Fargate service long-polls them. Replaces the former
# SNS topic + HTTP subscription.

# Dead Letter Queue for messages that keep failing. 14-day retention (the SQS
# maximum) gives time to inspect poison messages before they expire.
resource "aws_sqs_queue" "app_events_dlq" {
  name                      = "${var.project_name}-app-events-dlq"
  message_retention_seconds = 1209600
}

resource "aws_sqs_queue" "app_events" {
  name = "${var.project_name}-app-events"

  # Must exceed the consumer's processing time so an in-flight message is not
  # redelivered to another receiver while still being handled.
  visibility_timeout_seconds = 60

  # After max_receive_count failed deliveries the message is parked in the DLQ
  # instead of being retried forever.
  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.app_events_dlq.arn
    maxReceiveCount     = var.sqs_max_receive_count
  })
}
