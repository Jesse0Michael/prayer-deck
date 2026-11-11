# Deploying

Everything runs on free tiers and is managed by Terraform in [`deploy/`](../deploy).

| Piece | Service | Managed by |
| --- | --- | --- |
| Web client (static build) | Cloudflare Pages at `https://prayer-deck.pages.dev`, or `https://<app_subdomain>.<domain>` when `domain` is set | Terraform creates the project, domain, and DNS; CI uploads the build |
| API | Google Cloud Run (scale to zero, one instance) | Terraform creates the service; CI deploys images |
| Database | Neon Postgres | Terraform |
| Artwork | Cloudflare R2, private bucket, presigned URLs | Terraform |
| Runtime secrets | Google Secret Manager | Terraform generates and stores them |

Secrets never live in the repo. CI reads them from GitHub Actions secrets,
the API reads them from Secret Manager, and Terraform state (which holds them)
lives in a private, versioned GCS bucket.

## One-time setup

1. **Accounts.** A Google Cloud project with billing enabled, a Neon account, and
   a Cloudflare account. A domain is optional; without one the app is served from
   its free `pages.dev` address.
2. **Google sign-in client.** In the Google Cloud console, create an OAuth
   client of type *Web application*. Add `https://<app_subdomain>.<domain>` as an
   authorized JavaScript origin (`https://prayer-deck.pages.dev` without a custom
   domain; the `app_url` Terraform output shows the exact value). Google does not let Terraform create this
   client. Keep the client ID.
3. **Cloudflare API token.** Create an account-owned token under *Manage Account >
   Account API Tokens*, scoped to your account, with:
   - Account: *Cloudflare Pages: Edit*, *Workers R2 Storage: Edit*,
     *Account API Tokens: Edit* (lets Terraform create the R2 access key)
   - Zone: *DNS: Edit*, *Zone: Read* (limited to your domain; only needed with a
     custom domain)

   CI uses this token for Terraform and for publishing to Pages. The API server
   never sees it; it runs with its own bucket-scoped R2 key.
4. **Neon API key** from the Neon console (Account settings, API keys).
5. **Bootstrap** from your machine, signed in with `gcloud auth application-default login`:

   ```bash
   cd deploy/bootstrap
   cp terraform.tfvars.example terraform.tfvars   # edit
   terraform init && terraform apply
   terraform output -raw github_variables         # prints gh commands; run them
   ```

   This creates the state bucket, image registry, the keyless GitHub identity,
   and a $1 budget alert email. Its own state stays local and is gitignored.
   Keep it: it's small and rarely changes.
6. **Repository settings.** Set the remaining variables and secrets:

   ```bash
   gh variable set DOMAIN --body example.com     # optional; skip to use pages.dev
   gh variable set CLOUDFLARE_ACCOUNT_ID --body <id>
   gh variable set GOOGLE_CLIENT_ID --body <client id>
   gh variable set NEON_ORG_ID --body <org id>   # skip for a personal Neon account
   gh secret set CLOUDFLARE_API_TOKEN
   gh secret set NEON_API_KEY
   ```

7. **First run.** Run the *Terraform* workflow (Actions tab, *Run workflow*).
   It creates everything with a placeholder API image. Then run *Deploy* the same
   way to ship the real API and web build.

After that, merging to `main` deploys automatically:

- Changes under `deploy/` run `terraform plan`/`apply` (the plan is on PRs too).
- Any merge that passes the test workflow builds the API image, deploys it, and
  publishes the web build with the live API URL baked in.

## Local Terraform

```bash
cd deploy
terraform init -backend-config="bucket=<project>-tfstate"
cp terraform.tfvars.example terraform.tfvars   # edit
export CLOUDFLARE_API_TOKEN=... NEON_API_KEY=...
terraform plan
```

## Things to know

- **Cold starts.** Cloud Run and Neon both scale to zero, so the first request
  after idle takes a few seconds.
- **API URL.** The API is served at its `run.app` address, shown in the
  `api_url` output. A custom `api.` domain can be added later with a Cloud Run
  domain mapping.
- **Migrations.** The API runs `Schema.Create` at startup. See
  [decisions](decisions.md) before real data accumulates.
- **Backups.** Neon's free tier keeps a short history window. Take occasional
  `pg_dump` backups of anything you can't recreate.
- **Cost guardrails.** Cloud Run is capped at one instance, the registry prunes
  old images, and the budget alert emails billing admins; it does not stop spend.
