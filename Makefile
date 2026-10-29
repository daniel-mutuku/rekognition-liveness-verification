# Run `make` or `make help` to list targets.

BIN := build/server
TF  := terraform -chdir=infra

.DEFAULT_GOAL := help
.PHONY: help run test test-aws smoke vet fmt build tidy web-install web-dev web-build dev demo tf-plan tf-apply tf-output

help: ## List targets
	@awk 'BEGIN {FS = ":.*## "} /^[a-z-]+:.*## / {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

run: build ## Build and run the server (reads .env; Ctrl+C to stop)
	./$(BIN)

test: ## Run all tests (no AWS needed)
	go test -race ./...

test-aws: ## Run the store contract tests against the real DynamoDB table (uses .env)
	set -a && . ./.env && set +a && DYNAMO_TEST_TABLE=$$VERIFICATIONS_TABLE go test -race -run TestDynamo -v ./internal/store/

PHOTO ?= testdata/face.jpg
smoke: ## Walk the API with curl up to the camera step (server must be running; PHOTO=...)
	scripts/smoke.sh $(PHOTO)

vet: ## Static checks with go vet
	go vet ./...

fmt: ## Format all Go code
	go fmt ./...

build: ## Compile the server to build/server
	go build -o $(BIN) ./cmd/server

tidy: ## Sync go.mod/go.sum with the imports
	go mod tidy

web-install: ## Install the web app's exact dependencies (from package-lock.json)
	cd web && npm ci

web-dev: ## Run only the Vite dev server on :5173 (API calls proxy to :8080)
	cd web && npm run dev

web-build: ## Type-check and build the web app into web/dist
	cd web && npm run build

dev: build ## Go API + Vite together; open http://localhost:5173 (Ctrl+C stops both)
	scripts/dev.sh

demo: web-build build ## Build everything and serve app + API from Go; open http://localhost:8080
	./$(BIN)

tf-plan: ## Terraform plan, saved to infra/tfplan for review
	$(TF) plan -out=tfplan

tf-apply: ## Apply exactly the plan from tf-plan
	$(TF) apply tfplan

tf-output: ## Show Terraform outputs (incl. env_file and aws_config_snippet)
	$(TF) output
