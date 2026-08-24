# Инженерный и release checklist

Актуализировано: 2026-08-24. Порядок продуктовой разработки задаёт `ROADMAP.md`.
Этот файл содержит только критерии готовности к merge/release/deploy.

## Состояние ветки `main`

- [x] Go agent source и production package подготовлены с версией `0.8.0`.
- [x] LuCI package `0.2.2` и отдельный `luci-i18n-rmm-agent-ru`.
- [x] Notification center, verification, quiet hours, webhook, per-device overrides и incidents.
- [x] Delivery metrics, channel diagnostics и server-side notification history filters.
- [x] Profile/Security/Notifications tabs и admin-only Users tab.
- [x] LAN client persistence с online/recent/unconfirmed.
- [x] Current OpenWrt 24.10/25.12 matrix и отдельная manual legacy matrix.
- [x] Подписанные IPK/APK repositories, SBOM/provenance и Cosign для server images.
- [x] Stable update manifest с package compatibility, ECDSA signature и Sigstore bundle.
- [x] DirectDNS удалён; cloud access использует wildcard domain и исходящий tunnel.
- [x] Secure tunnel mode: per-device Ed25519 keys, pinned host key, dynamic port-scoped authorization, key epoch rotation и принудительное закрытие отозванных listeners.

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
- [x] `agent-v0.6.14` опубликован без преждевременного restart при managed update; production update подтверждён.
- [x] `server-v0.9.6` опубликован с heartbeat reconciliation ложного результата update.
- [x] `server-v0.10.0` и `agent-v0.7.0` опубликованы с per-device tunnel credentials и исправлением LAN client layout.
- [x] CI опубликовал images, IPK/APK и подписанные feed indexes для `server-v0.10.0` / `agent-v0.7.0`.
- [x] `server-v0.10.1` подготовлен для single-file Arcane GitOps, стабильных volume names и отображения server version.
- [x] `server-v0.10.2` передаёт token в `AuthorizedKeysCommand` через защищённые runtime-файлы и покрывает очищенное окружение регрессионным тестом.
- [x] `server-v0.11.0` опубликован с полным UI redesign и расширенной browser regression matrix.
- [x] `server-v0.11.1` опубликован с обновлёнными favicon/PWA assets.
- [x] `server-v0.11.2` опубликован с согласованными Login и cloud LuCI error states.
- [x] Windows E2E teardown исправлен и deployment defaults синхронизированы для следующего релиза.
- [ ] `server-v0.12.0` / `agent-v0.8.0` опубликованы после migration/package smoke.
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
- [ ] Сохранены отдельные офлайн-копии SQLite snapshot, command-signing key и data-encryption key; выполнен test restore.
- [ ] Проверены create/list/restore/delete router backup и несовместимый target.

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

Рабочее дерево `server-v0.12.0` / `agent-v0.8.0`, 2026-08-24:

- [x] `go test ./...` и `go vet ./...`.
- [x] `npm run check:web`.
- [x] Playwright: 27 сценариев, включая responsive UI и zoom 200%.
- [x] Base/release/NPMplus/dev Compose configurations валидны.
- [x] Shell syntax и UCI-sync regression проверены через Git Bash.
- [ ] Docker server/tunnel build: локальный Docker daemon не запущен.
- [ ] OpenWrt package smoke и production migration/restore drill.

Предыдущий опубликованный baseline:

Проверка `server-v0.11.2` / `agent-v0.7.0`, 2026-08-24:

- [x] `go test ./...` и `go vet ./...`.
- [x] `npm run check:web`.
- [x] Playwright: 27 сценариев, включая полный redesigned UI, 320–2560 px и zoom 200%.
- [x] Base/release/NPMplus Compose configurations валидны.
- [x] Main CI и server release workflow завершены успешно.
- [x] `server-v0.11.2` опубликован из GPG-signed tag.
- [x] `agent-v0.7.0` и подписанные current OpenWrt feeds опубликованы.
- [ ] Production smoke выполнен на release, содержащем текущий `main`.
