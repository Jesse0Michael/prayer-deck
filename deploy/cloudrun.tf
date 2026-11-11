resource "google_service_account" "api" {
  account_id   = "prayer-deck-api"
  display_name = "Prayer Deck API runtime"
}

resource "google_cloud_run_v2_service" "api" {
  name                = "prayer-deck-api"
  location            = var.region
  ingress             = "INGRESS_TRAFFIC_ALL"
  deletion_protection = false

  template {
    service_account = google_service_account.api.email

    # One instance keeps the free tier safe and is plenty for a personal deck.
    scaling {
      min_instance_count = 0
      max_instance_count = 1
    }

    containers {
      image = var.image

      ports {
        container_port = 8080
      }

      resources {
        limits = {
          cpu    = "1"
          memory = "512Mi"
        }
        cpu_idle = true
      }

      dynamic "env" {
        for_each = {
          LOG_FORMAT            = "JSON"
          ALLOWED_ORIGINS       = local.app_origin
          GOOGLE_CLIENT_ID      = var.google_client_id
          DEV_LOGIN             = "false"
          POSTGRES_HOST         = neon_project.db.database_host
          POSTGRES_USER         = neon_project.db.database_user
          POSTGRES_DB           = neon_project.db.database_name
          POSTGRES_SSLMODE      = "require"
          STORAGE_ENDPOINT      = "https://${var.cloudflare_account_id}.r2.cloudflarestorage.com"
          STORAGE_REGION        = "auto"
          STORAGE_BUCKET        = cloudflare_r2_bucket.artwork.name
          STORAGE_ACCESS_KEY    = cloudflare_account_token.r2_s3.id
          STORAGE_CREATE_BUCKET = "false"
        }
        content {
          name  = env.key
          value = env.value
        }
      }

      dynamic "env" {
        for_each = {
          AUTH_SECRET_KEY    = "auth-secret-key"
          POSTGRES_PASSWORD  = "postgres-password"
          STORAGE_SECRET_KEY = "storage-secret-key"
        }
        content {
          name = env.key
          value_source {
            secret_key_ref {
              secret  = google_secret_manager_secret.api[env.value].secret_id
              version = "latest"
            }
          }
        }
      }
    }
  }

  # CI deploys new images; Terraform owns everything else about the service.
  lifecycle {
    ignore_changes = [
      template[0].containers[0].image,
      client,
      client_version,
    ]
  }

  depends_on = [
    google_secret_manager_secret_version.api,
    google_secret_manager_secret_iam_member.api_runtime,
  ]
}

resource "google_cloud_run_v2_service_iam_member" "public" {
  name     = google_cloud_run_v2_service.api.name
  location = google_cloud_run_v2_service.api.location
  role     = "roles/run.invoker"
  member   = "allUsers"
}
