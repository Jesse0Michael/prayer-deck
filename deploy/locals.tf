locals {
  custom_domain = var.domain != ""
  app_hostname  = local.custom_domain ? "${var.app_subdomain}.${var.domain}" : cloudflare_pages_project.web.subdomain
  app_origin    = "https://${local.app_hostname}"
}
