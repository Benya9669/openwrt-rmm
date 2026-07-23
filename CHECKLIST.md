# Актуальный инженерный checklist

Синхронизировано с кодом 2026-07-23. Источник продуктового порядка — `ROADMAP.md`.

## Работает сейчас

- [x] Go server, SQLite/WAL, Docker/Compose и `/healthz`.
- [x] Go agent 0.6.3, heartbeat, команды, backoff и OpenWrt init integration.
- [x] APK/IPK и LuCI-пакет.
- [x] Multi-user, роли admin/user, владение и передача роутеров.
- [x] Профиль, e-mail, смена/сброс пароля и управление сессиями.
- [x] Одноразовый secure enrollment; reusable token secrets хранятся как hash.
- [x] Alerts: offline, WAN, memory, disk, packet loss, latency, WAN IP, commands.
- [x] Acknowledge/resolved lifecycle и audit events.
- [x] E-mail/Telegram notifications, пользовательские пороги, repeat и delivery history.
- [x] Durable notification queue: leased claims, restart recovery, exponential retry, dead-letter и retention.
- [x] Reverse tunnel, cloud LuCI и wildcard router domain.
- [x] DirectDNS удалён из продукта; адресация роутеров работает только через cloud/wildcard-домен и исходящий туннель.
- [x] UCI preview/apply/commit/revert/backup/restore operations.
- [x] Fleet filters, bulk commands, metrics history и SSE auto-refresh.
- [x] Адаптивный UI и согласованные error/loading/empty состояния.

## Обязательное перед следующим production-релизом

- [x] Сгенерировать ключи `usign`/APK и зафиксировать проверяемые публичные ключи.
- [ ] Сохранить приватные ключи в зашифрованной офлайн-копии и добавить два base64-значения в GitHub Actions Secrets.
- [ ] Включить GitHub Pages из Actions и проверить первый подписанный package feed на реальном роутере.
- [ ] Выпустить первый `server-v*`, проверить Cosign/provenance и закрепить `RMM_RELEASE_VERSION` в production `.env`.
- [ ] Настроить SMTP и/или `RMM_TELEGRAM_BOT_TOKEN` в production environment.
- [ ] Выполнить тестовую отправку из реального пользовательского профиля.
- [ ] Проверить active → repeat → resolved на тестовом роутере.
- [ ] Проверить миграцию существующей production SQLite базы на копии.
- [ ] Сделать backup базы перед обновлением контейнера.
- [ ] Проверить `docker compose config --quiet` и healthcheck после deploy.

## Следующая разработка

- [ ] Добавить подтверждение e-mail и безопасную привязку Telegram-чата перед включением канала.
- [ ] Сделать встроенный центр уведомлений: unread/read, счётчик, переход к роутеру и обновление через SSE.
- [ ] Настройки по типам событий, maintenance/snooze и временное подавление алертов на период работ.
- [ ] Добавить операционные метрики уведомлений: queued/sent/failed, возраст очереди и последняя ошибка канала.
- [ ] Активная проверка проводных клиентов и `last_seen`, чтобы flow offload не оставлял реально работающий ПК в состоянии `STALE`.
- [ ] Подписанный webhook channel.
- [ ] Quiet hours/timezone и per-device notification overrides.
- [ ] Конфигурационные backup artifacts и retention.
- [x] CI для тестов, Docker, multi-version APK/IPK и release artifacts.
- [x] Keyless Cosign, provenance/SBOM контейнеров и подписанные checksum релизов.
- [x] Нативные подписанные `Packages.sig`/`packages.adb` и публикация package feed.
- [ ] Signed update manifest и безопасное обновление агента из кабинета.
- [ ] Per-device tunnel credentials.
- [ ] Organizations и расширенный RBAC.
- [ ] MFA, command signatures и replay protection.
- [ ] Graceful shutdown, readiness, server metrics и load tests.

## Постоянная проверка изменений

Это повторяемый шаблон для каждого следующего изменения. Результат последнего проверенного коммита фиксируется отдельно ниже.

- [ ] `gofmt` и `go test ./...`.
- [ ] `go vet ./...`.
- [ ] `node --check web/app.js` и `web/landing-motion.js`.
- [ ] `docker compose config --quiet`.
- [ ] Docker build.
- [ ] Desktop 1366/1920, 4:3 1024, tablet 768 и mobile 390/360.
- [ ] Отсутствие секретов и случайных production-файлов в diff.

## Последняя верификация — `6a03061`

- [x] `gofmt`, `go test ./...` и `go vet ./...`.
- [x] `node --check web/app.js` и `web/landing-motion.js`.
- [x] `docker compose config --quiet` и Docker build.
- [x] Notification profile: 768×1024, 390×844 и 360×800 без горизонтального overflow.
- [x] Недоступные SMTP/Telegram-каналы блокируются с понятной подсказкой.
- [x] Ошибки тестовой отправки локализованы и сохраняют request ID.
- [x] Проверены diff и отсутствие реальных секретов; присутствуют только безопасные example-placeholder значения.

## Документация при изменении API и конфигурации

- [ ] Синхронизировать `server/README.md`, `docs/api.md` и `.env.example` с endpoint и переменными окружения.
- [ ] Обновить `docs/recent-progress.md`, checklist и roadmap без противоречащих статусов.
