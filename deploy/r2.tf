resource "cloudflare_r2_bucket" "artwork" {
  account_id = var.cloudflare_account_id
  name       = "prayer-deck-artwork"
  location   = var.r2_location
}

# The bucket stays private; the API hands the browser presigned URLs.
resource "cloudflare_r2_bucket_cors" "artwork" {
  account_id  = var.cloudflare_account_id
  bucket_name = cloudflare_r2_bucket.artwork.name

  rules = [{
    allowed = {
      methods = ["GET", "HEAD"]
      origins = [local.app_origin]
    }
    max_age_seconds = 3600
  }]
}

data "cloudflare_account_api_token_permission_groups_list" "object_read" {
  account_id = var.cloudflare_account_id
  name       = "Workers R2 Storage Bucket Item Read"
}

data "cloudflare_account_api_token_permission_groups_list" "object_write" {
  account_id = var.cloudflare_account_id
  name       = "Workers R2 Storage Bucket Item Write"
}

# R2's S3 credentials are an API token: the access key ID is the token ID and
# the secret access key is the SHA-256 of the token value.
resource "cloudflare_account_token" "r2_s3" {
  account_id = var.cloudflare_account_id
  name       = "prayer-deck-api-artwork"

  policies = [{
    effect = "allow"
    permission_groups = [
      { id = data.cloudflare_account_api_token_permission_groups_list.object_read.result[0].id },
      { id = data.cloudflare_account_api_token_permission_groups_list.object_write.result[0].id },
    ]
    resources = jsonencode({
      "com.cloudflare.edge.r2.bucket.${var.cloudflare_account_id}_default_${cloudflare_r2_bucket.artwork.name}" = "*"
    })
  }]
}
