# Переход production RMM на PostgreSQL

Эта инструкция рассчитана на существующий Compose-проект с SQLite, сервером
`0.12.3` и тем же томом `rmm-data`. Целевые версии: сервер и туннель `0.13.1`,
агент `0.9.0`. PostgreSQL 18 и инициализатор его TLS входят в тот же
Compose-проект. Имена проекта и тома RMM при переходе сохраняются.
Для Arcane используйте один файл `compose.postgres.yaml`: он содержит сервер,
туннель, PostgreSQL и инициализатор TLS. Сохраните прежнее имя проекта и
точные имена томов `RMM_DATA_VOLUME` и `RMM_TUNNEL_DATA_VOLUME`.
Конфигурация PostgreSQL встроена в Compose-файл; существующий публичный ключ
туннеля должен оставаться доступным по пути `RMM_TUNNEL_KEY_PATH` на хосте Arcane.

Если PostgreSQL уже запущен на `0.13.0`, перед обновлением сделайте дамп БД и
сохраните текущие настройки стека. Затем измените только
`RMM_RELEASE_VERSION=0.13.1` в том же проекте Arcane, загрузите новые образы
и пересоздайте сервисы. Томам оставьте прежние имена. Повторный импорт SQLite
не выполняется: сервер продолжит работать с заполненной PostgreSQL. После
обновления проверьте `/healthz`, список LAN-клиентов и логи `rmm-server`.

## 0. Проверить релизы и подготовить окно работ

Начинайте только после успешной публикации подписанных `agent-v0.9.0` и
`server-v0.13.1`. Последний должен содержать три образа с тегом `0.13.1`:
`openwrt-rmm-server`, `openwrt-rmm-tunnel` и
`openwrt-rmm-postgres-tls-init`. Релиз агента должен опубликовать подписанный
manifest и пакеты для нужных версий OpenWrt. До публикации этих тегов команды
ниже не выполняйте на production.

Запланируйте окно с остановкой записи. Если стек управляется Arcane или другим
GitOps-контроллером, приостановите его автоматическую сверку на всё окно.
Подготовьте в Arcane замену содержимого стека на `compose.postgres.yaml`,
переменные PostgreSQL добавьте в защищённые переменные стека.
Подготовьте место для полной копии `rmm-data`, снимка SQLite, PostgreSQL и
дампов. Убедитесь, что есть проверенный способ хранить резервные копии вне
сервера. Не запускайте второй RMM с production-томом.

Обновите checkout до релизного коммита. Сохраните прежние `.env` и конфигурацию
Compose в защищённом месте. До изменения версии проверьте фактический образ
запущенного сервера:

```sh
docker inspect "$(docker compose -f compose.yaml ps -q rmm-server)" --format '{{.Config.Image}}'
```

Если это не `0.12.3`, сначала проверьте совместимость реальной версии с
миграцией на отдельной копии тома. Затем в рабочем `.env` укажите:

```dotenv
RMM_RELEASE_VERSION=0.13.1
RMM_STABLE_AGENT_VERSION=0.9.0
RMM_POSTGRES_PASSWORD=<отдельный-сильный-пароль-приложения>
RMM_POSTGRES_ADMIN_PASSWORD=<другой-сильный-пароль-администратора-БД>
```

Не меняйте `RMM_DATA_VOLUME`, `RMM_TUNNEL_DATA_VOLUME`, адреса, порты и ключи.
Пароли не добавляйте в Git, логи или командную строку. До остановки сервиса
проверьте и заранее загрузите образы:

```sh
docker compose -f compose.postgres.yaml config --quiet
docker compose -f compose.postgres.yaml pull rmm-server tunnel-ssh postgres postgres-tls-init
docker pull alpine:3.23
```

## 1. Остановить запись и сделать исходную копию

Закройте внешний доступ к RMM на прокси на время переключения. Сначала узнайте
фактические имена томов у запущенных контейнеров; не подставляйте примерные
имена:

```sh
rmm_container="$(docker compose -f compose.yaml ps -q rmm-server)"
tunnel_container="$(docker compose -f compose.yaml ps -q tunnel-ssh)"
rmm_volume="$(docker inspect "$rmm_container" --format '{{range .Mounts}}{{if eq .Destination "/data"}}{{.Name}}{{end}}{{end}}')"
tunnel_volume="$(docker inspect "$tunnel_container" --format '{{range .Mounts}}{{if eq .Destination "/data"}}{{.Name}}{{end}}{{end}}')"
printf 'RMM volume: %s\nTunnel volume: %s\n' "$rmm_volume" "$tunnel_volume"
test -n "$rmm_volume" && test -n "$tunnel_volume"
docker compose -f compose.yaml stop rmm-server tunnel-ssh
```

Скопируйте **остановленные** тома целиком. В `rmm-data` должны остаться база,
возможные `rmm.db-wal` и `rmm.db-shm`, ключ подписи команд и ключ шифрования:

```sh
stamp="$(date -u +%Y%m%dT%H%M%SZ)"
backup_dir="$PWD/backups/postgres-cutover-$stamp"
install -d -m 0700 "$backup_dir"
docker run --rm --network none -v "$rmm_volume:/source:ro" -v "$backup_dir:/backup" alpine:3.23 \
  sh -c 'tar -C /source -czf /backup/rmm-data.tgz .'
docker run --rm --network none -v "$tunnel_volume:/source:ro" -v "$backup_dir:/backup" alpine:3.23 \
  sh -c 'tar -C /source -czf /backup/tunnel-data.tgz .'
(cd "$backup_dir" && sha256sum rmm-data.tgz tunnel-data.tgz > SHA256SUMS && sha256sum -c SHA256SUMS)
tar -tzf "$backup_dir/rmm-data.tgz" | grep -E 'rmm.db|data-encryption.key|command-signing-ed25519.pem'
```

Проверьте, что оба ключа и база присутствуют. Сохраните копию за пределами
хоста. Эти архивы содержат секреты; защищайте их шифрованием и доступом.
Не удаляйте исходные Docker-тома.

## 2. Репетиция на копии тома

Этот этап можно провести в отдельное окно заранее на такой же остановленной
копии. Импорт проверяет схему, внешние ключи, идентификаторы ключей, данные и
числа строк; при ошибке PostgreSQL остаётся без частично импортированной схемы.

Создайте отдельный тестовый том и Compose-проект. Тестовый сервер доступен
только через `127.0.0.1:18091`; туннель не запускается, SMTP и Telegram
отключены, чтобы копия production-данных не отправляла уведомления:

```sh
rehearsal="rmm-rehearsal-$stamp"
rehearsal_volume="${rehearsal}-data"
docker volume create "$rehearsal_volume"
docker run --rm --network none -v "$rehearsal_volume:/data" -v "$backup_dir:/backup:ro" alpine:3.23 \
  sh -c 'tar -C /data -xzf /backup/rmm-data.tgz'
rehearsal_compose() {
  RMM_DATA_VOLUME="$rehearsal_volume" RMM_HTTP_PORT=18091 \
  RMM_SMTP_HOST='' RMM_TELEGRAM_BOT_TOKEN='' \
    docker compose -p "$rehearsal" -f compose.postgres.yaml "$@"
}
rehearsal_compose up -d rmm-server
rehearsal_compose ps
rehearsal_compose logs --tail 100 rmm-server
curl --fail --silent --show-error http://127.0.0.1:18091/healthz
```

Ожидайте в логе `existing SQLite data imported into PostgreSQL` и ответ
`{"status":"ok"}`. Проверьте количество пользователей и устройств и запись
об импорте, затем сравните их с исходной системой:

```sh
rehearsal_compose exec -T rmm-server sh -c \
  'PGSSLMODE=verify-full PGSSLROOTCERT=/run/postgres-tls/ca.crt psql -h postgres -U rmm -d rmm -Atc "SELECT (SELECT count(*) FROM users), (SELECT count(*) FROM devices), (SELECT count(*) FROM database_imports), (SELECT ssl FROM pg_stat_ssl WHERE pid=pg_backend_pid())"'
rehearsal_compose down
```

Последнее поле должно быть `t`, число записей в `database_imports` — `1`.
`down` здесь выполняется **без** `-v`: тестовые тома с production-данными
остаются для проверки и должны храниться под теми же ограничениями доступа.
Если репетиция не прошла, оставьте production на SQLite и разберите ошибку.

## 3. Переключить production

Проверьте, что старые контейнеры RMM и туннеля по-прежнему остановлены, а
внешний доступ закрыт. Запустите сервер с PostgreSQL:

```sh
docker compose -f compose.postgres.yaml up -d rmm-server
docker compose -f compose.postgres.yaml ps
docker compose -f compose.postgres.yaml logs --tail 100 rmm-server
curl --fail --silent --show-error http://127.0.0.1:18080/healthz
```

Первый старт создаст PostgreSQL, TLS-сертификаты и базу, затем автоматически
перенесёт SQLite. Исходный `rmm.db` и WAL останутся на томе, а рядом появится
`rmm.db.pre-postgres-<sha>.db` — самостоятельная копия для восстановления.
Если импорт или проверка ключей не прошли, сервер не начнёт обслуживать запросы;
перейдите к разделу отката.

Сверьте пользователей, устройства, критичные очереди и TLS-подключение:

```sh
docker compose -f compose.postgres.yaml exec -T rmm-server sh -c \
  'PGSSLMODE=verify-full PGSSLROOTCERT=/run/postgres-tls/ca.crt psql -h postgres -U rmm -d rmm -Atc "SELECT (SELECT count(*) FROM users), (SELECT count(*) FROM devices), (SELECT count(*) FROM commands), (SELECT count(*) FROM database_imports), (SELECT ssl FROM pg_stat_ssl WHERE pid=pg_backend_pid())"'
```

Убедитесь, что вход оператора, список устройств, алерты, архивы роутеров и
скачивание снимка БД работают. Затем запустите туннель и откройте внешний
доступ. Дождитесь heartbeat нескольких роутеров и проверьте одну безопасную
команду и удалённую LuCI-сессию:

```sh
docker compose -f compose.postgres.yaml up -d tunnel-ssh
docker compose -f compose.postgres.yaml ps
```

## 4. Включить резервирование PostgreSQL и обновить агентов

Сделайте первый дамп сразу после проверки. Скрипт создаёт `pg_dump` в custom
format, проверяет его через `pg_restore --list` и записывает SHA-256:

```sh
bash scripts/backup-postgres.sh --dry-run "$backup_dir"
bash scripts/backup-postgres.sh "$backup_dir"
```

Путь к дампу напечатан скриптом. Проверьте восстановление в **отдельную** БД
тестового проекта (переменная и функция `rehearsal_compose` из шага 2 должны
быть доступны в текущем shell):

```sh
dump_file='<путь-к-напечатанному-rmm-postgres-YYYYMMDDTHHMMSSZ.dump>'
rehearsal_compose up -d rmm-server
rehearsal_compose exec -T postgres createdb -U postgres -O rmm rmm_restore_check
rehearsal_compose exec -T rmm-server sh -c \
  'PGSSLMODE=verify-full PGSSLROOTCERT=/run/postgres-tls/ca.crt pg_restore -h postgres -U rmm -d rmm_restore_check --no-owner --no-acl' \
  < "$dump_file"
rehearsal_compose exec -T rmm-server sh -c \
  'PGSSLMODE=verify-full PGSSLROOTCERT=/run/postgres-tls/ca.crt psql -h postgres -U rmm -d rmm_restore_check -Atc "SELECT (SELECT count(*) FROM users), (SELECT count(*) FROM devices)"'
rehearsal_compose down
```

Сравните числа строк с production. `createdb` безопасно завершится ошибкой,
если тестовая БД с таким именем уже есть; для повторной проверки используйте
другое новое имя. Найдите имена TLS-томов в остановленном или работающем
контейнере и включите их в защищённое резервирование:

```sh
tls_container="$(docker compose -f compose.postgres.yaml ps -a -q postgres-tls-init)"
docker inspect "$tls_container" --format '{{range .Mounts}}{{println .Name .Destination}}{{end}}'
```

Сохраните дамп вместе с исходным архивом `rmm-data`, TLS-томами и отдельной
защищённой копией ключей вне production-хоста. Настройте регулярный запуск
скрипта в существующем планировщике резервирования. Например, задание cron
для каталога, доступного только оператору резервного копирования:

```cron
15 2 * * * cd /srv/rmm-openwrt && /usr/bin/env bash scripts/backup-postgres.sh /secure/rmm-backups >> /var/log/rmm-postgres-backup.log 2>&1
```

Замените пути на свои и добавьте контроль неуспешного завершения и срок
хранения в используемой системе резервирования. `pg_restore --list`
проверяет формат, а шаг выше проверяет фактическое восстановление. Не копируйте работающий
`postgres-data` как обычный архив файлов без согласованной процедуры остановки
или физического резервирования.

Только после стабильной работы сервера обновляйте роутеры на подписанный
`rmm-agent-go-production` `0.9.0` через штатный feed/managed rollout. Агент
`0.8.0` можно оставить на время наблюдения. Обновите также
`luci-app-rmm-agent` до `0.2.3` (и установленный русский перевод), чтобы новый
параметр появился в интерфейсе. Для нужного режима задайте в LuCI
или UCI heartbeat `30` секунд и проверку интернета `300` секунд:

```sh
uci set rmm-agent.main.interval_seconds='30'
uci set rmm-agent.main.connectivity_check_interval_seconds='300'
uci commit rmm-agent
/etc/init.d/rmm-agent restart
```

После обновления проверьте фактическую версию агента, регулярные heartbeat и
время последней проверки связи. Новая опция имеет default `300` секунд даже
если старый конфигурационный файл пакета был сохранён при обновлении.

## Откат

Откат до открытия внешнего доступа использует сохранённый SQLite без обратного
импорта. Остановите RMM и туннель, верните прежний `.env` с версией `0.12.3`
и прежние файлы Compose, затем пересоздайте только эти сервисы из старых
образов:

```sh
docker compose -f compose.postgres.yaml stop rmm-server tunnel-ssh
# Здесь восстановите прежние .env и Compose-конфигурацию.
docker compose -f compose.yaml pull rmm-server tunnel-ssh
docker compose -f compose.yaml up -d --force-recreate rmm-server tunnel-ssh
docker compose -f compose.yaml ps
```

Не удаляйте PostgreSQL или TLS-тома во время разбора ошибки. После открытия
внешнего доступа PostgreSQL уже может содержать новые heartbeat, команды и
другие записи. Их нет в исходной SQLite: откат к ней после начала traffic
означает потерю этих изменений и требует отдельного решения оператора. Перед
таким откатом сохраните дамп PostgreSQL и зафиксируйте момент переключения.
