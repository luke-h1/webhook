locals {
  domain_name = "webhook.${var.root_domain}"
}

resource "aws_apigatewayv2_api" "webhook" {
  name                         = "${var.project_name}-gw-${var.env}"
  protocol_type                = "HTTP"
  disable_execute_api_endpoint = true

  tags = merge(var.tags, {
    Environment = var.env
  })
}

resource "aws_apigatewayv2_stage" "webhook" {
  api_id      = aws_apigatewayv2_api.webhook.id
  name        = "$default"
  auto_deploy = true
  default_route_settings {
    throttling_burst_limit = 50
    throttling_rate_limit  = 20
  }
  access_log_settings {
    destination_arn = aws_cloudwatch_log_group.api_gw.arn
    format = jsonencode({
      requestId               = "$context.requestId"
      requestTime             = "$context.requestTime"
      httpMethod              = "$context.httpMethod"
      routeKey                = "$context.routeKey"
      status                  = "$context.status"
      responseLatency         = "$context.responseLatency"
      integrationLatency      = "$context.integration.latency"
      integrationErrorMessage = "$context.integrationErrorMessage"
      sourceIp                = "$context.identity.sourceIp"
      userAgent               = "$context.identity.userAgent"
    })
  }
  tags = merge(var.tags, {
    Environment = var.env
  })
}

resource "aws_apigatewayv2_domain_name" "webhook" {
  domain_name = local.domain_name

  domain_name_configuration {
    certificate_arn = aws_acm_certificate.webhook.arn
    endpoint_type   = "REGIONAL"
    security_policy = "TLS_1_2"
  }

  tags = merge(var.tags, {
    Environment = var.env
  })
}

resource "aws_apigatewayv2_api_mapping" "webhook" {
  api_id      = aws_apigatewayv2_api.webhook.id
  domain_name = aws_apigatewayv2_domain_name.webhook.domain_name
  stage       = aws_apigatewayv2_stage.webhook.id
}

resource "aws_apigatewayv2_integration" "webhook" {
  api_id                 = aws_apigatewayv2_api.webhook.id
  integration_uri        = aws_lambda_function.webhook.invoke_arn
  integration_type       = "AWS_PROXY"
  integration_method     = "POST"
  payload_format_version = "1.0"
}

resource "aws_apigatewayv2_route" "appeeky" {
  api_id         = aws_apigatewayv2_api.webhook.id
  target         = "integrations/${aws_apigatewayv2_integration.webhook.id}"
  route_key      = "POST /api/appeeky"
  operation_name = "post appeeky"
}

resource "aws_apigatewayv2_route" "sentry" {
  api_id         = aws_apigatewayv2_api.webhook.id
  target         = "integrations/${aws_apigatewayv2_integration.webhook.id}"
  route_key      = "POST /api/sentry"
  operation_name = "post sentry"
}

resource "aws_cloudwatch_log_group" "api_gw" {
  name              = "/aws/api_gw/${aws_apigatewayv2_api.webhook.name}"
  retention_in_days = 14
  log_group_class   = "STANDARD"

  tags = merge(var.tags, {
    Environment = var.env
  })
}

resource "aws_lambda_permission" "api_gw" {
  statement_id  = "AllowExecutionFromAPIGateway"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.webhook.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.webhook.execution_arn}/*/*"
}
