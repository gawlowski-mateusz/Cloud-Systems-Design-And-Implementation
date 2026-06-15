# Fargate log groups (one per microservice). The Lambda groups live in lambda.tf
# so logs from the two compute models stay clearly separated.
resource "aws_cloudwatch_log_group" "service" {
  for_each = local.services

  name              = "/ecs/${var.project_name}/${each.key}"
  retention_in_days = 3
}

locals {
  notif_err_namespace = "${var.project_name}/notifications"
}

# The notifications service has no built-in CloudWatch error metric, so we derive
# one from its log group: any line mentioning a failure increments the counter.
resource "aws_cloudwatch_log_metric_filter" "notification_errors" {
  name           = "${var.project_name}-notification-errors"
  log_group_name = aws_cloudwatch_log_group.service["notifications"].name
  pattern        = "?failed ?error"

  metric_transformation {
    name          = "NotificationServiceErrors"
    namespace     = local.notif_err_namespace
    value         = "1"
    default_value = "0"
  }
}

# --- Alarms -----------------------------------------------------------------

resource "aws_cloudwatch_metric_alarm" "lambda_errors" {
  for_each = aws_lambda_function.reservation

  alarm_name          = "${each.value.function_name}-errors"
  alarm_description   = "Reservation Lambda is returning errors"
  comparison_operator = "GreaterThanOrEqualToThreshold"
  evaluation_periods  = 1
  period              = 60
  namespace           = "AWS/Lambda"
  metric_name         = "Errors"
  statistic           = "Sum"
  threshold           = 1
  treat_missing_data  = "notBreaching"

  dimensions = {
    FunctionName = each.value.function_name
  }
}

resource "aws_cloudwatch_metric_alarm" "lambda_duration" {
  for_each = aws_lambda_function.reservation

  alarm_name          = "${each.value.function_name}-duration"
  alarm_description   = "Reservation Lambda is running slow (approaching its timeout)"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 3
  period              = 60
  namespace           = "AWS/Lambda"
  metric_name         = "Duration"
  statistic           = "Average"
  threshold           = 10000 # ms, ~two-thirds of the 15s function timeout
  treat_missing_data  = "notBreaching"

  dimensions = {
    FunctionName = each.value.function_name
  }
}

# Backlog building up means the notifications consumer is not keeping pace.
resource "aws_cloudwatch_metric_alarm" "sqs_backlog" {
  alarm_name          = "${var.project_name}-sqs-backlog"
  alarm_description   = "Too many unprocessed messages in the events queue"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  period              = 60
  namespace           = "AWS/SQS"
  metric_name         = "ApproximateNumberOfMessagesVisible"
  statistic           = "Maximum"
  threshold           = 100
  treat_missing_data  = "notBreaching"

  dimensions = {
    QueueName = aws_sqs_queue.app_events.name
  }
}

# Any message in the DLQ is a failed event that needs attention.
resource "aws_cloudwatch_metric_alarm" "dlq_messages" {
  alarm_name          = "${var.project_name}-dlq-not-empty"
  alarm_description   = "Messages have landed in the Dead Letter Queue"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 1
  period              = 60
  namespace           = "AWS/SQS"
  metric_name         = "ApproximateNumberOfMessagesVisible"
  statistic           = "Maximum"
  threshold           = 0
  treat_missing_data  = "notBreaching"

  dimensions = {
    QueueName = aws_sqs_queue.app_events_dlq.name
  }
}

resource "aws_cloudwatch_metric_alarm" "notification_errors" {
  alarm_name          = "${var.project_name}-notification-errors"
  alarm_description   = "Notifications microservice logged processing errors"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 1
  period              = 300
  namespace           = local.notif_err_namespace
  metric_name         = "NotificationServiceErrors"
  statistic           = "Sum"
  threshold           = 0
  treat_missing_data  = "notBreaching"
}

# --- Dashboard --------------------------------------------------------------

resource "aws_cloudwatch_dashboard" "main" {
  dashboard_name = "${var.project_name}-overview"

  dashboard_body = jsonencode({
    widgets = [
      {
        type   = "metric"
        x      = 0
        y      = 0
        width  = 12
        height = 6
        properties = {
          title   = "Lambda invocations"
          region  = var.aws_region
          view    = "timeSeries"
          stat    = "Sum"
          metrics = [for fn in aws_lambda_function.reservation : ["AWS/Lambda", "Invocations", "FunctionName", fn.function_name]]
        }
      },
      {
        type   = "metric"
        x      = 12
        y      = 0
        width  = 12
        height = 6
        properties = {
          title   = "Lambda errors"
          region  = var.aws_region
          view    = "timeSeries"
          stat    = "Sum"
          metrics = [for fn in aws_lambda_function.reservation : ["AWS/Lambda", "Errors", "FunctionName", fn.function_name]]
        }
      },
      {
        type   = "metric"
        x      = 0
        y      = 6
        width  = 12
        height = 6
        properties = {
          title   = "Lambda duration (avg ms)"
          region  = var.aws_region
          view    = "timeSeries"
          stat    = "Average"
          metrics = [for fn in aws_lambda_function.reservation : ["AWS/Lambda", "Duration", "FunctionName", fn.function_name]]
        }
      },
      {
        type   = "metric"
        x      = 12
        y      = 6
        width  = 12
        height = 6
        properties = {
          title  = "SQS queue depth (main vs DLQ)"
          region = var.aws_region
          view   = "timeSeries"
          stat   = "Maximum"
          metrics = [
            ["AWS/SQS", "ApproximateNumberOfMessagesVisible", "QueueName", aws_sqs_queue.app_events.name],
            ["AWS/SQS", "ApproximateNumberOfMessagesVisible", "QueueName", aws_sqs_queue.app_events_dlq.name],
          ]
        }
      },
      {
        type   = "metric"
        x      = 0
        y      = 12
        width  = 12
        height = 6
        properties = {
          title   = "Notification service errors"
          region  = var.aws_region
          view    = "timeSeries"
          stat    = "Sum"
          metrics = [[local.notif_err_namespace, "NotificationServiceErrors"]]
        }
      },
      {
        type   = "metric"
        x      = 12
        y      = 12
        width  = 12
        height = 6
        properties = {
          title  = "Notifications ECS utilisation"
          region = var.aws_region
          view   = "timeSeries"
          stat   = "Average"
          metrics = [
            ["AWS/ECS", "CPUUtilization", "ClusterName", aws_ecs_cluster.main.name, "ServiceName", aws_ecs_service.app["notifications"].name],
            ["AWS/ECS", "MemoryUtilization", "ClusterName", aws_ecs_cluster.main.name, "ServiceName", aws_ecs_service.app["notifications"].name],
          ]
        }
      },
    ]
  })
}
