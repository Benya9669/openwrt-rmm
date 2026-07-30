# UI/UX roadmap

Актуализировано: 2026-07-31. Общий порядок разработки находится в `ROADMAP.md`.
Здесь перечислены только UI-результаты, зависящие от соответствующих backend-этапов.

## Реализовано в `main`

- [x] Общие design tokens, типографика, focus и motion/reduced-motion.
- [x] Адаптивный landing, partners, app shell и mobile navigation.
- [x] Fleet, фильтры, сортировка и состояния online/offline/problem.
- [x] Карточка роутера, локально прокручиваемые таблицы и mobile layout.
- [x] Метрики, alerts, диагностика, UCI, пакеты и audit.
- [x] Cloud LuCI waiting/error flow.
- [x] Профиль, e-mail, пароль, пользователи и сессии.
- [x] Notification settings, verification, delivery history и test send.
- [x] Notification center, unread counter, incident grouping и переход к роутеру.
- [x] Quiet hours/timezone, maintenance pause, webhook и per-device overrides.
- [x] LAN clients со статусами «В сети», «Недавно был в сети» и «Не подтверждён».
- [x] Личный кабинет разделён на Profile, Security, Notifications и admin-only Users.
- [x] Метрики очереди, диагностика каналов и фильтры истории доставки.

## Следующие UI-этапы

### 1. Стабилизация текущего интерфейса

- [x] Фильтры notification center/history по роутеру, severity, event, channel и delivery status.
- [x] Состояние каналов: последняя успешная доставка, последняя ошибка и queue age.
- [ ] Production regression для notification, LAN clients и cloud access.
- [ ] Автоматический browser smoke и responsive/accessibility matrix.

### 2. Безопасный remote access

- [ ] Показ tunnel identity/health без раскрытия credentials.
- [ ] Понятные состояния port allocation, access denied, expired и revoked.
- [ ] Browser terminal с ограниченной сессией, предупреждениями и audit trail.
- [ ] Экран активных сессий и административных лимитов.

### 3. Backup/restore

- [ ] Список архивов с device/version/date/size/status.
- [ ] Создание, скачивание и retention.
- [ ] Безопасный diff с секретами, скрытыми по умолчанию.
- [ ] Restore wizard с compatibility check, повторным подтверждением и rollback warning.

### 4. Обновления

- [ ] Установленная и доступная версии agent/LuCI.
- [ ] Update preview, progress, reconnect и итоговый health result.
- [ ] Canary/group rollout и автоматическая остановка при ошибке.
- [ ] История обновлений и rollback.

### 5. Organizations и безопасность

- [ ] Workspace switcher и приглашения.
- [ ] Роли owner/admin/operator/viewer.
- [ ] Матрица прав групп и устройств.
- [ ] MFA/WebAuthn, recovery codes и управление device credentials.

### 6. Доступность и локализация

- [ ] Web UI i18n без жёстко заданного русского текста.
- [ ] Проверка keyboard-only, screen reader, contrast и zoom 200% в CI.
- [ ] Полные loading/empty/offline/error состояния для каждого нового wizard.
