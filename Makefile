# Default environment - override per call with ENV=stage / ENV=prod.
# (GNU make rejects unknown `--dev`-style flags before reading this file,
# so the environment is a plain variable.)
ENV ?= dev

BIN := bin/fairchild-admin-panel-be

#Targets
.PHONY: run-dev run-staging run-prod build-dev build-stage build-prod run build migrate-up migrate-down migrate-up-dev migrate-down-dev migrate-up-stage migrate-down-stage migrate-up-prod create-panel-user seed-members test

run-dev:
	$(MAKE) run ENV=dev

run-staging:
	$(MAKE) run ENV=stage

run-prod:
	$(MAKE) run ENV=prod

build-dev:
	$(MAKE) build ENV=dev

build-stage:
	$(MAKE) build ENV=stage

build-prod:
	$(MAKE) build ENV=prod

run: build
	@$(BIN)

build:
	@go build -ldflags "-X 'main.BuildEnv=${ENV}'" -o $(BIN) cmd/main.go

migrate-up-dev:
	$(MAKE) migrate-up ENV=dev

migrate-down-dev:
	$(MAKE) migrate-down ENV=dev

migrate-up-stage:
	$(MAKE) migrate-up ENV=stage

migrate-down-stage:
	$(MAKE) migrate-down ENV=stage

# No migrate-down-prod on purpose - `down` rolls back every migration
migrate-up-prod:
	$(MAKE) migrate-up ENV=prod

migrate-up:
	@go run -ldflags "-X 'main.BuildEnv=${ENV}'" cmd/migrate/main.go up

migrate-down:
	@go run -ldflags "-X 'main.BuildEnv=${ENV}'" cmd/migrate/main.go down

# make create-panel-user ENV=dev ARGS="-username jdelacruz -first-name Juan -last-name Cruz -role admin"
create-panel-user:
	@go run -ldflags "-X 'main.BuildEnv=${ENV}'" cmd/create_panel_user/main.go $(ARGS)

# Sample rows for an empty admin_members table (dev only; refuses if it has rows)
seed-members:
	@go run -ldflags "-X 'main.BuildEnv=${ENV}'" cmd/seed_members/main.go

test:
	@go test ./...

## Usage:
# make run-dev
# make migrate-up-dev
# make create-panel-user ENV=dev ARGS="-username admin -first-name First -last-name Admin -role admin"
