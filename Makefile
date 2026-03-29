DOCKER_COMPOSE = docker compose
PROTO_DIR      = proto
BACKEND_GEN    = src/backend/gen
FRONTEND_GEN   = src/frontend/telegram/gen

.PHONY: proto
proto:
	protoc \
		--proto_path=$(PROTO_DIR) \
		--go_out=$(BACKEND_GEN) \
		--go_opt=paths=source_relative \
		--go-grpc_out=$(BACKEND_GEN) \
		--go-grpc_opt=paths=source_relative \
		$(PROTO_DIR)/debt/v1/debt.proto
	protoc \
		--proto_path=$(PROTO_DIR) \
		--go_out=$(FRONTEND_GEN) \
		"--go_opt=Mdebt/v1/debt.proto=github.com/mrralexandrov/debt-bot/frontend/telegram/gen/debt/v1;debtv1" \
		--go_opt=paths=source_relative \
		--go-grpc_out=$(FRONTEND_GEN) \
		"--go-grpc_opt=Mdebt/v1/debt.proto=github.com/mrralexandrov/debt-bot/frontend/telegram/gen/debt/v1;debtv1" \
		--go-grpc_opt=paths=source_relative \
		$(PROTO_DIR)/debt/v1/debt.proto

.PHONY: build
build:
	${DOCKER_COMPOSE} build

.PHONY: up
up:
	${DOCKER_COMPOSE} up -d --build

.PHONY: down
down:
	${DOCKER_COMPOSE} down

.PHONY: logs
logs:
	${DOCKER_COMPOSE} logs -f

.PHONY: migrate
migrate:
	${DOCKER_COMPOSE} exec backend sh -c 'echo "Migrations are applied automatically on backend startup"'

.PHONY: test
test:
	cd src/backend && go test ./...

.PHONY: coverage
coverage:
	cd src/backend && go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out
