output "api_url" {
  description = "Public API URL, baked into the web build as VITE_API_URL."
  value       = google_cloud_run_v2_service.api.uri
}

output "app_url" {
  value = local.app_origin
}

output "pages_project" {
  value = cloudflare_pages_project.web.name
}
