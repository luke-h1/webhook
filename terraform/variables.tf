variable "env" {
  type        = string
  description = "The environment to deploy to"
  default     = "production"
}

variable "tags" {
  type        = map(string)
  description = "The tags to apply to the resources"
  default = {
    "Service"   = "Webhook"
    "ManagedBy" = "Terraform"
  }
}

variable "git_sha" {
  type        = string
  description = "the git sha that triggered the deployment"
  default     = "change-me"
}

variable "deployed_by" {
  type        = string
  description = "Who initiated the deployment?"
  default     = "luke-h1"
}

variable "project_name" {
  type        = string
  description = "name of the project"
  default     = "webhook"
}

variable "appeeky_api_key" {
  type        = string
  description = "Secret the /api/appeeky ?key is compared against. Empty disables the route."
  default     = ""
  sensitive   = true
}

variable "sentry_client_secret" {
  type        = string
  description = "Client secret of the Sentry internal integration, used to verify Sentry-Hook-Signature. Empty disables the route."
  default     = ""
  sensitive   = true
}

variable "discord_webhook_url" {
  type        = string
  description = "The discord webhook URL which should receive forwarded alerts"
  default     = ""
  sensitive   = true
}

variable "telegram_bot_token" {
  type        = string
  description = "The telegram bot token"
  default     = ""
  sensitive   = true
}

variable "telegram_chat_id" {
  type        = string
  description = "The telegram chat ID which should receive forwarded alerts"
  default     = ""
  sensitive   = true
}

variable "root_domain" {
  type        = string
  description = "The root domain for the custom domain"
  default     = "lhowsam.com"
}

variable "private_key" {
  type        = string
  description = "Private key of the Cloudflare origin certificate"
  sensitive   = true
}

variable "certificate_body" {
  type        = string
  description = "Body of the Cloudflare origin certificate"
  sensitive   = true
}

variable "certificate_chain" {
  type        = string
  description = "Cloudflare Origin CA root certificate"
}
