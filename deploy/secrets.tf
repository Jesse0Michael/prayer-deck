resource "random_password" "auth_secret" {
  length  = 64
  special = false
}

locals {
  secret_names = ["auth-secret-key", "postgres-password", "storage-secret-key"]

  # Looked up by name rather than iterated, because for_each rejects sensitive values.
  secret_values = {
    auth-secret-key    = random_password.auth_secret.result
    postgres-password  = neon_project.db.database_password
    storage-secret-key = sha256(cloudflare_account_token.r2_s3.value)
  }
}

resource "google_secret_manager_secret" "api" {
  for_each  = toset(local.secret_names)
  secret_id = "prayer-deck-${each.key}"

  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_version" "api" {
  for_each    = toset(local.secret_names)
  secret      = google_secret_manager_secret.api[each.key].id
  secret_data = local.secret_values[each.key]
}

resource "google_secret_manager_secret_iam_member" "api_runtime" {
  for_each  = toset(local.secret_names)
  secret_id = google_secret_manager_secret.api[each.key].id
  role      = "roles/secretmanager.secretAccessor"
  member    = google_service_account.api.member
}
