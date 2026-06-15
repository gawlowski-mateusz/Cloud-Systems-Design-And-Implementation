resource "aws_lb" "main" {
  name               = "${var.project_name}-alb"
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb.id]
  subnets            = local.default_subnet_ids
}

resource "aws_lb_listener" "http" {
  load_balancer_arn = aws_lb.main.arn
  port              = 80
  protocol          = "HTTP"

  default_action {
    type = "fixed-response"
    fixed_response {
      content_type = "application/json"
      status_code  = "404"
      message_body = "{\"error\":\"route not found\"}"
    }
  }
}

locals {
  # Each pattern list contains both the bare prefix (e.g. "/reservations") and the
  # subpath wildcard ("/reservations/*"). The bare prefix is required because ALB
  # path patterns of the form "/foo/*" do not match "/foo" — and the frontend hits
  # bare URLs like GET /reservations to list items.
  # reservations is intentionally absent: it now runs as Lambda behind API Gateway
  # (see lambda.tf / apigateway.tf), not as a Fargate service behind this ALB.
  services = {
    auth          = { paths = ["/auth", "/auth/*"], health = "/auth/health", priority = 10 }
    files         = { paths = ["/files", "/files/*"], health = "/files/health", priority = 30 }
    notifications = { paths = ["/notifications", "/notifications/*"], health = "/notifications/health", priority = 40 }
  }
}

resource "aws_lb_target_group" "service" {
  for_each = local.services

  name        = "${var.project_name}-tg-${each.key}"
  port        = 8080
  protocol    = "HTTP"
  target_type = "ip"
  vpc_id      = data.aws_vpc.default.id

  health_check {
    path                = each.value.health
    matcher             = "200-299"
    interval            = 30
    timeout             = 5
    healthy_threshold   = 2
    unhealthy_threshold = 3
  }
}

resource "aws_lb_listener_rule" "service" {
  for_each = local.services

  listener_arn = aws_lb_listener.http.arn
  priority     = each.value.priority

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.service[each.key].arn
  }

  condition {
    path_pattern {
      values = each.value.paths
    }
  }
}
