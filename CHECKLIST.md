# Инженерный и release checklist

Актуализировано: 2026-07-31. Порядок продуктовой разработки задаёт `ROADMAP.md`.
Этот файл содержит только критерии готовности к merge/release/deploy.

## Состояние ветки `main`

- [x] Go agent source и production package имеют версию `0.6.8`.
- [x] LuCI package `0.2.2` и отдельный `luci-i18n-rmm-agent-ru`.
- [x] Notification center, verification, quiet hours, webhook, per-device overrides и incidents.
- [x] LAN client persistence с online/recent/unconfirmed.
- [x] Current OpenWrt 24.10/25.12 matrix и отдельная manual legacy matrix.
- [x] Подписанные IPK/APK repositories, SBOM/provenance и Cosign для server images.
- [x] Stable update manifest с package compatibility, ECDSA signature и Sigstore bundle.
- [x] DirectDNS удалён; cloud access использует wildcard domain и исходящий tunnel.

## Текущее состояние релизов

- [x] `server-v0.8.1` опубликован, но не содержит последних notification/client изменений.
- [x] `agent-v0.6.8` создан как подписанный tag.
- [x] Все jobs `agent-v0.6.8` завершены успешно и package feed опубликован.
- [ ] Следующий server tag создан с GPG-подписью и содержит текущий `main`.
- [ ] Production закреплён на точной `RMM_RELEASE_VERSION`, а не `latest`.

## Перед `agent-v*`

- [ ] Версия совпадает в Go source и production package.
- [ ] В `CHANGELOG.md` есть английский раздел для точного tag.
- [ ] Tag подписан GPG и ранее не публиковался.
- [ ] Основная OpenWrt matrix собрана без ошибок.
- [ ] Release содержит только устанавливаемые `.ipk`/`.apk`.
- [ ] Feed содержит подписанные indexes и публичные ключи.
- [ ] Update manifest и Sigstore bundle опубликованы и независимо проверены.
- [ ] Установлены agent, LuCI и optional Russian i18n из опубликованного feed.
- [ ] Проверены install, upgrade, restart, stop и удаление runtime lock/state.
- [ ] При необходимости manual legacy workflow добавляет пакеты без перемещения tag.

## Перед `server-v*`

- [ ] В `CHANGELOG.md` есть английский раздел для точного tag.
- [ ] Tag подписан GPG и ранее не публиковался.
- [ ] `go test ./...`, `go vet ./...`, web checks и Docker build прошли.
- [ ] Compose base/release/NPMplus configurations валидны.
- [ ] Server и tunnel images имеют один version tag, digest, SBOM, provenance и Cosign signature.
- [ ] Миграция проверена на копии актуальной production SQLite.
- [ ] Подготовлен rollback на предыдущий image tag.

## Перед production deploy

- [ ] Приватные package/tunnel keys имеют зашифрованную офлайн-копию.
- [ ] GitHub Actions secrets настроены; секреты отсутствуют в repository и build artifacts.
- [ ] Сделан согласованный backup SQLite и проверено его чтение в отдельном окружении.
- [ ] `.env` использует точный release, production URL/domain и secure cookie.
- [ ] Legacy enrollment/LuCI proxy и insecure dev mode выключены.
- [ ] `docker compose config --quiet`, pull/up и service healthcheck прошли.
- [ ] Проверены login, profile, password reset, enrollment и user isolation.
- [ ] Проверены свежие SSH/LuCI sessions, timeout, close и error states.
- [ ] Проверены SMTP/Telegram/webhook test sends без раскрытия секретов.
- [ ] Проверены active → repeat → resolved и retry → dead-letter.
- [ ] Проверены notification center/SSE и quiet hours/maintenance.
- [ ] Проверены LAN client online → recent → unconfirmed.
- [ ] Проверены desktop, 4:3, tablet и mobile.

## Для каждого изменения

- [ ] Diff ограничен задачей и не содержит generated/production/secret файлов.
- [ ] `gofmt` и `go test ./...`.
- [ ] `go vet ./...`.
- [ ] `npm run check:web`.
- [ ] `docker compose config --quiet`, если затронут deployment.
- [ ] Docker build, если затронут server image или runtime.
- [ ] OpenWrt package smoke, если затронут agent/LuCI/builder/workflow.
- [ ] Добавлены или обновлены тесты для изменённой логики.
- [ ] Обновлены API/config/deployment docs, если изменился контракт.
- [ ] `ROADMAP.md` меняется только при изменении фактического статуса или порядка работ.

## Последняя подтверждённая проверка

Commit: `a343963`.

- [x] `go test ./...`.
- [x] `go vet ./...`.
- [x] `npm run check:web`.
- [x] `docker compose config --quiet`.
- [x] Main CI завершён успешно.
- [x] Локальная OpenWrt 24.10.7 ramips/mt7621 сборка создала agent, LuCI и Russian i18n.
- [x] Полная tagged matrix `agent-v0.6.8` и GitHub Pages deployment завершены.
- [ ] Production smoke выполнен на release, содержащем текущий `main`.
