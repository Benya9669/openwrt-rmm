# OpenWrt RMM — roadmap

Актуализировано: 2026-07-31.

Этот файл — единственный источник порядка продуктовой разработки. Инженерные и
production-проверки находятся в `CHECKLIST.md`, UI-критерии — в `UI_CHECKLIST.md`.

Статусы в этом документе означают:

- `[x]` — функция реализована в ветке `main` и покрыта базовыми проверками;
- `[ ]` — функция ещё не реализована полностью;
- наличие функции в `main` не означает, что она уже вошла в опубликованный server/agent
  release или развёрнута на production.

## Текущее состояние

### Агент и OpenWrt-пакеты

- [x] Go-агент `0.6.8`: enrollment, heartbeat, inventory, метрики и очередь команд.
- [x] Allowlist операций, backoff, восстановление связи и очистка lock-файла.
- [x] WAN, интерфейсы, DHCP/Wi-Fi-клиенты, память, диск и connectivity checks.
- [x] Безопасная активная проверка до 32 IPv4 DHCP-клиентов и передача результатов серверу.
- [x] Reverse SSH/LuCI tunnel через облачный сервер.
- [x] IPK/APK для основной OpenWrt-матрицы и ручной legacy workflow.
- [x] LuCI-приложение с английским интерфейсом по умолчанию.
- [x] Отдельный пакет `luci-i18n-rmm-agent-ru`.

### Сервер и аккаунты

- [x] Go API, SQLite/WAL, совместимые startup-migrations и retention метрик.
- [x] Multi-user аккаунты, admin/user роли, профиль, e-mail, пароль и отзыв сессий.
- [x] Владение роутерами, передача устройства и изоляция пользовательского API.
- [x] Одноразовый enrollment grant; legacy shared token выключен по умолчанию.
- [x] Хеширование паролей и reusable access tokens.
- [x] Rate limiting авторизации, CSRF/origin checks и audit log.
- [x] Wildcard device domains и одноразовый LuCI access grant.
- [x] Alerts с состояниями active, acknowledged и resolved.

### Уведомления и LAN-клиенты

- [x] E-mail, Telegram и подписанный webhook.
- [x] Подтверждение e-mail и безопасная привязка Telegram.
- [x] Пользовательские пороги, repeat, quiet hours/timezone и maintenance pause.
- [x] Per-device notification overrides.
- [x] Durable queue с lease, retry, dead-letter и retention истории.
- [x] Центр уведомлений: unread/read, счётчик, переход к роутеру и SSE-обновление.
- [x] Группировка связанных событий по incident.
- [x] Хранение `last_seen` LAN-клиентов и статусы online/recent/unconfirmed.

### Web UI и deployment

- [x] Лендинг, favicon, партнёры и motion/reduced-motion.
- [x] Fleet, поиск, фильтры, сортировка и автообновление.
- [x] Карточка роутера, графики, alerts, диагностика, UCI, пакеты и аудит.
- [x] Cloud LuCI со сценариями starting/offline/timeout/error.
- [x] Адаптивный desktop/tablet/4:3/mobile layout.
- [x] Hardened Docker image, Compose, NPMplus и wildcard overlay.
- [x] Версионированные server/tunnel images, SBOM/provenance и Cosign.
- [x] Подписанный IPK/APK feed через GitHub Pages.

## Порядок дальнейшей разработки

### 0. Стабилизация и выпуск текущего `main`

- [x] Дождаться полного успешного release workflow `agent-v0.6.8`, включая все текущие
  OpenWrt jobs и публикацию GitHub Pages.
- [ ] Проверить установку и обновление `rmm-agent-go-production`,
  `luci-app-rmm-agent` и `luci-i18n-rmm-agent-ru` из подписанного feed на реальном
  OpenWrt 24.10/25.12.
- [x] Подготовить и выпустить подписанный `server-v0.9.0` с notification center,
  verification, per-device settings и LAN client presence.
- [ ] Выпустить `server-v0.9.1` с операционными метриками уведомлений, фильтрами,
  вкладками личного кабинета и очисткой шума LAN-клиентов.
- [ ] Проверить миграцию копии production SQLite, сделать backup и только затем обновить
  production.
- [ ] Выполнить production-проверки SMTP, Telegram, webhook, active → repeat → resolved,
  retry → dead-letter, SSE и online → recent → unconfirmed.
- [x] Добавить операционные метрики уведомлений: queued/sent/failed, queue age,
  последняя ошибка и последняя успешная доставка по каналу.

### 1. Изоляция туннелей и защита управляющих команд

- [ ] Персональные SSH credentials или короткоживущие сертификаты для каждого устройства.
- [ ] Ротация/отзыв tunnel credentials при transfer, delete и компрометации.
- [ ] Авторизованное выделение портов без возможности pre-bind/hijack другой сессии.
- [ ] Проверка полной цепочки agent → SSH → LuCI HTTP → wildcard TLS.
- [ ] Лимиты одновременных сессий, rate limit и административная политика remote access.
- [ ] Device token rotation и явный отзыв device credentials.
- [ ] Подписанные команды, срок действия, nonce и replay protection.
- [ ] Шифрование webhook secrets и чувствительных полей notification queue в SQLite.

### 2. Автоматические проверки и наблюдаемость

- [ ] Интеграционные тесты notification center, contact verification, per-device
  overrides, incident grouping и quiet hours/timezone.
- [ ] Тесты активной проверки LAN-клиентов, лимита адресов, ICMP-blocked и IPv6-сценариев.
- [ ] E2E reverse tunnel и cloud LuCI с проверкой ошибок и истечения сессии.
- [ ] Playwright smoke для login, fleet, router, profile, notifications и LuCI errors.
- [ ] Автоматическая responsive/accessibility матрица 1920/1366/1024/768/390/360.
- [ ] Package install/upgrade/remove smoke для IPK/APK и проверка LuCI i18n.
- [ ] Prometheus metrics, readiness, queue/tunnel metrics и структурированные логи.
- [ ] `govulncheck`, SAST, container/dependency/secret scanning.
- [ ] Pin GitHub Actions и базовых container images на проверяемые immutable revisions.

### 3. Backup и безопасное восстановление

- [ ] Получение `sysupgrade -b` архива через агент.
- [ ] Шифрованное хранение, retention и контроль доступа к backup.
- [ ] Скачивание, безопасный diff и проверка совместимости с target/device/version.
- [ ] Restore preview, повторное подтверждение и план отката.
- [ ] Backup/restore SQLite через согласованный snapshot, а не копирование активного файла.
- [ ] Disaster recovery runbook и регулярный restore drill.

### 4. Обновления агента и LuCI

- [x] Подписанный stable update manifest с версиями, target/feed compatibility и Sigstore bundle.
- [x] Проверка detached manifest signature сервером перед публикацией stable version в UI.
- [ ] Проверка manifest signature агентом перед выполнением обновления.
- [ ] Проверка свободного места, feed signature и package health до обновления.
- [ ] Обновление одного роутера из кабинета с progress/reconnect/result.
- [ ] Canary и поэтапный rollout с автоматической остановкой при ошибках.
- [ ] История версий и rollback, если агент не вернулся после обновления.
- [ ] Stable/candidate release channels без перемещения опубликованных тегов.

### 5. Организации и расширенная безопасность

- [ ] Organizations/workspaces и приглашения.
- [ ] Роли owner/admin/operator/viewer.
- [ ] Доступ к группам и отдельным устройствам без передачи владельца.
- [ ] MFA/WebAuthn и recovery codes.
- [ ] Audit retention/export и журнал административных изменений.
- [ ] Удаление аккаунта, экспорт данных и отзыв связанных credentials.
- [ ] Опциональный mTLS для agent transport.

### 6. Расширение cloud DNS и fleet management

- [ ] Безопасное переименование, резервирование и освобождение router DNS names.
- [ ] Проверка wildcard DNS/TLS/tunnel health в интерфейсе.
- [ ] Защита от повторного захвата недавно освобождённого имени и пользовательские квоты.
- [ ] Опциональные пользовательские домены.
- [ ] Конфигурационные шаблоны, группы, config drift и плановые команды.
- [ ] Диагностический архив агента и безопасная выгрузка логов.
- [ ] Интернационализация web UI и правила добавления языков LuCI.

### 7. Масштабирование и долговременный hardening

- [ ] Версионированные migrations и отдельный migration log.
- [ ] Graceful shutdown HTTP server, workers и активных lease.
- [ ] Distributed rate limiting и worker coordination для нескольких server instances.
- [ ] Load/soak tests, SLO и алерты состояния самой RMM-платформы.
- [ ] Политика хранения audit, metrics, notifications и client history.
