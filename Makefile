#################################################################################
# BUILD COMMANDS
#################################################################################
.PHONY: build
build: build-api build-web # Build all binaries into bin/.

.PHONY: build-api
build-api: # Build the API server.
	go build -o bin/prayer-deck-api ./cmd/server

.PHONY: build-web
build-web: # Build the web client as a Wails server-mode binary.
	cd app/frontend && npm ci && npm run build
	cd app && go build -tags server -o ../bin/prayer-deck-web .

.PHONY: tools
tools: # Install the Wails CLI into bin/.
	GOBIN=$(CURDIR)/bin go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0

.PHONY: generate
generate: # Regenerate ent code, mocks, frontend API types, and Wails bindings.
	go generate ./...
	cd app/frontend && npm run generate:api
	cd app && ../bin/wails3 generate bindings -ts

#################################################################################
# RUN COMMANDS
#################################################################################
.PHONY: up
up: # Start the full local stack (Postgres, MinIO, API, web) in Docker.
	docker compose -p prayer-deck up --build -d

.PHONY: down
down: # Stop the local stack and remove its volumes.
	docker compose -p prayer-deck down -v

.PHONY: logs
logs: # Tail the local stack logs.
	docker compose -p prayer-deck logs -f

.PHONY: run
run: # Run the API locally against the Docker Postgres and MinIO. Loads .env if present.
	docker compose -p prayer-deck up -d postgres minio
	set -a; if [ -f .env ]; then . ./.env; fi; set +a; \
	AUTH_SECRET_KEY=$${AUTH_SECRET_KEY:-dev-secret-change-me} \
	DEV_LOGIN=$${DEV_LOGIN:-true} \
	LOG_FORMAT=TEXT \
	ALLOWED_ORIGINS=http://localhost:3000,http://localhost:9245,http://127.0.0.1:9245 \
	POSTGRES_PORT=$${POSTGRES_HOST_PORT:-5434} \
	POSTGRES_USER=prayerdeck POSTGRES_PASSWORD=prayerdeck POSTGRES_DB=prayerdeck POSTGRES_SSLMODE=disable \
	STORAGE_ACCESS_KEY=prayerdeck STORAGE_SECRET_KEY=prayerdeck-secret STORAGE_CREATE_BUCKET=true \
	go run ./cmd/server

.PHONY: dev-web
dev-web: # Run the frontend with hot reload at http://127.0.0.1:9245 against the API on :8080.
	cd app/frontend && npm run dev

#################################################################################
# TEST / LINT COMMANDS
#################################################################################
.PHONY: tidy
tidy: # Sync go.mod / go.sum for both modules.
	go mod tidy
	cd app && go mod tidy

.PHONY: lint
lint: # Run the Go linters, vulnerability checker, and frontend typecheck.
	go tool golangci-lint run ./...
	go tool govulncheck ./...
	cd app/frontend && npm run typecheck

.PHONY: test
test: # Run the tests.
	go test -cover ./... -timeout 120s

#################################################################################
# DEPLOY COMMANDS
#################################################################################
.PHONY: tf-fmt
tf-fmt: # Format the Terraform configuration.
	terraform fmt -recursive deploy

.PHONY: tf-plan
tf-plan: # Plan the production infrastructure. Needs deploy/terraform.tfvars and TF_STATE_BUCKET.
	cd deploy && terraform init -backend-config="bucket=$(TF_STATE_BUCKET)" && terraform plan
