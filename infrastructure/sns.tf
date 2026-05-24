resource "aws_sns_topic" "app_events" {
  name = "${var.project_name}-app-events"
}

# Subscription auto-confirms by HTTP-callback to the notifications service.
# That requires the service to already be running, otherwise the apply hangs and
# times out. Apply once with the default (false) to bring infra up, push images,
# wait for runningCount=2, then re-apply with -var enable_sns_subscription=true.
resource "aws_sns_topic_subscription" "notifications" {
  count = var.enable_sns_subscription ? 1 : 0

  topic_arn              = aws_sns_topic.app_events.arn
  protocol               = "http"
  endpoint               = "http://${aws_lb.main.dns_name}/notifications/sns"
  endpoint_auto_confirms = true
  raw_message_delivery   = false

  confirmation_timeout_in_minutes = 5

  depends_on = [aws_ecs_service.app]
}
