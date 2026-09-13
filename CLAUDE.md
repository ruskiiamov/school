# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## О проекте

Учебный монолит — школьный электронный журнал. Модуль `github.com/ruskiiamov/school`,
Go 1.27. Сервер рендерит HTML на templ, интерактивность — HTMX, стили — Tailwind v4
(standalone-бинарник, без npm), данные — SQLite через `modernc.org/sqlite`.

Интерфейс полностью на русском; вёрстка должна быть одинаково пригодна на телефоне
и на десктопе. Готова итерация 1: вход и дашборд. Остальные разделы (классы, предметы,
учителя, ученики, расписание, журнал оценок) стоят в сайдбаре заглушками — см.
`view.NavItems`.

## Проектные документы

Требования и решения по итерациям 2+ лежат в `docs/`: процесс — `docs/README.md`,
журнал решений — `docs/decisions.md`, открытые вопросы — `docs/open-questions.md`,
доменная модель — `docs/domain.md`, план итераций — `docs/roadmap.md`. Перед
реализацией сверяться с ними; новое решение — новая запись в `decisions.md`.

## Команды

```sh
make tools     # скачать tailwindcss/htmx в bin/ и подключить templ как go-tool
make build     # generate + css + сборка bin/server (CGO_ENABLED=0)
make run       # build + запуск с config.yaml (CONFIG=... чтобы указать другой)
make watch     # templ --watch и tailwind --watch параллельно
make test      # generate + go test -race ./...
make lint      # generate + go tool golangci-lint run
make fmt       # go tool golangci-lint fmt
make check     # fmt --diff + lint + test (прогонять перед коммитом)
```

Один тест: `make generate` (один раз, если `*_templ.go` отсутствуют), затем
`go test ./internal/server -run TestLoginSuccessSetsSecureCookie`.

`golangci-lint` и `templ` подключены как инструменты проекта (блок `tool` в `go.mod`),
вызывать только через `go tool ...`. Глобальный `~/go/bin/golangci-lint` собран более
старым Go и на этом проекте падает.

## Генерируемые файлы

Не редактировать и не коммитить: `internal/view/**/*_templ.go`,
`internal/view/static/css/app.css`, `bin/`, `data/`, `logs/`, `config.yaml`
(шаблон — `config.example.yaml`). Правки идут в `.templ` и `css/input.css`;
`make generate` / `make css` пересобирают остальное. Tailwind сканирует классы
через `@source "../../../view"` в `input.css` — это значит и `.templ`, и
сгенерированные `_templ.go`.

## Архитектура

Зависимости идут строго в одну сторону: `cmd/server` → `internal/app` →
`internal/server` → `internal/auth` → `internal/storage`. `internal/view` не знает
ни о `server`, ни о `auth`; `storage` не знает о HTTP.

- `cmd/server/main.go` — флаг `-config` (или `CONFIG_PATH`), логгер, `signal.NotifyContext`,
  порядок закрытия ресурсов.
- `internal/app` — единственное место сборки зависимостей: открыть БД, применить
  миграции, создать `auth.Service`, гарантировать админа, собрать `http.Server`.
  `Run` держит три горутины в `errgroup`: сервер, уборка сессий, graceful shutdown
  по `ctx`.
- `internal/config` — YAML с дефолтами в `Load` и списком проверок в `validate`
  (все ошибки собираются через `errors.Join`). Профилей окружения (`app.env`,
  `dev`/`prod`) нет и не вводить: каждое поведение — отдельное явное поле конфига.
- `internal/server` — `Handler()` собирает два `http.ServeMux`: внешний со служебными
  маршрутами (статика, `/healthz`) и внутренний `pages()` с маршрутами приложения
  (`"GET /login"`, точное совпадение корня — `"GET /{$}"`), смонтированный под `"/"`
  как `chain(s.pages(), s.logRequests, s.recoverPanic)`. Внешний обёрнут
  `chain(mux, requestID, secureHeaders, s.crossOriginProtection)`. Порядок значим:
  `requestID` снаружи всего, потому что логгер — `contextHandler`, который достаёт
  `request_id` из контекста; `recoverPanic` внутри `logRequests`, чтобы паника попала
  в лог запроса. Новые страницы регистрировать в `pages()` — так они логируются
  автоматически; служебные маршруты без access-лога — на внешнем mux.
- `internal/auth` — сервисный слой: логин/логаут/аутентификация, bcrypt, серверные
  сессии, роли. Зависит от `storage` через локальные интерфейсы `userRepo`/`sessionRepo`
  (объявлены в `service.go`) — тесты подставляют свои реализации.
- `internal/storage` — репозитории на `database/sql`, `ErrNotFound` вместо
  `sql.ErrNoRows` наружу, миграции goose из `embed.FS`.
- `internal/view` — view-модели (`models.go`), данные для них (`data.go`), форматирование
  (`format.go`); templ-шаблоны в `layout/`, `pages/`, `components/`; статика в
  `static/` через `embed.FS`.

## Конвенции, важные при доработке

**Три отдельных типа пользователя.** `storage.User` (строка таблицы, с хешем пароля),
`auth.User` (домен, с типизированной `Role`), `view.User` (только то, что рисуется).
Конвертация на границах: `auth.toUser`, `server.toViewUser`. Не протаскивать
`storage.User` в шаблоны и не добавлять в `view.*` поля, которых не должно быть в HTML.
Русские подписи ролей — в `view.RoleTitle`, `auth.Role` знает только коды.

**Никаких внешних ключей в схеме.** В миграциях не использовать `FOREIGN KEY`,
`REFERENCES`, `ON DELETE CASCADE`, `CHECK`; прагма `foreign_keys` не включается.
Целостность держит сервисный слой: при каждой новой связи писать явную очистку
зависимых строк (образец — `SessionRepo.DeleteByUser`) и периодическую уборку
осиротевших записей (`Service.cleanupSessions` → `DeleteOrphaned`). Валидация
значений — в Go (`auth.ParseRole`), не в SQL.

**Время в БД — миллисекунды Unix** (`INTEGER`), через `toMillis`/`fromMillis`;
наружу отдаётся UTC. `created_at`/`updated_at` заполняют сами репозитории через
`time.Now()`, сервисный слой их не передаёт; доменные моменты вроде `ExpiresAt`
задаёт сервис.

**SQLite открывается с `SetMaxOpenConns(1)`**, WAL и `_txlock=immediate` —
писатель один, на это можно опираться, но не менять без причины.

**HTMX и редиректы.** `s.redirect` сам отличает HTMX-запрос (`HX-Request`) и отвечает
`HX-Redirect` + 204 вместо 303. Обработчики, отвечающие и фрагментом, и целой
страницей, ветвятся по тому же заголовку — образец `renderLoginError`. Рендер
только через `s.render` (ставит Content-Type и логирует ошибку рендера).

**CSP жёсткий** (`script-src 'self'`, без `unsafe-inline`) — никакого инлайнового JS
и inline-стилей в шаблонах; поведение живёт в `static/js/app.js` и в data-атрибутах.

**Статика версионирована**: ссылки в шаблонах только через `static.URL("css/app.css")`
→ `/static/<hash>/css/app.css`, где hash считается по содержимому embed-файлов при
старте. Совпавшая версия отдаётся с `immutable` на год, чужая — с `no-cache`.

**Формы защищены проверкой origin**, а не CSRF-токеном: весь mux обёрнут в
`http.CrossOriginProtection` (`s.crossOriginProtection` в `chain`), отдельные маршруты
оборачивать не нужно. Формы с `hx-post` обязаны иметь и `method="post" action="..."`,
чтобы работать без JS.

**Логирование** — `slog` в JSON, всегда `*Context`-варианты везде, где есть `ctx`
(и в `server`, и в `auth`), иначе потеряется `request_id`; ошибка как
`slog.Any("error", err)`. `request_id` кладёт в контекст middleware `requestID` через
`logger.WithRequestID`, а добавляет в записи `logger.NewContextHandler`, которым
обёрнут корневой логгер в `logger.New`. Access-лог пишется только для маршрутов
`pages()`; статика и `/healthz` зарегистрированы на внешнем mux и в лог не попадают.

**Ошибки** оборачиваются с контекстом (`fmt.Errorf("insert user: %w", err)`);
на границах сравнение через `errors.Is` с сентинелами (`storage.ErrNotFound`,
`auth.ErrInvalidCredentials`, `auth.ErrUnauthenticated`).

## Как добавить страницу

1. View-модель в `internal/view/models.go`, данные — в `data.go`.
2. Шаблон в `internal/view/pages/<name>.templ` поверх `layout.App` (страницы вне
   оболочки — поверх `layout.Base`).
3. Обработчик в `internal/server/<name>_handler.go`: достать пользователя
   `userFromContext`, собрать `view.*Page`, отдать через `s.render`.
4. Маршрут в `Server.pages()`, под `s.requireAuth`.
5. Пункт меню в `view.NavItems` — снять `Disabled` и задать `Href`.
6. Тест в `internal/server/server_test.go` через `newTestServer` (реальная БД в
   `t.TempDir()`, миграции, админ из конфига — моков HTTP нет).

## Стиль Go

Есть скилл `go-style` с соглашениями по коду — применять при написании и правке Go.
Линтеры и форматтеры заданы в `.golangci.yml` (`standard` + bodyclose, errorlint,
gosec, noctx, revive, rowserrcheck, sqlclosecheck и др.; goimports с
`local-prefixes: github.com/ruskiiamov/school` — импорты в три группы: stdlib,
внешние, локальные).
