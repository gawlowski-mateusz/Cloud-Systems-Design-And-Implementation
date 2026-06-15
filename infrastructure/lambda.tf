# Reservation operations, one Lambda per route. Binaries are cross-compiled to
# infrastructure/build/<name>/bootstrap by scripts/build-lambdas.* before apply.
locals {
  reservation_lambdas = {
    create-reservation = { route_key = "POST /reservations" }
    list-reservations  = { route_key = "GET /reservations" }
    get-reservation    = { route_key = "GET /reservations/{id}" }
  }
  lambda_build_dir = "${path.module}/build"
}

data "archive_file" "reservation_lambda" {
  for_each = local.reservation_lambdas

  type        = "zip"
  source_file = "${local.lambda_build_dir}/${each.key}/bootstrap"
  output_path = "${local.lambda_build_dir}/${each.key}.zip"
}

# Separate Lambda log groups (distinct from the /ecs/* Fargate groups). Created
# explicitly so retention is managed instead of relying on the default never-expire group.
resource "aws_cloudwatch_log_group" "reservation_lambda" {
  for_each = local.reservation_lambdas

  name              = "/aws/lambda/${var.project_name}-${each.key}"
  retention_in_days = var.lambda_log_retention_days
}

resource "aws_lambda_function" "reservation" {
  for_each = local.reservation_lambdas

  function_name = "${var.project_name}-${each.key}"
  role          = data.aws_iam_role.lab_role.arn
  runtime       = "provided.al2023" # custom runtime for the Go bootstrap binary
  handler       = "bootstrap"
  architectures = ["x86_64"]
  timeout       = 15

  filename         = data.archive_file.reservation_lambda[each.key].output_path
  source_code_hash = data.archive_file.reservation_lambda[each.key].output_base64sha256

  environment {
    variables = {
      RESERVATIONS_TABLE = aws_dynamodb_table.reservations.name
      SQS_QUEUE_URL      = aws_sqs_queue.app_events.url
    }
  }

  depends_on = [aws_cloudwatch_log_group.reservation_lambda]
}
