# AGENTS.md

Go-сервер (chi + xorm, html/template) + htmx:
раскладка по слоям, Makefile как единая точка входа, YAML-фикстуры, интеграционные тесты без сокета
и e2e на Playwright, запускаемые из Go. Предметная область и запуск — `README.md`.

## Раскладка

- `main.go`, `cmd/` — CLI (`urfave/cli/v3`): `web` (по умолчанию), `migrate`. `bindata.go` (тег `bindata`)
  встраивает `templates/` и `public/` в бинарь; без тега они читаются с диска (`DBACCESS_STATIC_ROOT`),
  шаблоны и ассеты перечитываются на каждый рендер.
- `models/<feature>/` — xorm-структура + функции доступа (`account.GetByLogin(ctx, …)`),
  ошибки-типы с `Unwrap()` → `db.ErrNotExist` / `db.ErrInvalidArgument`. В пакете регистрация `db.RegisterModel`.
  Импорт в чужих пакетах — алиасом `account_model` (importas).
- `models/db/` — движок, `GetEngine(ctx)`, `WithTx(ctx, fn)` (вложенные вызовы переиспользуют транзакцию).
- `models/migrations/` — версионные миграции, только добавлять в конец. Миграция объявляет свою копию структуры
  и не импортирует `models/*` (depguard).
- `models/fixtures/<table>.yml` — фикстуры для всех уровней тестов. `models/unittest/` — харнесс:
  `MainTest(m)` в `TestMain` каждого пакета, `PrepareTestDatabase(t)` в начале каждого теста,
  `AssertExistsAndLoadBean` / `AssertNotExistsBean` / `AssertCount` (xorm игнорирует bool-поля в условиях бина).
- `services/context/` — `*context.Context` для хендлеров: `Data`, `HTML(status, "access/page")`, `IsHTMX()`,
  `Redirect` (303 или `HX-Redirect`), `NotFoundOrServerError` (404/422/500, тело — текст для тоста).
- `routers/web/web.go` — все маршруты; `routers/web/<feature>/` — хендлеры. Роутеры не трогают xorm (depguard).
  Мутации — POST (`/account`, `/account/rotate`, `/write`), CSRF — `http.CrossOriginProtection`.
- `templates/` — `*.tmpl`, имя = путь без расширения. `base/head` + `base/footer`, фрагменты фичи рядом.
- `web_src/js` (TypeScript, webpack + esbuild) → `public/assets/js/index.js`; `web_src/css` (Tailwind 3) →
  `public/assets/css/index.css`. `web_src/js/htmx.ts` — конфиг htmx и тосты на ошибки ответа/сети.
  Ассеты не раздаются отдельно: `InlineCSS`, `InlineJS`, `InlineLogo` встраивают их в страницу, лимит 22 КБ по сети.

## htmx-конвенции

- Каждая форма работает без JS (`method="post" action=…`) и с htmx (`hx-post`, ответ — изменённый фрагмент).
  Ответ с паролем не перенаправляет: пароль существует только в этом ответе. Ветвление — `ctx.IsHTMX()` (boosted-запросы считаются обычными).
- Ошибки 4xx/5xx не свопаются (`htmx.config.responseHandling`), текст ответа показывается тостом.

## Тесты (три уровня)

- Юнит: `make test-backend` (Go, все пакеты кроме `tests/…`), `make test-frontend` (vitest + happy-dom,
  `web_src/**/*.test.ts`).
- Интеграционные: `tests/integration/` — весь роутер в процессе через `httptest`; `NewRequest`,
  `NewRequestWithValues`, `AsHTMX`, `MakeRequest(t, req, status)`, `NewHTMLParser` (goquery).
  `make test-sqlite`, `make test-sqlite#TestRotate`, `make test-pgsql` (`TEST_PGSQL_DSN`).
- E2E: `tests/e2e/*.test.e2e.ts`. `make test-e2e` / `make test-e2e#access` — Go (`tests/e2e/e2e_test.go`)
  поднимает настоящий сервер на `localhost:3003` с фикстурами и запускает Playwright по файлу на сабтест.
  Фикстура `resetFixtures` в `utils_e2e.ts` перезагружает фикстуры перед каждым тестом через
  `POST /_test/reset-fixtures` (есть только в харнессе). Любая JS-ошибка на странице валит тест.
  `PLAYWRIGHT_PROJECT=firefox`; свой браузер — `PLAYWRIGHT_CHROMIUM_EXECUTABLE`.

## Провижининг

- `services/provision` — единственное место, меняющее роли и права; `Fake` — для тестов уровня выше.
  Любое изменение прав проверять `make test-provision` на настоящем Postgres: SQLite и Fake этого не покрывают.
- `services/access` держит роль и строку аккаунта согласованными и компенсирует сбой второго шага первым.

## Конвенции

- Перед коммитом: `make fmt lint test test-sqlite` (CI: `make pr-go`, `make pr-frontend`, `test-pgsql`, `test-e2e`).
  Новые проверки добавлять в Makefile, не в workflow.
- Go: gofumpt, golangci-lint v2 (`.golangci.yml`), логирование — `log/slog`, `fmt.Print*` запрещён.
- Новая фича: миграция → `models/<f>/` + фикстура + тесты модели → хендлеры в `routers/web/<f>/` + маршруты →
  шаблоны → интеграционные тесты → e2e.
- npm-версии точные (`.npmrc save-exact`).
