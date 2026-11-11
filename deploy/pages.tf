data "cloudflare_zone" "this" {
  count = local.custom_domain ? 1 : 0

  filter = {
    name = var.domain
  }
}

# Content is uploaded by CI with wrangler; Terraform owns the project and its domain.
resource "cloudflare_pages_project" "web" {
  account_id        = var.cloudflare_account_id
  name              = "prayer-deck"
  production_branch = "main"
}

resource "cloudflare_pages_domain" "web" {
  count = local.custom_domain ? 1 : 0

  account_id   = var.cloudflare_account_id
  project_name = cloudflare_pages_project.web.name
  name         = local.app_hostname
}

resource "cloudflare_dns_record" "web" {
  count = local.custom_domain ? 1 : 0

  zone_id = data.cloudflare_zone.this[0].zone_id
  name    = local.app_hostname
  type    = "CNAME"
  content = cloudflare_pages_project.web.subdomain
  ttl     = 1
  proxied = true
}
