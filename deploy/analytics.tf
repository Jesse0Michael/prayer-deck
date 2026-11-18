resource "googleanalytics_property" "app" {
  account_id    = var.analytics_account_id
  display_name  = "Prayer Deck"
  time_zone     = "America/Phoenix"
  currency_code = "USD"

  lifecycle {
    prevent_destroy = true
  }
}

resource "googleanalytics_data_stream" "web" {
  parent       = googleanalytics_property.app.name
  type         = "WEB_DATA_STREAM"
  display_name = "Prayer Deck web"
  default_uri  = local.app_origin

  lifecycle {
    prevent_destroy = true
  }
}

resource "googleanalytics_custom_dimension" "event" {
  for_each = {
    deck_mode = "Deck mode"
    draw_type = "Draw type"
  }

  parent         = googleanalytics_property.app.name
  parameter_name = each.key
  display_name   = each.value
  scope          = "EVENT"
}

resource "googleanalytics_custom_metric" "hand_size" {
  parent           = googleanalytics_property.app.name
  parameter_name   = "hand_size"
  display_name     = "Cards dealt"
  scope            = "EVENT"
  measurement_unit = "STANDARD"
}

resource "googleanalytics_key_event" "login" {
  parent          = googleanalytics_property.app.name
  event_name      = "login"
  counting_method = "ONCE_PER_EVENT"
}

resource "googleanalytics_data_retention_settings" "app" {
  parent                          = googleanalytics_property.app.name
  event_data_retention            = "TWO_MONTHS"
  reset_user_data_on_new_activity = false
}

# The provider lacks this stream setting. Automatic form and link tracking can
# capture private card details, so switch it off before publishing the tag.
resource "terraform_data" "enhanced_measurement" {
  triggers_replace = [googleanalytics_data_stream.web.name]

  provisioner "local-exec" {
    environment = {
      GA_STREAM = googleanalytics_data_stream.web.name
    }
    command = <<-SH
      curl --fail --silent --show-error --output /dev/null --max-time 45 \
        --request PATCH --header "Authorization: Bearer $${GOOGLE_OAUTH_ACCESS_TOKEN:?}" \
        --header 'Content-Type: application/json' --data '{"streamEnabled":false}' \
        "https://analyticsadmin.googleapis.com/v1alpha/$GA_STREAM/enhancedMeasurementSettings?updateMask=stream_enabled"
    SH
  }
}
