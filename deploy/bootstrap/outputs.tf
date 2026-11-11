output "state_bucket" {
  value = google_storage_bucket.state.name
}

output "workload_identity_provider" {
  value = google_iam_workload_identity_pool_provider.github.name
}

output "deployer_service_account" {
  value = google_service_account.deployer.email
}

output "image_repository" {
  value = "${google_artifact_registry_repository.images.location}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.images.repository_id}"
}

# Paste into a shell with gh authenticated to set the repository variables CI reads.
output "github_variables" {
  value = <<-EOT
    gh variable set GCP_PROJECT_ID --body '${var.project_id}'
    gh variable set GCP_REGION --body '${var.region}'
    gh variable set TF_STATE_BUCKET --body '${google_storage_bucket.state.name}'
    gh variable set WIF_PROVIDER --body '${google_iam_workload_identity_pool_provider.github.name}'
    gh variable set DEPLOYER_SERVICE_ACCOUNT --body '${google_service_account.deployer.email}'
  EOT
}
