mock_provider "googleanalytics" {}
mock_provider "google" {}
mock_provider "cloudflare" {}
mock_provider "neon" {}
mock_provider "random" {}

variables {
  project_id            = "test-project"
  cloudflare_account_id = "0123456789abcdef0123456789abcdef"
  google_client_id      = "test-client"
  analytics_account_id  = "123456789"
}

run "analytics_contract" {
  command = plan
  plan_options {
    target = [
      terraform_data.enhanced_measurement,
      googleanalytics_custom_dimension.event,
      googleanalytics_custom_metric.hand_size,
      googleanalytics_key_event.login,
      googleanalytics_data_retention_settings.app,
    ]
  }
  variables {
    domain = "example.com"
  }
  assert {
    condition = (
      googleanalytics_data_stream.web.default_uri == "https://prayer.example.com" &&
      output.analytics_hostname == "prayer.example.com"
    )
    error_message = "The stream URL and web build must agree on the public app hostname."
  }
  assert {
    condition = (
      toset(keys(googleanalytics_custom_dimension.event)) == toset(["deck_mode", "draw_type"]) &&
      googleanalytics_custom_metric.hand_size.parameter_name == "hand_size" &&
      googleanalytics_key_event.login.event_name == "login"
    )
    error_message = "The GA4 report definitions must match the events emitted by the client."
  }
}
