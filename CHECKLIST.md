# Инженерный и release checklist

Актуализировано: 2026-08-11. Порядок продуктовой разработки задаёт `ROADMAP.md`.
Этот файл содержит только критерии готовности к merge/release/deploy.

## Состояние ветки `main`

- [x] Go agent source и production package подготовлены с версией `0.6.14`.
- [x] LuCI package `0.2.2` и отдельный `luci-i18n-rmm-agent-ru`.
- [x] Notification center, verification, quiet hours, webhook, per-device overrides и incidents.
- [x] Delivery metrics, channel diagnostics и server-side notification history filters.
- [x] Profile/Security/Notifications tabs и admin-only Users tab.
- [x] LAN client persistence с online/recent/unconfirmed.
- [x] Current OpenWrt 24.10/25.12 matrix и отдельная manual legacy matrix.
- [x] Подписанные IPK/APK repositories, SBOM/provenance и Cosign для server images.
- [x] Stable update manifest с package compatibility, ECDSA signature и Sigstore bundle.
- [x] DirectDNS удалён; cloud access использует wildcard domain и исходящий tunnel.

## Текущее состояние релизов

- [x] `server-v0.9.0`, `server-v0.9.1` и `server-v0.9.2` опубликованы.
- [x] `agent-v0.6.9` опубликован, package feed и исторические manifests сохранены.
- [x] `server-v0.9.3` опубликован из подписанного commit/tag.
- [x] `agent-v0.6.10` подписан, но package matrix остановилась до сборки из-за устаревшего parser версии.
- [x] Matrix `agent-v0.6.11` отменена без завершённого package release; опубликованный tag не перемещать.
- [x] `agent-v0.6.12` опубликован; production upgrade `0.6.9 → 0.6.12` завершён.
- [x] `server-v0.9.4` опубликован как hotfix центра уведомлений.
- [x] `agent-v0.6.13` опубликован с прямым OpenWrt APK `packages.adb` URL; production upgrade выполнен.
- [x] `server-v0.9.5` опубликован с canonical package manifest origin.
- [ ] `agent-v0.6.14` подготовлен без преждевременного restart при managed update.
- [ ] `server-v0.9.6` подготовлен с heartbeat reconciliation ложного результата update.
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

Проверка функциональной подготовки `server-v0.9.3` / `agent-v0.6.10`, 2026-08-10:

- [x] `gofmt`, `go test ./...` и `go vet ./...`.
- [x] `npm run check:web`.
- [x] Playwright: 12 сценариев, включая rollback, reconnect, 1920/1366/1024/768/390/360 и zoom 200%.
- [x] `docker compose config --quiet`.
- [x] Локальный server image `0.9.3` собран и отвечает на `/healthz`.
- [x] OpenWrt 25.12.4 ramips/mt7621 smoke создал agent `0.6.10`, LuCI `0.2.2`, Russian i18n и APK index; checksums проверены.
- [ ] Полная 24.10/25.12 CI matrix и подписанные release feeds опубликованы.
- [ ] Реальный роутер проверен для install и `0.6.9 → 0.6.10 → 0.6.9` rollback.

Последний опубликованный commit:

Commit: `5905478` (`agent-v0.6.9`).

- [x] `go test ./...`.
- [x] `go vet ./...`.
- [x] `npm run check:web`.
- [x] `docker compose config --quiet`.
- [x] Main CI завершён успешно.
- [x] Локальная OpenWrt 24.10.7 ramips/mt7621 сборка создала agent, LuCI и Russian i18n.
- [x] Полная tagged matrix `agent-v0.6.9` и GitHub Pages deployment завершены.
- [ ] Production smoke выполнен на release, содержащем текущий `main`.
