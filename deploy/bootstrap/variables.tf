variable "project_id" {
  description = "Existing Google Cloud project that hosts the API."
  type        = string
}

variable "region" {
  description = "Region for Cloud Run, the registry, and the state bucket."
  type        = string
  default     = "us-central1"
}

variable "github_repo" {
  description = "GitHub repository allowed to deploy, as owner/name."
  type        = string
  default     = "Jesse0Michael/prayer-deck"
}

variable "billing_account" {
  description = "Billing account ID for the budget alert. Leave empty to skip the alert."
  type        = string
  default     = ""
}

variable "budget_usd" {
  description = "Monthly budget in whole USD that triggers alert emails."
  type        = number
  default     = 1
}
