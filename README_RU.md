<p align="center">
  <img src="web/web-app-192.png" width="112" alt="Логотип OpenWrt RMM">
</p>

# OpenWrt RMM

[English](README.md) · [Релизы](https://github.com/Benya9669/openwrt-rmm/releases) · [Changelog](CHANGELOG.md) · [Безопасность](docs/security.md)

OpenWrt RMM — самостоятельная платформа удалённого мониторинга и управления роутерами
OpenWrt. В неё входят облачный сервер на Go, компактный исходящий агент, адаптивный
веб-интерфейс, временный доступ по SSH и LuCI, уведомления и подписанные репозитории
пакетов OpenWrt.

На роутере не требуется открывать входящий порт. Агент подключается к серверу по HTTPS,
а ограниченный обратный туннель создаётся только по запросу авторизованного оператора.

## Возможности

- Отдельные аккаунты пользователей и привязка роутеров к владельцу.
- Одноразовые коды подключения роутеров.
- Инвентаризация, метрики, проверки соединения и определение присутствия LAN-клиентов.
- Безопасный ограниченный набор диагностических, пакетных и UCI-команд.
- Временный доступ к SSH и LuCI через изолированный облачный туннель.
- Центр уведомлений, e-mail, Telegram и подписанные webhook-запросы.
- Тихие часы, режим обслуживания и настройки алертов для каждого роутера.
- Адаптивный веб-интерфейс и LuCI-приложение для настройки агента.
- Подписанные IPK/APK-репозитории для поддерживаемых версий OpenWrt.

## Компоненты

```text
Браузер ──HTTPS──> RMM-сервер ──SQLite
                         │
                         ├── обработчики уведомлений
                         └── изолированный SSH tunnel-сервис
                                         ▲
OpenWrt-роутер ──исходящие HTTPS/SSH─────┘
  ├── rmm-agent-go-production
  ├── luci-app-rmm-agent
  └── luci-i18n-rmm-agent-ru (необязательно)
```

| Компонент | Назначение | Лицензия |
| --- | --- | --- |
| `server/`, `web/` | API, база данных, интерфейс и уведомления | AGPL-3.0-only |
| `deploy/tunnel/` | Ограниченный обратный SSH-туннель | AGPL-3.0-only |
| `agent/` | Go-агент, LuCI и пакеты OpenWrt | MIT |

## Развёртывание

Потребуются:

- Docker Engine и Compose v2;
- HTTPS reverse proxy, например NPMplus;
- wildcard DNS-запись и сертификат для облачного доступа к LuCI;
- отдельный SSH-ключ для туннелей роутеров.

Скопируйте пример конфигурации и следуйте инструкции по развёртыванию:

```sh
cp .env.example .env
docker compose -f compose.yaml -f compose.release.yaml pull
docker compose -f compose.yaml -f compose.release.yaml up -d
docker compose -f compose.yaml -f compose.release.yaml ps
```

Не запускайте прод с примерными секретами. Сначала создайте ключ туннеля и заполните
обязательные переменные окружения.

- [Развёртывание через Docker Compose](docs/docker-compose.md)
- [Настройка NPMplus](docs/npmplus.md)
- [Wildcard-доступ к роутерам в стиле KeenDNS](docs/keendns.md)
- [Архитектура](docs/architecture.md)

## Установка агента OpenWrt

Рекомендуемый способ — подписанный пакетный репозиторий:

- OpenWrt 24.10 и старше: IPK/opkg;
- OpenWrt 25.12 и новее: APK;
- OpenWrt 21.02, 22.03 и 23.05: уровень legacy-поддержки.

Установите агент и LuCI-приложение:

```sh
opkg install rmm-agent-go-production luci-app-rmm-agent
# или
apk add rmm-agent-go-production luci-app-rmm-agent
```

По умолчанию интерфейс LuCI остаётся английским. Русский язык устанавливается отдельным
стандартным пакетом:

```sh
opkg install luci-i18n-rmm-agent-ru
# или
apk add luci-i18n-rmm-agent-ru
```

Адреса репозиториев, ключи проверки и инструкции для архитектур находятся в документе
[Подписанный репозиторий OpenWrt](docs/package-repository.md).

## Разработка и проверки

```sh
go test ./...
go vet ./...
node --check web/app.js
docker compose config
```

Запуск сервера в явно включённом локальном режиме разработки:

```sh
RMM_INSECURE_DEV_MODE=true \
RMM_OPERATOR_PASSWORD='replace-with-a-long-development-password' \
go run ./server/cmd/rmm-server
```

Небезопасный режим разработки нельзя использовать на публичном сервере.

## Релизы

Сервер и агент версионируются независимо:

- `server-v*` публикует контейнеры сервера и tunnel-сервиса;
- `agent-v*` публикует устанавливаемые IPK/APK-пакеты;
- подробные описания релизов ведутся на английском в [CHANGELOG.md](CHANGELOG.md).

В GitHub Release загружаются только устанавливаемые пакеты. Подписанные индексы и
публичные ключи публикуются через GitHub Pages, а provenance хранится в GitHub
attestations. Подробнее: [политика релизов](RELEASES.md).

## Безопасность и лицензирование

Перед публикацией сервиса изучите [модель безопасности](docs/security.md). Перед каждым
обновлением серверной части создавайте резервную копию SQLite volume.

Облачная часть распространяется по `AGPL-3.0-only`, агент и LuCI-пакеты — по MIT.
Подробнее: [LICENSE.md](LICENSE.md) и [NOTICE.md](NOTICE.md).
