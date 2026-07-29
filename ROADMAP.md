# OpenWrt RMM — актуальный roadmap

Актуализировано: 2026-07-30. Текущая стабильная линия агента: `0.6.6`.

## Цель продукта

OpenWrt RMM — self-hosted платформа для мониторинга и безопасного управления роутерами
OpenWrt через исходящее соединение агента. Пользователь видит только свои устройства,
получает предупреждения и открывает LuCI через облако без публичного порта на роутере.

## Реализовано

### Агент и пакеты

- [x] Go-агент с enrollment, heartbeat, inventory, метриками и очередью команд.
- [x] Allowlist операций, backoff, восстановление связи и очистка lock-файла при остановке.
- [x] Сбор WAN, интерфейсов, DHCP/Wi-Fi-клиентов, памяти, диска и connectivity checks.
- [x] Reverse SSH/LuCI tunnel через облачный сервер.
- [x] APK и IPK для OpenWrt, включая `mipsel`/`ramips/mt7621`.
- [x] LuCI-приложение настройки агента.

### Server и безопасность

- [x] Go API, SQLite/WAL, автоматические совместимые миграции и retention метрик.
- [x] Multi-user аккаунты, профиль, e-mail, смена/сброс пароля и отзыв сессий.
- [x] Владение устройствами, передача другому пользователю и изоляция API.
- [x] Одноразовый enrollment grant; legacy shared token выключен по умолчанию.
- [x] Хеширование паролей, browser sessions, enrollment/device/access token.
- [x] Rate limit входа и восстановления пароля, CSRF/origin checks, audit log.
- [x] Облачные поддомены роутеров и одноразовый LuCI access grant.
- [x] Persistent alerts: active, acknowledged, resolved.
- [x] Пользовательские пороги и уведомления e-mail/Telegram с дедупликацией, повторами и историей.

### Web UI

- [x] Лендинг, favicon/бренд, партнёры и адаптивная motion-система.
- [x] Полноширинный список объектов, поиск, фильтры, сортировка и автообновление.
- [x] Карточка роутера: обзор, клиенты, сеть, операции, сведения, эксперт.
- [x] Графики метрик, диагностика, команды, UCI, пакеты и аудит.
- [x] Облачный LuCI с состояниями starting/offline/timeout/error и единым дизайном ошибок.
- [x] Mobile/tablet/4:3 layout без случайного горизонтального overflow.
- [x] Личный кабинет и настройки уведомлений.

### Deployment

- [x] Hardened Docker image и Compose.
- [x] NPMplus overlay и wildcard device domain.
- [x] Healthcheck, ограничение capabilities/resources и persistent volumes.
- [x] SMTP STARTTLS/TLS и Telegram bot configuration через environment.
- [x] Версионированные server/tunnel images в GHCR и production Compose overlay.
- [x] SBOM/provenance, keyless Cosign и подписанный APK/IPK package repository.
- [x] Быстрая матрица текущих OpenWrt 24.10/25.12 и отдельная ручная legacy-сборка
  OpenWrt 21.02/22.03/23.05 без блокировки основного релиза.

## Следующие этапы

### Этап 1 — завершение notification platform

- [x] E-mail и Telegram для warning/critical/resolved.
- [x] Пользовательские пороги памяти, диска, потерь и latency.
- [x] Lifecycle deduplication, повтор открытой проблемы и журнал доставки.
- [x] Тестовая отправка из профиля.
- [x] Перезапускаемая очередь с lease, exponential retry, dead-letter и retention терминальной истории.
- [ ] Webhook-канал с подписанным payload и ротацией секрета.
- [ ] Per-device override и расписание тишины.
- [ ] Группировка нескольких событий в incident.

### Уточнение присутствия проводных клиентов

- [ ] Дополнить пассивный `ip neigh` безопасной активной проверкой LAN-клиентов.
- [ ] Хранить время последнего подтверждения и показывать `STALE` как «Недавно был в сети».

### Этап 2 — backup и безопасное восстановление

- [ ] Получение `sysupgrade -b` архива через агент.
- [ ] Зашифрованное хранение и retention версий.
- [ ] Скачивание, сравнение и проверка совместимости.
- [ ] Restore с preview, подтверждением и планом отката.
- [ ] Backup/restore самой SQLite базы RMM.

### Этап 3 — обновления и release pipeline

- [x] Автосборка APK/IPK и multi-architecture artifacts в CI.
- [x] Нативная подпись IPK/APK feed, Cosign checksums и публикация репозитория.
- [ ] Подписанный update manifest агента и LuCI-приложения.
- [ ] Обновление одного роутера из кабинета.
- [ ] Canary/поэтапный rollout с остановкой при потере связи.
- [ ] История и безопасный rollback версии агента.

### Этап 4 — усиление cloud access

- [ ] Персональные ключи туннеля для каждого устройства и их ротация.
- [ ] Browser terminal с ограниченной сессией и аудитом.
- [ ] Детальная проверка цепочки agent → SSH → HTTP LuCI → TLS.
- [ ] Лимиты одновременных сессий и административная политика доступа.

### Этап 5 — организации и расширенная безопасность

- [ ] Organizations/workspaces и приглашения.
- [ ] Роли owner/admin/operator/viewer и права на отдельные группы устройств.
- [ ] MFA/WebAuthn.
- [ ] Device token rotation, signed commands и replay protection.
- [ ] mTLS как дополнительный режим agent transport.

### Этап 6 — production hardening

- [ ] Версионированные migrations с отдельным migration log.
- [ ] Graceful shutdown фоновых задач и HTTP server.
- [ ] Server Prometheus metrics и расширенные health/readiness checks.
- [ ] E2E agent-server, browser smoke и load tests в CI.
- [ ] Disaster recovery runbook и регулярная проверка восстановления.

## Ближайший приоритет

1. Завершить и проверить `agent-v0.6.6`, при необходимости добавить legacy-пакеты,
   развернуть server `0.8.1` и проверить свежие SSH/LuCI-сессии.
2. Развернуть и проверить notification release на production.
3. Добавить активное подтверждение проводных клиентов, webhook и quiet hours, затем
   начать конфигурационные backup/restore.
