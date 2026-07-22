# Актуальный инженерный checklist

Синхронизировано с кодом 2026-07-22. Источник продуктового порядка — `ROADMAP.md`.

## Работает сейчас

- [x] Go server, SQLite/WAL, Docker/Compose и `/healthz`.
- [x] Go agent 0.6.1, heartbeat, команды, backoff и OpenWrt init integration.
- [x] APK/IPK и LuCI-пакет.
- [x] Multi-user, роли admin/user, владение и передача роутеров.
- [x] Профиль, e-mail, смена/сброс пароля и управление сессиями.
- [x] Одноразовый secure enrollment; reusable token secrets хранятся как hash.
- [x] Alerts: offline, WAN, memory, disk, packet loss, latency, WAN IP, commands.
- [x] Acknowledge/resolved lifecycle и audit events.
- [x] E-mail/Telegram notifications, пользовательские пороги, repeat и delivery history.
- [x] Reverse tunnel, cloud LuCI и wildcard router domain.
- [x] UCI preview/apply/commit/revert/backup/restore operations.
- [x] Fleet filters, bulk commands, metrics history и SSE auto-refresh.
- [x] Адаптивный UI и согласованные error/loading/empty состояния.

## Обязательное перед следующим production-релизом

- [ ] Настроить SMTP и/или `RMM_TELEGRAM_BOT_TOKEN` в production environment.
- [ ] Выполнить тестовую отправку из реального пользовательского профиля.
- [ ] Проверить active → repeat → resolved на тестовом роутере.
- [ ] Проверить миграцию существующей production SQLite базы на копии.
- [ ] Сделать backup базы перед обновлением контейнера.
- [ ] Проверить `docker compose config --quiet` и healthcheck после deploy.

## Следующая разработка

- [ ] Активная проверка проводных клиентов и `last_seen`, чтобы flow offload не оставлял реально работающий ПК в состоянии `STALE`.
- [ ] Подписанный webhook channel.
- [ ] Quiet hours/timezone и per-device notification overrides.
- [ ] Конфигурационные backup artifacts и retention.
- [ ] CI для тестов, Docker, APK/IPK и release artifacts.
- [ ] Signed update manifest и безопасное обновление агента.
- [ ] Per-device tunnel credentials.
- [ ] Organizations и расширенный RBAC.
- [ ] MFA, command signatures и replay protection.
- [ ] Graceful shutdown, readiness, server metrics и load tests.

## Постоянная проверка изменений

- [ ] `gofmt` и `go test ./...`.
- [ ] `go vet ./...`.
- [ ] `node --check web/app.js` и `web/landing-motion.js`.
- [ ] `docker compose config --quiet`.
- [ ] Docker build.
- [ ] Desktop 1366/1920, 4:3 1024, tablet 768 и mobile 390/360.
- [ ] Отсутствие секретов и случайных production-файлов в diff.
