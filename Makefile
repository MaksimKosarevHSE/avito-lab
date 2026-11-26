-include .env

HTTP_READ_TIMEOUT ?= 5s
HTTP_READ_HEADER_TIMEOUT ?= 2s
HTTP_WRITE_TIMEOUT ?= 10s
HTTP_IDLE_TIMEOUT ?= 60s
IDEMPOTENCY_TTL ?= 24h
IDEMPOTENCY_CLEANUP_INTERVAL ?= 1m
OTEL_METRIC_EXPORT_INTERVAL ?= 1000
POSITION_SCAN_BATCH_SIZE ?= 100

export HTTP_ADDR LOG_LEVEL SHUTDOWN_TIMEOUT
export HTTP_READ_TIMEOUT HTTP_READ_HEADER_TIMEOUT HTTP_WRITE_TIMEOUT HTTP_IDLE_TIMEOUT
export IDEMPOTENCY_TTL IDEMPOTENCY_CLEANUP_INTERVAL
export DATABASE_URL DATABASE_MAX_CONNS DATABASE_MIN_CONNS
export DATABASE_MAX_CONN_LIFETIME DATABASE_CONNECT_TIMEOUT DATABASE_QUERY_TIMEOUT
export POSITION_CHECK_INTERVAL POSITION_STALE_AFTER POSITION_SCAN_BATCH_SIZE
export POSITION_WORKERS POSITION_QUEUE_SIZE PUSH_HTTP_URL PUSH_REQUEST_TIMEOUT
export OTEL_SERVICE_NAME OTEL_EXPORTER_OTLP_ENDPOINT
export OTEL_EXPORTER_OTLP_PROTOCOL OTEL_RESOURCE_ATTRIBUTES
export OTEL_TRACES_SAMPLER OTEL_METRIC_EXPORT_INTERVAL

.PHONY: generate migrate migrate-down migrate-status run lint test test-unit test-integration

generate:
	go generate ./internal/usecase/trip/... ./internal/http/trip/...
	go tool oapi-codegen \
		-config ./api/oapi-codegen.yaml \
		-o ./api/api.gen.go \
		./contracts/openapi/trip-service.openapi.yaml
	go tool oapi-codegen \
		-config ./api/push/oapi-codegen.yaml \
		-o ./api/push/api.gen.go \
		./contracts/openapi/push-service.openapi.yaml

migrate:
	go tool goose -dir ./migrations postgres "$$DATABASE_URL" up

migrate-down:
	go tool goose -dir ./migrations postgres "$$DATABASE_URL" down

migrate-status:
	go tool goose -dir ./migrations postgres "$$DATABASE_URL" status

run:
	go run ./cmd/trip-service

lint:
	go tool golangci-lint run ./...

test: test-unit test-integration

test-unit:
	go test -race -count=2 -cover ./...

test-integration:
	go test -race -count=2 -tags=integration -p=2 -timeout=5m ./...
