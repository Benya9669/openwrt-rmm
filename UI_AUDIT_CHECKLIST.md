# UI/UX audit record

Основной аудит: 2026-07-21. Последняя синхронизация: 2026-07-31.

Это запись уже выполненной проверки, а не список будущей разработки. Актуальные
критерии повторной приёмки находятся в `UI_CHECKLIST.md`.

## Подтверждённая базовая проверка

- [x] Login, recovery, profile, password, users и sessions используют общий layout.
- [x] Fleet и все вкладки роутера проверены с длинными hostname/ID/IPv6.
- [x] Основной layout не имеет page-level overflow на 1920, 1366, 1024×768,
  768×1024, 390×844 и 360×800.
- [x] Cloud LuCI starting/offline/expired/401/403/404/409/429/502/503/504 используют
  согласованный дизайн и request ID.
- [x] Landing motion, scroll reveal и reduced-motion fallback проверены.
- [x] Видимый focus, touch targets и внутренний scroll диалогов проверены.
- [x] Notification profile и delivery history проверены локально на tablet/mobile.
- [x] Недоступные SMTP/Telegram channels блокируются с объяснением.
- [x] Provider error отображается безопасно и не раскрывает destination/secret.

## Добавлено после основного production-аудита

- [x] Notification center, unread/read, incident grouping и SSE refresh реализованы в `main`.
- [x] E-mail/Telegram verification, webhook, quiet hours и per-device overrides реализованы в `main`.
- [x] LAN clients online/recent/unconfirmed и `last_seen` реализованы в `main`.
- [ ] Эти изменения выпущены в новом server release после `server-v0.8.1`.
- [ ] Полная browser matrix повторена на опубликованном server release.
- [ ] Реальная SMTP/Telegram/webhook доставка проверена на production.
- [ ] Retry/dead-letter и длинный provider error проверены в production history.
- [ ] Online → recent → unconfirmed подтверждено на реальных wired/Wi-Fi клиентах.

## Ограничения

- Изменения пароля/e-mail на production ранее не выполнялись, чтобы не менять реальную
  учётную запись; формы и API проверялись локально.
- Галочка «реализовано в `main`» не заменяет release и production smoke.
