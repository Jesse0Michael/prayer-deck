terraform {
  required_version = ">= 1.9"

  # The bucket is supplied at init time: terraform init -backend-config="bucket=<name>".
  backend "gcs" {
    prefix = "prayer-deck"
  }

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 5.0"
    }
    neon = {
      source  = "kislerdm/neon"
      version = "~> 0.9"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}

# Credentials come from CLOUDFLARE_API_TOKEN and NEON_API_KEY in the environment.
provider "cloudflare" {}

provider "neon" {}
