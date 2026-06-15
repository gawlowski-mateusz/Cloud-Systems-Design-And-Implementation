# HTTP API fronting the reservation Lambdas. Cheaper/simpler than a REST API and
# supports a native Cognito JWT authorizer.
resource "aws_apigatewayv2_api" "reservations" {
  name          = "${var.project_name}-reservations-api"
  protocol_type = "HTTP"

  cors_configuration {
    allow_origins = ["*"]
    allow_methods = ["GET", "POST", "OPTIONS"]
    allow_headers = ["authorization", "content-type"]
  }
}

# API Gateway validates the Cognito id token before invoking a Lambda, so the
# functions trust the claims and never re-verify the signature. Audience is the
# user pool client id; issuer is the user pool.
resource "aws_apigatewayv2_authorizer" "cognito" {
  api_id           = aws_apigatewayv2_api.reservations.id
  name             = "${var.project_name}-cognito-jwt"
  authorizer_type  = "JWT"
  identity_sources = ["$request.header.Authorization"]

  jwt_configuration {
    audience = [aws_cognito_user_pool_client.app.id]
    issuer   = "https://cognito-idp.${var.aws_region}.amazonaws.com/${aws_cognito_user_pool.app.id}"
  }
}

resource "aws_apigatewayv2_integration" "reservation" {
  for_each = local.reservation_lambdas

  api_id                 = aws_apigatewayv2_api.reservations.id
  integration_type       = "AWS_PROXY"
  integration_uri        = aws_lambda_function.reservation[each.key].invoke_arn
  payload_format_version = "2.0"
}

resource "aws_apigatewayv2_route" "reservation" {
  for_each = local.reservation_lambdas

  api_id             = aws_apigatewayv2_api.reservations.id
  route_key          = each.value.route_key
  target             = "integrations/${aws_apigatewayv2_integration.reservation[each.key].id}"
  authorization_type = "JWT"
  authorizer_id      = aws_apigatewayv2_authorizer.cognito.id
}

resource "aws_apigatewayv2_stage" "default" {
  api_id      = aws_apigatewayv2_api.reservations.id
  name        = "$default"
  auto_deploy = true
}

# Allow this specific API to invoke each function. source_arn is scoped to the
# API's execution ARN (any stage/method/route under it).
resource "aws_lambda_permission" "apigw" {
  for_each = local.reservation_lambdas

  statement_id  = "AllowInvokeFromApiGateway"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.reservation[each.key].function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.reservations.execution_arn}/*/*"
}
