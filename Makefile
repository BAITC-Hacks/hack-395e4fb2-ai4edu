# The server reads only the process environment, so `make run` exports .env when present.
-include .env
export

PORT ?= $(or $(lastword $(subst :, ,$(ADDR))),8080)
BIN  := bin/server

GOLDEN := {"decisions":[{"measure_id":"M7","district_id":"nura"},{"measure_id":"M8","district_id":"nura"},{"measure_id":"M10","district_id":"nura"},{"measure_id":"M12"},{"measure_id":"M5","district_id":"saryarka"}]}

.DEFAULT_GOAL := help
.PHONY: help run build test race vet fmt fmt-check cover check up down logs demo demo-explain clean

help: ## Show available targets
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "  %-14s %s\n", $$1, $$2}'

run: ## Run the API locally (loads .env if present)
	go run ./cmd/server

build: ## Build the server binary into bin/
	CGO_ENABLED=0 go build -trimpath -o $(BIN) ./cmd/server

test: ## Run all tests
	go test ./...

race: ## Run tests with the race detector
	go test -race ./...

vet: ## Run go vet
	go vet ./...

fmt: ## Format all Go files
	gofmt -w .

fmt-check: ## Fail if any Go file is not gofmt-formatted
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "Not formatted:"; echo "$$out"; exit 1; fi

cover: ## Show test coverage per package
	go test -cover ./...

check: fmt-check vet test race ## Run every check before a demo or commit

up: ## Build and start the API in Docker
	docker compose up --build -d

down: ## Stop the Docker stack
	docker compose down

logs: ## Follow API container logs
	docker compose logs -f api

demo: ## Send the golden scenario to /api/simulate (server must be running)
	@curl -sS localhost:$(PORT)/api/simulate -H 'Content-Type: application/json' -d '$(GOLDEN)'; echo

demo-explain: ## Send the golden scenario to /api/explain (server must be running)
	@curl -sS localhost:$(PORT)/api/explain -H 'Content-Type: application/json' -d '$(GOLDEN)'; echo

clean: ## Remove build output
	rm -rf bin
