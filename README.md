# db-access

Сервис выдачи доступов к учебным базам `cu-sql` и `cu-db` на `pg-grm`. Работает на https://db.cu3rd.ru,
сами базы доступны на `db.cu3rd.ru:5432` (`sslmode=require`).

## Что видит посетитель

- При первом заходе браузер получает долгоживущую cookie с идентификатором из 8 символов `a-zA-Z0-9`.
  Он же становится логином в Postgres.
- Страница сама создаёт пользователя: только чтение обеих баз. Пароль показывается один раз и нигде не хранится.
- Галочка у каждой базы даёт write-доступ к этой базе на сутки: `INSERT`, `UPDATE`, `DELETE` и создание таблиц.
- Кнопка «Обновить логин и пароль» выдаёт новую пару, старая сразу перестаёт работать.

## Как устроена безопасность

- **Подпись cookie.** Логины видны всем пользователям базы в `pg_roles`. Поэтому рядом лежит вторая cookie
  `cu_login_sig` с HMAC: чужой логин не даёт действовать от чужого имени.
- **Создание роли.** Роль создаётся POST-запросом со страницы, а не на GET: сканеры и превью ссылок ролей не создают.
  Есть лимиты: `DBACCESS_CREDENTIALS_PER_IP_HOUR` на адрес и `DBACCESS_MAX_ACCOUNTS` всего.
- **Пароли.** В Postgres уходит только SCRAM-верификатор, открытый пароль не попадает в журналы сервера.
- **Конец write-доступа.** Закрываются сессии роли (они могли сделать `SET ROLE`), а созданные ею объекты
  переходят роли `cu_archive` без прав. Владельцу базы их отдавать нельзя: оставленная функция `SECURITY DEFINER`
  получила бы его права.
- **Изоляция баз.** `CONNECT` к обеим базам отозван у `PUBLIC`. Сессии ограничены: 5 подключений на роль,
  `statement_timeout` 60 с.

## Access-лог

Каждый запрос, кроме проб `/api/healthz`, пишется в таблицу `access_log` своей базы сервиса и в stdout (Loki):
время, IP, `X-Forwarded-For`, логин, метод, хост, путь, query, статус, размер, длительность, `User-Agent`, `Referer`,
`Accept-Language`. Тела запросов и ответов и cookie не пишутся. Записи старше 90 дней удаляются.

## Оформление

Тема повторяет `cu3rd` из Forgejo. Палитра и шрифты в `tailwind.config.ts` (значения из homelab `apps/forgejo/values.yaml`),
компоненты `.box`, `.btn`, `.message-warning` в `web_src/css/index.css`, логотип и фавикон в `public/img/`.

## Настройка

| Переменная | По умолчанию | Назначение |
|---|---|---|
| `DBACCESS_DB_DSN` | | своя база сервиса, выдаёт self-service |
| `DBACCESS_PG_PASSWORD` | | пароль роли `cu_provisioner` |
| `DBACCESS_COOKIE_SECRET` | | ключ подписи cookie, от 32 символов. При смене посетители теряют привязку к своим ролям |
| `DBACCESS_PG_HOST` | `pg-grm-rw.postgres.svc.cluster.local` | кластер учебных баз |
| `DBACCESS_DATABASES` | `cu-sql,cu-db` | выдаваемые базы |
| `DBACCESS_MAX_ACCOUNTS` | `5000` | лимит ролей |
| `DBACCESS_CREDENTIALS_PER_IP_HOUR` | `300` | лимит созданий и ротаций с одного адреса |

Роль `cu_provisioner` (`CREATEROLE`, член `cu_owner`) и `pg_hba` публичного пула описаны в homelab:
`postgres/cluster-grm.yaml`, `postgres/public-ysapunova.yaml`.

Сервис рассчитан на одну реплику: блокировка, связывающая роль и строку аккаунта, живёт в памяти процесса.

## Разработка

```sh
make build && DBACCESS_PG_PASSWORD=... DBACCESS_COOKIE_SECRET=... ./dbaccess web
docker compose up --build      # приложение и локальный кластер, http://localhost:3000
make lint test test-sqlite     # линтеры, юнит- и интеграционные тесты
make test-e2e                  # Playwright (один раз: make playwright)
# права на настоящем Postgres; тест создаёт базы и роли, нужен одноразовый кластер
TEST_PROVISION_SUPERUSER_DSN='postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable' make test-provision
```

Архитектура и соглашения: [AGENTS.md](AGENTS.md).
