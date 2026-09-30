# Наблюдаемость и интеграционные проверки RMM

## Readiness и остановка

- `GET /healthz` сохраняет прежний контракт: `200` при доступной БД, `503` при ошибке.
- `GET /readyz` возвращает `200` после startup-проверок схемы и ключей, если БД доступна. При начале остановки возвращает `503`. Проверка соединения ограничена двумя секундами.
- `SIGINT` и `SIGTERM` снимают readiness, отменяют фоновые задачи и закрывают SSE. Сервер даёт до 30 секунд на завершение HTTP-запросов и фоновых задач, затем закрывает БД. Клиенты SSE могут подключиться к новому экземпляру после перезапуска.
- Отменённая доставка остаётся в состоянии `sending`; обычное восстановление lease позволяет обработать её снова. Доставка через внешние каналы сохраняет семантику at-least-once: после неоднозначного сетевого результата возможен повтор.

Миграции проверяются до запуска HTTP-listener. `/readyz` не запускает миграции повторно и не является проверкой состояния внешнего SSH-сервиса. Compose даёт `rmm-server` 40 секунд (`stop_grace_period`), чтобы Docker не прерывал 30-секундное завершение раньше времени.

## Prometheus

`GET /metrics` требует действующий токен `RMM_OPERATOR_TOKEN` bootstrap-администратора или сессию администратора. Неавторизованный запрос получает `401`, пользователь без роли admin — `403`. Для Prometheus настройте `RMM_OPERATOR_TOKEN` в защищённых переменных сервера и положите то же значение в защищённый файл Prometheus вне Git. Этот токен даёт административный API-доступ, поэтому доступ к файлу должен иметь только Prometheus.

Пример конфигурации Prometheus для внутреннего HTTPS-адреса RMM:

```yaml
scrape_configs:
  - job_name: rmm
    scheme: https
    metrics_path: /metrics
    scrape_interval: 30s
    authorization:
      type: Bearer
      credentials_file: /run/secrets/rmm-metrics-token
    static_configs:
      - targets: [rmm.example.com]
```

Проверяйте конфигурацию штатным `promtool check config` в окружении Prometheus. TLS-проверка остаётся включённой. Новый порт для метрик не требуется.

Метрики:

| Имя | Значение |
| --- | --- |
| `rmm_database_info{backend}` | Выбранная БД: sqlite/postgres |
| `rmm_database_connections_open/idle/in_use` | Состояние пула соединений |
| `rmm_database_wait_total` | Число ожиданий соединения с момента запуска |
| `rmm_database_wait_seconds_total` | Суммарная длительность ожиданий |
| `rmm_commands{status}` | Число сохранённых команд по статусам |
| `rmm_notification_deliveries{status}` | Число сохранённых доставок по статусам |
| `rmm_notification_queue_oldest_age_seconds` | Возраст самой старой незавершённой доставки |
| `rmm_tunnel_sessions` | Неистёкшие requested/queued/active сессии |

Статусы — gauges текущих записей, а не накопительные счётчики событий: retention уменьшает их значения. Идентификаторы устройств, destinations, сообщения, credentials и DSN не публикуются. При ошибке сбора endpoint возвращает `503`, а не недостоверные нули. Сбор выполняет агрегирующие запросы; его нагрузку на очень большие базы нужно измерить перед увеличением частоты scrape.

## PostgreSQL CI

Workflow `.github/workflows/postgres.yml` запускает PostgreSQL 16 и 18 на отдельных runners и создаёт пять пустых БД: для импорта, новой схемы, конкурентных сценариев и пары source/restore. Пароль в workflow — только одноразовая учётная запись изолированного CI, не production credential.

Для локального запуска используйте три **пустые тестовые БД** на loopback-адресе и задайте `RMM_TEST_POSTGRES_URL`, `RMM_TEST_POSTGRES_FRESH_URL`, `RMM_TEST_POSTGRES_CRITICAL_URL` с `sslmode=disable`. Не используйте рабочую БД. Тесты оставляют тестовые записи, поэтому повторный запуск требует новых пустых БД.

```sh
go test -race -count=1 -v ./server/internal/dbmigrate
```

Без переменных PostgreSQL-интеграционные тесты явно пропускаются. CI задаёт все три URL. Проверяются импорт и ключи, schema/checksum guards, подпись команды после round trip, heartbeat, NULL `last_seen`, конкуренция двух независимых connection pools, recovery notification lease и одноразовый enrollment.

Реальные dump/restore, совпадение хешей всех таблиц, восстановленные login/heartbeat и отдельный recovery-скрипт описаны в [инструкции проверки восстановления PostgreSQL](postgres-recovery-drill.md).

## Reverse SSH / cloud LuCI E2E

На Linux с Go, Docker и OpenSSH client:

```sh
bash scripts/test-remote-access.sh
```

Workflow `.github/workflows/remote-access.yml` выполняет эту же команду. Скрипт собирает настоящий агент и production SSH-образ, создаёт один тестовый контейнер с `/data` в tmpfs, а затем удаляет только созданный им контейнер и временный агент. Свободными должны быть тестовые порты `18085`, `18086`, `22055`, `22155`; production-порты и тома не меняются. API теста слушает все интерфейсы на `18085`, чтобы SSH-контейнер мог обратиться к нему; запускать на изолированной машине/runner.

Тест использует реальный heartbeat, подписанную команду и SSH-клиент агента. LuCI и локальный SSH banner имитируются; аппаратная OpenWrt-проверка остаётся отдельным release gate. HTTPS проходит с проверкой тестового wildcard-сертификата и SNI; публичный DNS не требуется. Проверяются также ошибка upstream (502), последующее восстановление, отсутствие RMM-cookie у LuCI и отказ TLS для имени вне сертификата.

Проверяются успешный доступ, неправильный device host, повторное использование grant, немедленный запрет cookie после expiry/revoke, закрытие SSH-listener в пределах 12 секунд и отказ реального sshd принимать отозванный ключ. Истечение ускоряется изменением срока только в временной тестовой SQLite. Тестовый таймер агента — 60 секунд, поэтому он не может скрыть ошибку reaper.

## Исправление tunnel reaper

OpenSSH 9.8+ использует `sshd-session`, что учтено в reaper. Для сопоставления listener и PID через `/proc` SSH-контейнеру в `compose.yaml` и `compose.postgres.yaml` добавлен `SYS_PTRACE`. Это расширяет доступ к процессам **внутри этого контейнера**. Нельзя совместно включать host PID namespace или privileged. Сервер RMM этой capability не получает.

Перед обновлением сохраните прежнюю Compose-конфигурацию и точный image tag. Проверьте `docker compose config --quiet`, затем после обновления выполните smoke создания/закрытия/expiry SSH и LuCI-сессий. Для отката восстановите предыдущие Compose и image tag; удаление `SYS_PTRACE` возвращает прежнюю конфигурацию, но вновь лишает reaper доступа к владельцам сокетов в обычном Docker-контейнере. Изменение затрагивает только закрытие истёкших/отозванных reverse tunnels; LAN/WAN/firewall роутеров не меняются.
