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

## Проверить перед выпуском notification UI

- [ ] Профиль без e-mail: e-mail channel выключен с понятной подсказкой.
- [ ] SMTP отсутствует: e-mail channel недоступен, остальные настройки сохраняются.
- [ ] Telegram token отсутствует: Telegram channel недоступен.
- [ ] Неверный Chat ID и пороги показывают ошибку формы.
- [ ] Test send показывает sent и failed без раскрытия секретов.
- [ ] История корректна при пустом списке и длинной ошибке доставки.
- [ ] Profile dialog прокручивается внутри 390×844 и 360×800.
- [ ] Keyboard navigation и zoom 200%.

## Следующие элементы

- [ ] Quiet hours/timezone.
- [ ] Webhook editor с ротацией секрета.
- [ ] Per-device notification override.
- [ ] Backup/restore wizard.
- [ ] Agent update/rollout wizard.
