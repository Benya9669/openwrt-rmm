# UI checklist

Синхронизировано: 2026-07-22. Детальный последний аудит находится в
`UI_AUDIT_CHECKLIST.md`.

## Проверено и реализовано

- [x] Вход, восстановление, профиль, пароль и управление пользователями.
- [x] Fleet list, поиск, фильтры, сортировка, offline/online/problem states.
- [x] Все вкладки роутера и длинные hostname/ID/IPv6.
- [x] DHCP/Wi-Fi-клиенты не считаются online только из-за static lease.
- [x] Metrics charts, alerts, acknowledge и operator diagnostics.
- [x] Cloud LuCI button, waiting page и согласованные ошибки.
- [x] Landing motion и reduced-motion fallback.
- [x] Desktop, 4:3, tablet и mobile без page-level overflow.
- [x] Notification settings, thresholds, test send и delivery history.
- [x] История различает queued/sending/retry/sent/dead-letter и показывает номер попытки и время повтора.

## Проверить перед выпуском notification UI

- [ ] Профиль без e-mail: e-mail channel выключен с понятной подсказкой.
- [x] SMTP отсутствует: e-mail channel недоступен, остальные настройки сохраняются.
- [x] Telegram token отсутствует: Telegram channel недоступен.
- [ ] Неверный Chat ID и пороги показывают ошибку формы.
- [ ] Test send показывает sent/retry/dead-letter без раскрытия секретов.
- [ ] История корректна при пустом списке и длинной ошибке доставки.
- [ ] Profile dialog прокручивается внутри 390×844 и 360×800.
- [ ] Keyboard navigation и zoom 200%.

## Следующие элементы

- [ ] Встроенный notification center: колокольчик, unread/read, счётчик и переход к источнику события.
- [ ] Фильтры уведомлений по роутеру, типу, severity, каналу и статусу доставки.
- [ ] Настройки отдельных типов событий и maintenance/snooze для роутера.
- [ ] Экран состояния каналов: последняя успешная отправка, последняя ошибка и тест соединения.
- [ ] Quiet hours/timezone.
- [ ] Webhook editor с ротацией секрета.
- [ ] Per-device notification override.
- [ ] Backup/restore wizard.
- [ ] Agent update/rollout wizard.
