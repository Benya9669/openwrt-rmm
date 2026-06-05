# OpenWrt RMM Checklist

Практический чеклист разработки. Он дополняет `ROADMAP.md`: roadmap описывает направление, а этот файл фиксирует, что уже сделано, что делаем следующим и что считается готовностью этапов.

## Статусы

- `[x]` - сделано и проверено.
- `[~]` - частично сделано, требует доработки.
- `[ ]` - не начато.

## Текущий фокус

- [x] Первый вертикальный сценарий: enrollment -> heartbeat -> command queue -> command result.
- [x] Сервер на Go.
- [x] SQLite storage для MVP.
- [x] Shell-agent для OpenWrt.
- [x] HTTP polling вместо постоянного соединения.
- [x] Operator bearer token для MVP.
- [x] Command lifecycle: timeout, retry, history.
- [x] OpenWrt package skeleton.
- [x] Минимальный web UI.

## Этап 0: Подготовка проекта

- [x] Выбрать backend stack: Go.
- [x] Выбрать storage для MVP: SQLite.
- [x] Выбрать agent transport: outbound HTTP polling.
- [x] Выбрать agent MVP format: POSIX shell.
- [x] Создать базовую структуру репозитория.
- [x] Создать `ROADMAP.md`.
- [x] Создать документацию:
  - [x] `docs/architecture.md`
  - [x] `docs/api.md`
  - [x] `docs/openwrt.md`
- [~] Зафиксировать минимальные версии OpenWrt.
- [ ] Создать `docs/security.md`.
- [ ] Описать threat model.
- [ ] Описать release/versioning policy.

## Этап 1: MVP Agent

- [x] Создать shell-agent.
- [x] Добавить конфигурационный файл агента через `/etc/rmm-agent.conf`.
- [x] Реализовать enrollment по shared token.
- [x] Сохранять `DEVICE_ID` и `DEVICE_TOKEN`.
- [x] Реализовать heartbeat.
- [x] Отправлять inventory:
  - [x] hostname;
  - [x] OpenWrt version;
  - [x] `ubus call system board`;
  - [x] network interfaces;
  - [x] default route.
- [x] Отправлять metrics:
  - [x] `ubus call system info`;
  - [x] `/proc/loadavg`;
  - [x] `/proc/uptime`.
- [x] Добавить расширенный сбор метрик:
  - [x] memory usage в нормализованном виде;
  - [x] disk usage;
  - [x] WAN IP;
  - [x] DHCP leases;
  - [x] Wi-Fi clients;
  - [x] interface counters/errors.
- [x] Реализовать polling команд.
- [x] Реализовать отправку результата команды.
- [x] Добавить allowlist команд на агенте:
  - [x] `ping`;
  - [x] `traceroute`;
  - [x] `reboot`;
  - [x] `service_restart`;
  - [x] `opkg_list_installed`.
- [x] Добавить OpenWrt procd init script.
- [x] Добавить локальный буфер событий/результатов при потере связи.
- [x] Добавить backoff при сетевых ошибках.
- [x] Добавить lock-файл, чтобы не запускать несколько копий агента.
- [ ] Добавить shell syntax check в CI/dev scripts.
- [~] Начать Go-agent migration:
  - [x] skeleton `agent/go/cmd/rmm-agent`;
  - [x] config `/etc/rmm-agent.conf`;
  - [x] lock/backoff/graceful shutdown;
  - [x] enrollment/heartbeat;
  - [x] inventory/metrics MVP;
  - [x] command polling/result/spool;
  - [~] перенос allowlist команд из shell-agent;
    - [x] `ping`;
    - [x] `traceroute`;
    - [x] `route_show`;
    - [x] `interfaces_show`;
    - [x] `pkg_list_installed` / `opkg_list_installed`;
    - [x] `pkg_update` / `opkg_update`;
    - [x] `pkg_list_upgradable` / `opkg_list_upgradable`;
    - [x] `pkg_install` / `opkg_install`;
    - [x] `pkg_remove` / `opkg_remove`;
    - [x] `uci_show`;
    - [x] `uci_backup`;
    - [x] `uci_preview`;
    - [x] `uci_set`;
    - [x] `uci_commit`;
    - [x] `uci_commit_confirmed`;
    - [x] `uci_revert`;
    - [x] `uci_restore`;
    - [x] `remote_ssh_reverse`;
    - [x] `remote_ssh_close`;
  - [ ] OpenWrt cross-build/package integration.

## Этап 2: MVP Server API

- [x] Создать Go module.
- [x] Реализовать HTTP server.
- [x] Реализовать SQLite migrations.
- [x] Реализовать device enrollment.
- [x] Реализовать per-device bearer token.
- [x] Реализовать heartbeat endpoint.
- [x] Хранить текущее состояние устройства.
- [x] Реализовать command queue.
- [x] Реализовать shell-friendly `commands/next`.
- [x] Реализовать command result endpoint.
- [x] Реализовать device list endpoint.
- [x] Реализовать device detail endpoint.
- [x] Реализовать create command endpoint.
- [x] Добавить allowlist команд на сервере.
- [x] Добавить operator bearer token:
  - [x] `RMM_OPERATOR_TOKEN`;
  - [x] защита `/api/devices`;
  - [x] защита `/api/devices/{id}`;
  - [x] защита `/api/devices/{id}/commands`.
- [x] Разнести сервер по пакетам:
  - [x] `server/cmd/rmm-server`;
  - [x] `server/internal/httpapi`;
  - [x] `server/internal/store`;
  - [x] `server/internal/model`.
- [x] Добавить API smoke test.
- [x] Проверить `go build ./server/cmd/rmm-server`.
- [x] Проверить `go test ./...`.
- [x] Добавить endpoint истории команд: `GET /api/devices/{id}/commands`.
- [x] Добавить command detail endpoint.
- [x] Добавить timeout для `claimed` команд.
- [x] Добавить повторную выдачу зависших команд.
- [x] Добавить статус `cancelled`.
- [x] Добавить отмену команды.
- [x] Добавить basic audit log.
- [x] Добавить request id.
- [x] Добавить structured logging.
- [ ] Добавить graceful shutdown.

## Этап 3: MVP Web UI

- [x] Выбрать frontend stack: static HTML/CSS/JS.
- [x] Создать `web/`.
- [x] Добавить настройку API base URL: same-origin API.
- [x] Добавить ввод/хранение operator token для MVP.
- [x] Реализовать список устройств.
- [x] Показать online/offline.
- [x] Реализовать карточку устройства.
- [x] Показать inventory.
- [x] Показать metrics.
- [x] Показать network interfaces.
- [x] Реализовать запуск команды `ping`.
- [x] Реализовать запуск команды `traceroute`.
- [x] Показать историю команд.
- [x] Показать результат команды.
- [x] Добавить basic error states.
- [x] Добавить loading states.
- [x] Проверить UI в браузере.

## Этап 4: Command Lifecycle

- [x] Добавить поля/индексы для lifecycle, если текущей схемы недостаточно.
- [x] Определить таймаут `claimed` команд.
- [x] Вернуть зависшие `claimed` команды в `queued` или `expired`.
- [x] Добавить `expires_at`.
- [x] Добавить `attempt_count`.
- [x] Добавить `max_attempts`.
- [x] Добавить `cancelled`.
- [x] Добавить endpoint отмены команды.
- [x] Добавить command history per device.
- [x] Покрыть lifecycle тестами.
- [x] Обновить `docs/api.md`.

## Этап 5: OpenWrt Package

- [x] Создать `agent/package/Makefile`.
- [x] Описать package metadata.
- [x] Установить `/usr/bin/rmm-agent`.
- [x] Установить `/etc/init.d/rmm-agent`.
- [x] Добавить default config.
- [ ] Решить формат конфигурации:
  - [x] `/etc/rmm-agent.conf`;
  - [ ] `/etc/config/rmm-agent`.
- [ ] Добавить postinst/prerm scripts.
- [~] Проверить установку на OpenWrt.
- [x] Описать build/install flow в `docs/openwrt.md`.

## Этап 6: Security Hardening

- [x] Shared enrollment token для MVP.
- [x] Per-device bearer token.
- [x] Operator bearer token для MVP.
- [x] Allowlist команд на сервере.
- [x] Allowlist команд на агенте.
- [x] Redaction sensitive command output.
- [x] Audit log для operator actions.
- [ ] Audit log для agent actions.
- [ ] Token rotation для устройств.
- [ ] Secure enrollment flow вместо постоянного shared token.
- [ ] Hash device tokens в базе.
- [ ] Rate limits.
- [ ] Replay protection.
- [ ] Command signatures.
- [ ] RBAC.
- [ ] Organization isolation.
- [ ] mTLS или signed device tokens.

## Этап 7: OpenWrt Configuration Management

- [x] Спроектировать safe UCI operation model.
- [x] Добавить read-only UCI inventory command.
- [x] Добавить backup текущей конфигурации.
- [x] Добавить preview diff.
- [x] Добавить apply/commit/revert workflow.
- [x] Добавить restore из последнего локального backup.
- [x] Добавить commit with connectivity confirmation.
- [x] Управление Wi-Fi SSID.
- [ ] Управление firewall rules.
- [x] Управление DHCP.
- [ ] Управление static routes.
- [ ] Тесты на недопустимые изменения.

## Этап 8: Monitoring And Alerts

- [x] Online/offline detection на основе `last_seen_at`.
- [x] Базовые system metrics через heartbeat.
- [ ] История heartbeat.
- [x] Хранение временных рядов метрик.
- [x] WAN status.
- [x] Packet loss checks.
- [x] Latency checks.
- [x] Alert rules.
- [x] Alert state.
- [ ] Email notification.
- [ ] Telegram notification.
- [ ] Webhook notification.

## Этап 9: Remote Access

- [x] Спроектировать reverse tunnel.
- [x] Выбрать транспорт tunnel.
- [x] Временный SSH-доступ через сервер.
- [ ] Browser-based terminal.
- [x] TCP port forwarding.
- [x] Доступ к LuCI через tunnel.
- [x] Ограничение доступа по времени.
- [x] Audit events для remote sessions.

## Этап 10: Packages And Updates

- [x] Команда `opkg_list_installed`.
- [ ] Нормализованный список установленных пакетов.
- [x] `opkg update`.
- [x] Проверка доступных обновлений.
- [x] Установка пакета.
- [x] Удаление пакета.
- [ ] История изменений пакетов.
- [x] Защита опасных операций подтверждением.
- [~] Совместимость с конкретными версиями OpenWrt.

## Этап 11: Fleet Management

- [x] Группы устройств.
- [x] Теги.
- [x] Поиск и фильтры.
- [x] Массовые команды.
- [ ] Rollout limits.
- [ ] Canary-группы.
- [ ] Политики.
- [ ] Шаблоны конфигурации.
- [ ] Отчет о применении изменений.

## Этап 12: Production Hardening

- [x] Structured logging.
- [ ] Server metrics.
- [ ] Health checks глубже, чем `/healthz`.
- [ ] DB backup/restore.
- [ ] Версионированные миграции.
- [ ] Graceful shutdown.
- [~] Retry policies.
- [ ] E2E tests agent-server.
- [ ] Load tests.
- [ ] CI pipeline.
- [ ] Release artifacts.
- [ ] OpenWrt package build automation.

## Definition Of Done: MVP

- [x] Agent enrollment работает.
- [x] Heartbeat работает.
- [x] Basic inventory передается.
- [x] Device list API работает.
- [x] Device detail API работает.
- [x] Command queue работает.
- [x] Command result работает.
- [x] Allowlist команд есть на сервере и агенте.
- [x] Offline detection есть на server API.
- [x] Basic audit log.
- [x] Web UI показывает устройство online.
- [x] OpenWrt package устанавливает агент как service.
- [x] MVP проверен на реальном OpenWrt-устройстве.

## Ближайшие задачи

1. [ ] Собрать `rmm-agent.ipk` в OpenWrt buildroot.
2. [x] Проверить агента на реальном OpenWrt.
3. [x] Создать минимальный web UI.
4. [x] Добавить расширенные metrics/inventory.
5. [x] Добавить `max_attempts` и более явный command expiry.
6. [x] Начать UCI managed configuration.
7. [x] Добавить UCI backup command.
8. [x] Добавить UCI preview command.
9. [x] Добавить UCI presets в UI.
10. [x] Улучшить command output UX.
11. [x] Добавить agent lock/backoff/result buffer.
