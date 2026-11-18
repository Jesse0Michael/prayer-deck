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

output "measurement_id" {
  description = "Public GA4 web stream ID for the production Pages build."
  value       = googleanalytics_data_stream.web.measurement_id
  depends_on  = [terraform_data.enhanced_measurement]
}

output "analytics_property_id" {
  description = "Numeric ID of the Prayer Deck GA4 property shown in Google Analytics."
  value       = trimprefix(googleanalytics_property.app.name, "properties/")
}

output "analytics_hostname" {
  description = "Canonical hostname allowed to load the Google tag."
  value       = local.app_hostname
  depends_on  = [googleanalytics_data_stream.web]
}
