locals {
  lambda_runtime       = "provided.al2023"
  lambda_architectures = ["arm64"]

  build_dir   = "${path.module}/.."
  webhook_zip = "${local.build_dir}/build/webhook.zip"

  function_name = "${var.project_name}-lambda-${var.env}"
}

resource "aws_iam_role" "lambda_exec" {
  name = "${var.project_name}-${var.env}-exec-role"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action    = "sts:AssumeRole"
      Effect    = "Allow"
      Principal = { Service = "lambda.amazonaws.com" }
    }]
  })
}

resource "aws_iam_role_policy_attachment" "lambda_policy" {
  role       = aws_iam_role.lambda_exec.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

resource "aws_lambda_function" "webhook" {
  function_name    = local.function_name
  runtime          = local.lambda_runtime
  handler          = "bootstrap"
  role             = aws_iam_role.lambda_exec.arn
  filename         = local.webhook_zip
  source_code_hash = filebase64sha256(local.webhook_zip)
  timeout          = 15
  memory_size      = 128
  architectures    = local.lambda_architectures
  description      = "webhook-${var.env}"

  environment {
    variables = {
      DEPLOYED_AT          = timestamp()
      DEPLOYED_BY          = var.deployed_by
      GIT_SHA              = var.git_sha
      ENVIRONMENT          = var.env
      APPEEKY_API_KEY      = var.appeeky_api_key
      SENTRY_CLIENT_SECRET = var.sentry_client_secret
      DISCORD_WEBHOOK_URL  = var.discord_webhook_url
      TELEGRAM_BOT_TOKEN   = var.telegram_bot_token
      TELEGRAM_CHAT_ID     = var.telegram_chat_id
    }
  }

  tags = merge(var.tags, {
    Environment = var.env
  })

  depends_on = [aws_cloudwatch_log_group.lambda_logs]

  lifecycle {
    precondition {
      condition     = var.appeeky_api_key != "" || var.sentry_client_secret != ""
      error_message = "At least one of appeeky_api_key or sentry_client_secret is required."
    }
    precondition {
      condition     = var.discord_webhook_url != "" || (var.telegram_bot_token != "" && var.telegram_chat_id != "")
      error_message = "Forwarding requires discord_webhook_url or both telegram_bot_token and telegram_chat_id."
    }
    precondition {
      condition     = (var.telegram_bot_token == "") == (var.telegram_chat_id == "")
      error_message = "telegram_bot_token and telegram_chat_id must both be set or both empty."
    }
  }
}

resource "aws_cloudwatch_log_group" "lambda_logs" {
  name              = "/aws/lambda/${local.function_name}"
  retention_in_days = 14
  log_group_class   = "STANDARD"

  tags = merge(var.tags, {
    Environment = var.env
  })
}
