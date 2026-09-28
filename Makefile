-include .env

HTTP_READ_TIMEOUT ?= 5s
HTTP_READ_HEADER_TIMEOUT ?= 2s
HTTP_WRITE_TIMEOUT ?= 10s
HTTP_IDLE_TIMEOUT ?= 60s
IDEMPOTENCY_TTL ?= 24h

export HTTP_ADDR LOG_LEVEL SHUTDOWN_TIMEOUT
export HTTP_READ_TIMEOUT HTTP_READ_HEADER_TIMEOUT HTTP_WRITE_TIMEOUT HTTP_IDLE_TIMEOUT
export IDEMPOTENCY_TTL
export DATABASE_URL DATABASE_MAX_CONNS DATABASE_MIN_CONNS
export DATABASE_MAX_CONN_LIFETIME DATABASE_CONNECT_TIMEOUT DATABASE_QUERY_TIMEOUT

.PHONY: generate migrate migrate-down migrate-status run

generate:
	go tool oapi-codegen \
		-config ./api/oapi-codegen.yaml \
		-o ./api/api.gen.go \
		./contracts/openapi/trip-service.openapi.yaml

migrate:
	go tool goose -dir ./migrations postgres "$$DATABASE_URL" up

migrate-down:
	go tool goose -dir ./migrations postgres "$$DATABASE_URL" down

migrate-status:
	go tool goose -dir ./migrations postgres "$$DATABASE_URL" status

run:
	go run ./cmd/trip-service
