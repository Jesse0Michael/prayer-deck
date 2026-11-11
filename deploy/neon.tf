# Free-plan Postgres that suspends when idle; the first query after a pause is slow.
resource "neon_project" "db" {
  name       = "prayer-deck"
  org_id     = var.neon_org_id
  region_id  = var.neon_region
  pg_version = 17

  branch {
    name          = "main"
    database_name = "prayerdeck"
    role_name     = "prayerdeck"
  }
}
