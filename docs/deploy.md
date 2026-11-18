# Deploying

Everything runs on free tiers and is managed by Terraform in [`deploy/`](../deploy).

| Piece | Service | Managed by |
| --- | --- | --- |
| Web client (static build) | Cloudflare Pages at `https://prayer-deck.pages.dev`, or `https://<app_subdomain>.<domain>` when `domain` is set | Terraform creates the project, domain, and DNS; CI uploads the build |
| Web analytics | Google Analytics 4 | Terraform creates the property, web stream, and reporting definitions. CI passes the stream's measurement ID and production hostname to Vite. |
| API | Google Cloud Run (scale to zero, one instance) | Terraform creates the service; CI deploys images |
| Database | Neon Postgres | Terraform |
| Artwork | Cloudflare R2, private bucket, presigned URLs | Terraform |
| Runtime secrets | Google Secret Manager | Terraform generates and stores them |

Secrets never live in the repo. CI reads them from GitHub Actions secrets,
the API reads them from Secret Manager, and Terraform state (which holds them)
lives in a private, versioned GCS bucket.

## Add analytics to an existing deployment

Use the workstation and checkout that hold the original `deploy/bootstrap/terraform.tfstate`
and `deploy/bootstrap/terraform.tfvars`. If either file is missing, recover it
from the original checkout or backup first. Check that `project_id`, region,
and any billing settings in the tfvars match the existing deployment.

```bash
cd deploy/bootstrap
terraform init && terraform apply
```

Review the plan before approving it. The analytics upgrade adds
`analyticsadmin.googleapis.com` and restricts the existing GitHub identity
provider to `main`. If Terraform proposes replacing the state bucket or changing
your existing budget, check the inputs before continuing. After apply succeeds,
continue with [Enable analytics and publish](#enable-analytics-and-publish).

## One-time setup

Follow these steps for a new deployment.

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
5. **Bootstrap.** Sign in on your machine with `gcloud auth application-default login`.
   Create the local inputs file.

   ```bash
   cd deploy/bootstrap
   cp -n terraform.tfvars.example terraform.tfvars
   ```

   Edit `terraform.tfvars` to supply the real Google Cloud project ID.
   Set `billing_account` to your billing account ID for the budget alert,
   or to `""` to skip the alert. Apply the configuration and print the repository settings.

   ```bash
   terraform init && terraform apply && terraform output -raw github_variables
   ```

   Run the printed `gh` commands to configure the repository. Bootstrap creates
   the state bucket, image registry, keyless GitHub identity, and optional
   budget alert. Keep its local state and tfvars together for future upgrades.
6. **Repository settings.** Set the remaining variables and secrets:

   ```bash
   gh variable set DOMAIN --body example.com     # optional; skip to use pages.dev
   gh variable set CLOUDFLARE_ACCOUNT_ID --body <id>
   gh variable set GOOGLE_CLIENT_ID --body <client id>
   gh variable set NEON_ORG_ID --body <org id>   # skip for a personal Neon account
   gh secret set CLOUDFLARE_API_TOKEN
   gh secret set NEON_API_KEY
   ```

## Enable analytics and publish

Terraform creates a property named **Prayer Deck**, its web stream, and its
reporting definitions when the main `deploy/` configuration is applied.
Bootstrap enables the Google Cloud API used for this work. A Google Analytics
account owns properties and controls access. One account can contain several
properties, each with its own web stream and reports.

1. Open Google Analytics and use the account/property selector to select the
   parent account. Go to *Admin → Account → Account details* and copy the numeric
   **Account ID**. In *Admin → Account → Account access management*, click
   **+ → Add users** and grant **Editor** to
   `github-deployer@prayer-deck.iam.gserviceaccount.com`.
   [Google's account navigation guide](https://support.google.com/analytics/answer/12813202)
2. From this repository, run `gh variable set GA_ACCOUNT_ID` and enter that
   numeric Account ID.
3. Run the *Terraform* workflow on `main`
   (Actions tab, *Run workflow*) and wait for Apply to succeed. Then run
   *Deploy* to publish the web build with the generated measurement ID.
   The account/property selector will show **Prayer Deck** after Terraform
   succeeds. From the repository root,
   `terraform -chdir=deploy output -raw analytics_property_id` prints its numeric
   Property ID. Open the live site and check the
   [GA4 rollout steps](analytics.md#verify-the-rollout).

After that, merging to `main` deploys automatically:

- Changes under `deploy/` run Terraform. Pull requests validate the configuration
  without production credentials and merges apply it.
- After the test workflow passes on `main`, Deploy publishes the API image
  and builds the web client with the live API URL and the latest applied
  analytics outputs. If a release runs before the first analytics apply
  finishes, rerun Deploy after Terraform succeeds. Also rebuild the web client
  after applying a change to its canonical hostname.

## Local Terraform

For a first local run, create the inputs file in `deploy/`.

```bash
cd deploy
cp -n terraform.tfvars.example terraform.tfvars
```

Fill in the deployment values, including `analytics_account_id`. For subsequent
runs, use the saved tfvars. Run Terraform from `deploy/`.

```bash
terraform init -backend-config="bucket=<project>-tfstate"
export CLOUDFLARE_API_TOKEN=... NEON_API_KEY=...
# If your workstation login can impersonate the deployer service account:
export GOOGLE_OAUTH_ACCESS_TOKEN="$(gcloud auth application-default print-access-token \
  --impersonate-service-account=github-deployer@YOUR_PROJECT.iam.gserviceaccount.com \
  --scopes=https://www.googleapis.com/auth/cloud-platform,https://www.googleapis.com/auth/analytics.edit)"
terraform plan
```

The local plan and apply use an identity with access to the state bucket, GCP
resources, and Analytics account. Refresh the token if it expires before
apply. CI obtains its short-lived token through Workload Identity Federation.

## Things to know

- **Storage configuration.** The API uses `pkg/config.S3Config` and
  `AWSConfig`. Set `AWS_ENDPOINT_URL_S3`, `S3_REGION`, `S3_BUCKET`,
  `AWS_ACCESS_KEY_ID`, and `AWS_SECRET_ACCESS_KEY`; set `S3_PUBLIC_ENDPOINT`
  when browsers use a different endpoint from the API (as in Docker).
  These replace `STORAGE_ENDPOINT`, `STORAGE_REGION`, `STORAGE_BUCKET`,
  `STORAGE_ACCESS_KEY`, `STORAGE_SECRET_KEY`, and `STORAGE_PUBLIC_ENDPOINT`,
  respectively. `STORAGE_URL_EXPIRY` (default `6h`, maximum `168h`) and
  `STORAGE_CREATE_BUCKET` remain app settings. Docker and `make run` supply
  the local MinIO settings explicitly.
  The API accepts the legacy storage variables as fallback values, and explicit
  AWS/S3 configuration takes precedence. Terraform supplies both sets of
  names so the infrastructure and API image can deploy in either order.

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
