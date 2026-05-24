resource "aws_ecs_cluster" "main" {
  name = "${var.project_name}-cluster"

  setting {
    name  = "containerInsights"
    value = "disabled"
  }
}

locals {
  region     = var.aws_region
  account_id = data.aws_caller_identity.current.account_id

  shared_env = {
    AWS_REGION           = var.aws_region
    COGNITO_USER_POOL_ID = aws_cognito_user_pool.app.id
    COGNITO_CLIENT_ID    = aws_cognito_user_pool_client.app.id
    GIN_MODE             = "release"
    PORT                 = "8080"
  }

  service_env = {
    auth = merge(local.shared_env, {
      DYNAMO_PROFILES_TABLE = aws_dynamodb_table.auth_profiles.name
    })
    reservations = merge(local.shared_env, {
      DB_HOST       = aws_db_instance.postgres.address
      DB_PORT       = "5432"
      DB_USER       = var.db_username
      DB_PASSWORD   = var.db_password
      DB_NAME       = var.db_name
      DB_SSLMODE    = "require"
      SNS_TOPIC_ARN = aws_sns_topic.app_events.arn
    })
    files = merge(local.shared_env, {
      S3_MEDIA_BUCKET    = aws_s3_bucket.media.bucket
      DYNAMO_FILES_TABLE = aws_dynamodb_table.file_metadata.name
      SNS_TOPIC_ARN      = aws_sns_topic.app_events.arn
    })
    notifications = merge(local.shared_env, {
      DYNAMO_NOTIFICATIONS_TABLE = aws_dynamodb_table.notification_history.name
      SNS_TOPIC_ARN              = aws_sns_topic.app_events.arn
    })
  }
}

resource "aws_ecs_task_definition" "app" {
  for_each = local.services

  family                   = "${var.project_name}-${each.key}"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = var.task_cpu
  memory                   = var.task_memory
  execution_role_arn       = data.aws_iam_role.lab_role.arn
  task_role_arn            = data.aws_iam_role.lab_role.arn

  container_definitions = jsonencode([
    {
      name      = each.key
      image     = "${aws_ecr_repository.service[each.key].repository_url}:${var.image_tag}"
      essential = true
      portMappings = [
        {
          containerPort = 8080
          protocol      = "tcp"
        }
      ]
      environment = [
        for k, v in local.service_env[each.key] : { name = k, value = tostring(v) }
      ]
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          "awslogs-group"         = aws_cloudwatch_log_group.service[each.key].name
          "awslogs-region"        = var.aws_region
          "awslogs-stream-prefix" = each.key
        }
      }
    }
  ])
}

resource "aws_ecs_service" "app" {
  for_each = local.services

  name            = "${var.project_name}-${each.key}"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.app[each.key].arn
  desired_count   = var.service_desired_count
  launch_type     = "FARGATE"

  deployment_minimum_healthy_percent = 50
  deployment_maximum_percent         = 200

  network_configuration {
    subnets          = local.default_subnet_ids
    security_groups  = [aws_security_group.ecs_tasks.id]
    assign_public_ip = true
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.service[each.key].arn
    container_name   = each.key
    container_port   = 8080
  }

  lifecycle {
    ignore_changes = [desired_count]
  }

  depends_on = [aws_lb_listener_rule.service]
}

resource "aws_appautoscaling_target" "service" {
  for_each = local.services

  max_capacity       = var.service_max_capacity
  min_capacity       = var.service_min_capacity
  resource_id        = "service/${aws_ecs_cluster.main.name}/${aws_ecs_service.app[each.key].name}"
  scalable_dimension = "ecs:service:DesiredCount"
  service_namespace  = "ecs"
}

resource "aws_appautoscaling_policy" "cpu" {
  for_each = local.services

  name               = "${var.project_name}-${each.key}-cpu-target"
  policy_type        = "TargetTrackingScaling"
  resource_id        = aws_appautoscaling_target.service[each.key].resource_id
  scalable_dimension = aws_appautoscaling_target.service[each.key].scalable_dimension
  service_namespace  = aws_appautoscaling_target.service[each.key].service_namespace

  target_tracking_scaling_policy_configuration {
    predefined_metric_specification {
      predefined_metric_type = "ECSServiceAverageCPUUtilization"
    }
    target_value       = 70
    scale_in_cooldown  = 120
    scale_out_cooldown = 60
  }
}
