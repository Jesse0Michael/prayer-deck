variable "project_id" {
  description = "Google Cloud project that hosts the API."
  type        = string
}

variable "region" {
  description = "Region for Cloud Run."
  type        = string
  default     = "us-central1"
}

variable "domain" {
  description = "Cloudflare-managed zone to serve the app from, e.g. example.com. Leave empty to use the free pages.dev address."
  type        = string
  default     = ""
}

variable "app_subdomain" {
  description = "Subdomain the web app is served at when a domain is set."
  type        = string
  default     = "prayer"
}

variable "cloudflare_account_id" {
  type = string
}

variable "neon_org_id" {
  description = "Neon organization that owns the project. Leave null for a personal account."
  type        = string
  default     = null
}

variable "neon_region" {
  type    = string
  default = "aws-us-east-2"
}

variable "r2_location" {
  description = "R2 location hint."
  type        = string
  default     = "enam"
}

variable "google_client_id" {
  description = "OAuth web client ID for Google sign-in. Created by hand in the Google Cloud console."
  type        = string
}

variable "image" {
  description = "Initial API image. CI replaces it on every deploy, so only the first apply uses this."
  type        = string
  default     = "us-docker.pkg.dev/cloudrun/container/hello"
}
