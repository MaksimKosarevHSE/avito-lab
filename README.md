# Trip Service

Сервис создаёт поездки, возвращает их по ID и завершает. Данные хранятся в
PostgreSQL. Ошибки API возвращаются в формате RFC 9457
`application/problem+json`.

## Требования
  
- Go 1.25+
- Docker
- GNU Make
- `tripgoctl`

Инструменты для генерации кода, миграций и линтинга закреплены в `go.mod` и
запускаются через `go tool`, отдельно устанавливать их не нужно.

## Запуск

```bash
go mod download
tripgoctl cluster start
tripgoctl environment start
tripgoctl connect
make generate
make migrate
make run
```

`tripgoctl cluster start` достаточно выполнить один раз. Команда
`tripgoctl environment start` поднимает PostgreSQL и observability-окружение,
а `tripgoctl connect` держит локальные port-forward и выводит адреса Grafana и
Jaeger. По умолчанию сервис доступен на `localhost:8080`.

Проверка состояния:

```bash
curl -i http://localhost:8080/health
curl -i http://localhost:8080/ready
```

`/health` показывает, что процесс работает. `/ready` также проверяет PostgreSQL
и возвращает `503`, если база недоступна.

## HTTP API

| Метод | Путь | Назначение |
|---|---|---|
| `POST` | `/api/v1/trips` | создать поездку |
| `GET` | `/api/v1/trips/{tripId}` | получить поездку |
| `POST` | `/api/v1/trips/{tripId}/finish` | завершить поездку |
| `GET` | `/health` | liveness probe |
| `GET` | `/ready` | readiness probe |

## Команды

```bash
make generate
make migrate
make migrate-down
make migrate-status
make run
make lint
make test
make test-integration
```

`make test` запускает unit-тесты с race detector и печатает покрытие каждого
пакета. Интеграционные тесты имеют build tag `integration`, используют
Testcontainers и запускаются отдельно через `make test-integration`.

## Конфигурация

Все настройки приходят из переменных окружения. Полный пример находится в
`.env.example`.

| Группа | Переменные |
|---|---|
| HTTP | `HTTP_ADDR`, `HTTP_READ_TIMEOUT`, `HTTP_READ_HEADER_TIMEOUT`, `HTTP_WRITE_TIMEOUT`, `HTTP_IDLE_TIMEOUT` |
| Приложение | `LOG_LEVEL`, `SHUTDOWN_TIMEOUT`, `IDEMPOTENCY_TTL`, `IDEMPOTENCY_CLEANUP_INTERVAL` |
| PostgreSQL | `DATABASE_URL`, `DATABASE_MAX_CONNS`, `DATABASE_MIN_CONNS`, `DATABASE_MAX_CONN_LIFETIME`, `DATABASE_CONNECT_TIMEOUT`, `DATABASE_QUERY_TIMEOUT` |
| OpenTelemetry | `OTEL_SERVICE_NAME`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_PROTOCOL`, `OTEL_RESOURCE_ATTRIBUTES`, `OTEL_TRACES_SAMPLER`, `OTEL_METRIC_EXPORT_INTERVAL` |

## Архитектура

Зависимости направлены от внешних слоёв к внутренним:

```text
cmd/trip-service → container
                       ├── HTTP → service → domain
                       └── repository → domain
                              └── infra/postgres
```

- `internal/domain` содержит модели, доменные правила и ошибки. Этот слой не
  зависит от HTTP, PostgreSQL и OpenTelemetry.
- `internal/service/trip/<action>` — слой сценариев приложения. Здесь `service`
  выполняет роль `usecase`; отдельные пакеты отвечают за создание, получение и
  завершение поездки.
- `internal/http/trip/<action>` разбирает HTTP-запросы, преобразует DTO и
  вызывает только сервисный слой.
- `internal/repository/postgres/trip` реализует хранение поездок, а
  `internal/infra/postgres` отвечает за пул соединений, транзакции и
  инструментацию `pgx`.
- `internal/container` — composition root: только здесь создаются конкретные
  реализации и связываются зависимости. `cmd/trip-service` загружает
  конфигурацию, запускает приложение и выполняет graceful shutdown.

Интерфейсы внешних зависимостей объявлены на стороне потребителя в файлах
`contract.go`. Поэтому сервисы не импортируют PostgreSQL-репозиторий, а
HTTP-обработчики не зависят от конкретной реализации сервиса. Каждый потребитель
описывает только нужный ему небольшой контракт; это упрощает замену реализаций и
генерацию изолированных моков для unit-тестов. Направление зависимостей
дополнительно проверяется `depguard` из `.golangci.yml`.

## Обсервабилити

Сервис отправляет трейсы и метрики по OTLP/HTTP в OpenTelemetry Collector, а
JSON-логи пишет в stdout. У каждой записи есть `service`, `trace_id` и
`span_id`, поэтому запрос можно связать с трейсом в Jaeger.

Путь одного запроса выглядит так:

```text
клиент → HTTP middleware → handler → service → repository → PostgreSQL
             │                                      │
             └──────── общий context и trace ───────┘
```
