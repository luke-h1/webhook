output "function_name" {
  description = "The name of the lambda function"
  value       = aws_lambda_function.webhook.function_name
}

output "base_url" {
  description = "Custom domain URL of the webhook"
  value       = "https://${local.domain_name}"
}

output "cname_target" {
  description = "Target for the proxied Cloudflare CNAME record of the custom domain"
  value       = aws_apigatewayv2_domain_name.webhook.domain_name_configuration[0].target_domain_name
}

output "appeeky_webhook_url" {
  description = "ALERT_WEBHOOK_URL for Appeeky Spec, without the ?key=<appeeky_api_key> suffix"
  value       = "https://${local.domain_name}/api/appeeky"
}

output "sentry_webhook_url" {
  description = "Webhook URL for the Sentry internal integration"
  value       = "https://${local.domain_name}/api/sentry"
}
