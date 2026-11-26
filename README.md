# Trip Service

Сервис создаёт и завершает поездки, сохраняет координаты водителей и возвращает
пройденный маршрут. Если активная поездка давно не присылала координаты, сервис
запрашивает их через Push Service. Данные хранятся в PostgreSQL. Ошибки API
возвращаются в формате RFC 9457 `application/problem+json`.

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
| `POST` | `/api/v1/trips/{tripId}/positions` | сохранить координату |
| `GET` | `/api/v1/trips/{tripId}/positions` | получить маршрут поездки |
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
make test-unit
make test-integration
```

`make test` последовательно запускает unit- и интеграционные тесты. Оба набора
проверяются с race detector. Интеграционные тесты имеют build tag `integration`
и поднимают отдельный PostgreSQL через Testcontainers, поэтому для них нужен
запущенный Docker. Отдельно наборы запускаются командами `make test-unit` и
`make test-integration`.

## Конфигурация

Все настройки приходят из переменных окружения. Полный пример находится в
`.env.example`.

| Группа | Переменные |
|---|---|
| HTTP | `HTTP_ADDR`, `HTTP_READ_TIMEOUT`, `HTTP_READ_HEADER_TIMEOUT`, `HTTP_WRITE_TIMEOUT`, `HTTP_IDLE_TIMEOUT` |
| Приложение | `LOG_LEVEL`, `SHUTDOWN_TIMEOUT`, `IDEMPOTENCY_TTL`, `IDEMPOTENCY_CLEANUP_INTERVAL` |
| PostgreSQL | `DATABASE_URL`, `DATABASE_MAX_CONNS`, `DATABASE_MIN_CONNS`, `DATABASE_MAX_CONN_LIFETIME`, `DATABASE_CONNECT_TIMEOUT`, `DATABASE_QUERY_TIMEOUT` |
| Опрос координат | `POSITION_CHECK_INTERVAL`, `POSITION_STALE_AFTER`, `POSITION_SCAN_BATCH_SIZE`, `POSITION_WORKERS`, `POSITION_QUEUE_SIZE` |
| Push Service | `PUSH_HTTP_URL`, `PUSH_REQUEST_TIMEOUT` |
| OpenTelemetry | `OTEL_SERVICE_NAME`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_EXPORTER_OTLP_PROTOCOL`, `OTEL_RESOURCE_ATTRIBUTES`, `OTEL_TRACES_SAMPLER`, `OTEL_METRIC_EXPORT_INTERVAL` |

## Архитектура

Зависимости направлены от внешних слоёв к внутренним:

```text
cmd/trip-service → container
                       ├── HTTP → usecase → domain
                       ├── worker/positions → gateway/push
                       └── repository/postgres → infra/postgres
```

- `internal/domain` содержит модели, доменные правила и ошибки. Этот слой не
  зависит от HTTP, PostgreSQL и OpenTelemetry.
- `internal/usecase/<resource>/<action>` — слой сценариев приложения; отдельные
  пакеты отвечают за операции с поездками и координатами.
- `internal/http/<resource>/<action>` разбирает HTTP-запросы, преобразует DTO и
  вызывает только usecase-слой.
- `internal/worker/positions` содержит сканер, приоритетную очередь и пул
  воркеров. HTTP-клиент Push Service находится отдельно в
  `internal/gateway/push`.
- `internal/repository/postgres` содержит репозитории поездок и координат, а
  `internal/infra/postgres` отвечает за пул соединений, транзакции и
  инструментацию `pgx`.
- `internal/container` — composition root: только здесь создаются конкретные
  реализации и связываются зависимости. `cmd/trip-service` загружает
  конфигурацию, запускает приложение и выполняет graceful shutdown.

Интерфейсы внешних зависимостей объявлены на стороне потребителя в файлах
`contract.go`. Поэтому usecase-пакеты не импортируют PostgreSQL-репозиторий, а
HTTP-обработчики не зависят от конкретной реализации usecase. Каждый потребитель
описывает только нужный ему небольшой контракт; это упрощает замену реализаций и
генерацию изолированных моков для unit-тестов. Направление зависимостей
дополнительно проверяется `depguard` из `.golangci.yml`.

## Опрос устаревших поездок

Фоновый компонент запрашивает координаты по активным поездкам, которые давно не
обновлялись:

```text
PostgreSQL → сканер → приоритетная очередь → worker pool → Push Service
                            ↑
                новая координата отменяет задачу
```

Раз в `POSITION_CHECK_INTERVAL` сканер выбирает активные поездки, у которых с
последней координаты прошло больше `POSITION_STALE_AFTER`. Если координат ещё не
было, время отсчитывается от начала поездки. Выборка идёт батчами по
`POSITION_SCAN_BATCH_SIZE`, начиная с поездок, которые молчат дольше остальных.

Очередь ограничена `POSITION_QUEUE_SIZE` и не допускает две задачи для одной
поездки. Воркеры в количестве `POSITION_WORKERS` забирают самые старые задачи и
отправляют в Push Service команду `REQUEST_POSITION`. Каждый запрос ограничен
`PUSH_REQUEST_TIMEOUT`. Ошибка Push Service записывается в лог и метрики, но не
останавливает воркер.

При заполнении очереди новая задача сравнивается с самой свежей в очереди. Если
новая поездка молчит дольше, она занимает её место; иначе новая задача
отбрасывается. В обоих случаях дроп учитывается в метриках и сопровождается
`Warn`-логом. Сама поездка не теряется: её состояние в PostgreSQL не меняется, и
на следующем тике сканер выберет её снова.

Когда приходит новая координата, ещё не начатая задача удаляется из очереди и
Push-запрос по ней не отправляется. При остановке сервиса сначала прекращаются
HTTP-запросы и сканер, затем воркеры дорабатывают очередь в пределах общего
`SHUTDOWN_TIMEOUT`. После этого закрываются HTTP-клиент и пул PostgreSQL.

| Переменная | Значение по умолчанию | Назначение |
|---|---:|---|
| `POSITION_CHECK_INTERVAL` | `30s` | период запуска сканера |
| `POSITION_STALE_AFTER` | `60s` | сколько поездка может оставаться без координат |
| `POSITION_SCAN_BATCH_SIZE` | `100` | размер одной порции выборки из PostgreSQL |
| `POSITION_WORKERS` | `4` | количество Push-воркеров |
| `POSITION_QUEUE_SIZE` | `100` | максимальное число задач в очереди |
| `PUSH_HTTP_URL` | задаётся окружением | адрес Push Service |
| `PUSH_REQUEST_TIMEOUT` | `1s` | таймаут одного Push-запроса |

## Обсервабилити

Сервис отправляет трейсы и метрики по OTLP/HTTP в OpenTelemetry Collector, а
JSON-логи пишет в stdout. У каждой записи есть `service`, `trace_id` и
`span_id`, поэтому запрос можно связать с трейсом в Jaeger.

Путь одного запроса выглядит так:

```text
клиент → HTTP middleware → handler → usecase → repository → PostgreSQL
             │                                      │
             └──────── общий context и trace ───────┘
```

Для фонового опроса экспортируются размер очереди, число занятых воркеров,
результаты постановки задач и Push-запросов, длительность запросов и число строк,
прочитанных сканером. Обработка задачи воркером создаёт отдельный span, связанный
со span сканирования через span link.
