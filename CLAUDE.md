# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## О проекте

Учебный монолит — школьный электронный журнал. Модуль `github.com/ruskiiamov/school`,
Go 1.27. Сервер рендерит HTML на templ, интерактивность — HTMX, стили — Tailwind v4
(standalone-бинарник, без npm), данные — SQLite через `modernc.org/sqlite`.

Интерфейс полностью на русском; вёрстка должна быть одинаково пригодна на телефоне
и на десктопе. Готова итерация 1 (вход, дашборд); итерация 2 (справочники админа)
идёт по нумерованным срезам — что готово, см. `docs/roadmap.md`. Меню строится по роли в
`view.NavItems(role, active)`; пункты «Журнал» и «Дневник» пока заглушки
(`stub_handler.go`).

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
`internal/server` → {`internal/auth`, `internal/school`} → `internal/storage`.
`auth` и `school` друг о друге не знают; `internal/view` не знает ни о `server`,
ни о сервисах; `storage` не знает о HTTP.

- `cmd/server/main.go` — флаг `-config` (или `CONFIG_PATH`), логгер, `signal.NotifyContext`,
  порядок закрытия ресурсов.
- `internal/app` — единственное место сборки зависимостей: открыть БД, применить
  миграции, создать `auth.Service`, гарантировать админа, собрать `http.Server`.
  `Run` держит три горутины в `errgroup`: сервер, уборка сессий, graceful shutdown
  по `ctx`.
- `internal/config` — YAML с дефолтами в `Load` и списком проверок в `validate`
  (все ошибки собираются через `errors.Join`). Профилей окружения (`app.env`,
  `dev`/`prod`) нет и не вводить: каждое поведение — отдельное явное поле конфига.
  `timezone` проверяется через `time.LoadLocation` и отдаётся как `Config.Location`;
  `cmd/server` импортирует `time/tzdata`, чтобы статический бинарник не зависел
  от системных zoneinfo. `school.year_start_month` — номер месяца (1–12), год начинается с его первого числа.
- `internal/server` — `Handler()` собирает два `http.ServeMux`: внешний со служебными
  маршрутами (статика, `/healthz`) и внутренний `pages()` с маршрутами приложения
  (`"GET /login"`, точное совпадение корня — `"GET /{$}"`), смонтированный под `"/"`
  как `chain(s.pages(), s.logRequests, s.recoverPanic)`. Внешний обёрнут
  `chain(mux, requestID, secureHeaders, s.crossOriginProtection)`. Порядок значим:
  `requestID` снаружи всего, потому что логгер — `contextHandler`, который достаёт
  `request_id` из контекста; `recoverPanic` внутри `logRequests`, чтобы паника попала
  в лог запроса. Новые страницы регистрировать в `pages()` — так они логируются
  автоматически; служебные маршруты без access-лога — на внешнем mux.
  Маршруты для одной роли — `s.requireAuth(requireRole(role...)(h))`: аноним
  уходит на `/login`, чужая роль получает 404 (не 403) — образец `/journal` и
  `/diary` в `pages()`.
- `internal/auth` — сервисный слой: логин/логаут/аутентификация, bcrypt, серверные
  сессии, роли, управление пользователями (`users.go`: создание с генерацией
  логина `login.go` и пароля `password.go`, правка, смена пароля и
  деактивация с удалением сессий, списки с фильтром подстроки в Go — `lower()`
  в SQLite не складывает кириллицу). Зависит от `storage` через локальные
  интерфейсы `userRepo`/`sessionRepo` (объявлены в `service.go`); тесты идут
  через реальную БД. «Не найдено» — `auth.ErrNotFound`. Неактивный пользователь
  (`users.active = 0`) не входит и теряет сессию при следующем запросе.
- `internal/school` — справочники: предметы (`subject.go`), типы работ
  (`work_type.go`), классы (`class.go`), класс ученика (`student.go`); по D-038
  сюда же лягут нагрузка и дети родителя. Держит конкретные `*storage.*Repo`. Учебный год — не сущность (D-042):
  `Service.CurrentYear()` считает его по «сегодня» в `Location` и месяцу
  `year_start_month`, имя даёт `school.YearName`. «Не найдено» —
  `school.ErrNotFound`; ошибки ввода — `validation.Errors` (D-041), тексты
  сообщений живут в `school`.
- `internal/validation` — `Errors map[string]string` (реализует `error`) и
  `NormalizeSpaces`, общие для `auth` и `school`.
- `internal/storage` — репозитории на `database/sql`, `ErrNotFound` вместо
  `sql.ErrNoRows` наружу, миграции goose из `embed.FS`. Транзакции — внутри
  одного метода репозитория: все запросы через `tx`, потому что при
  `SetMaxOpenConns(1)` обращение к `db` изнутри транзакции повиснет.
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
задаёт сервис. Часовой пояс из конфига применяется только к «сегодня»;
учебный год — просто `int` (год начала, D-042).

**SQLite открывается с `SetMaxOpenConns(1)`**, WAL и `_txlock=immediate` —
писатель один, на это можно опираться, но не менять без причины.

**Формы справочников (D-041, D-043).** Простые справочники (предметы, типы
работ) — одна страница: форма добавления и строки текстом, «Изменить» ведёт на
тот же список с `?edit={id}` (`editingID`), и только эта строка рендерится
формой; сложные формы — отдельные страницы (D-035). Обработчик читает поля через `formValue`
(с `TrimSpace`), `id` из пути — через `pathID` (404 при мусоре), зовёт сервис
и ветвится: `formErrors(err)` → перерисовать страницу с введённым значением и
ошибкой у своей строки (`rowEdit`, строка остаётся в режиме правки), статус 200; иначе `s.handleServiceError`
(`ErrNotFound` → 404, прочее → `s.serverError` с логом и 500); успех —
`subjectsDone`: HTMX получает фрагмент списка (`pages.SubjectsList`), обычный
запрос — `s.redirect` на список с сохранением `?inactive=1`
(`catalogListURL`). Каждое действие строки — своя форма POST (деактивация, стрелки порядка) или
ссылка с `hx-get` («Изменить», «Отмена»); общие куски — `components.EditForm`,
`CreateForm`, `ActiveForm`, `EditLink`. Маршруты админа регистрируются через
`s.admin(h)`. Образец — `admin_subjects_handler.go` и
`pages/admin_subjects.templ`; общие компоненты — `components/form.templ`,
классы полей и кнопок — константы в `components/classes.go`. Классы (D-044)
идут по тому же строчному паттерну поверх вкладок лет: список и редиректы
берут год из `?year=` (`queryYear`, `classesListURL`), создание — всегда в
`CurrentYear()`, форма добавления только на вкладке текущего года
(`CanCreate`), карточка `/admin/classes/{id}` — отдельная страница;
образец — `admin_classes_handler.go`.

**Пользователи (D-045…D-048).** Три раздела
`/admin/{teachers|students|parents}` обслуживает один набор обработчиков в
`admin_users_handler.go`, параметризованный `userSection` (роль, путь,
русские подписи); маршруты регистрируются циклом по `userSections` в
`pages()`. Тот же строчный паттерн, что у предметов: строка добавления
сверху (`userFields`), `?edit={id}` рендерит одну строку формой с кнопкой
«Сменить пароль»; query списка (`q`, `class`, `inactive`) — `page.ListQuery`
в шаблоне и `rawListQuery` в редиректах. HTMX-фрагмент — `pages.UsersPage`,
контейнер `#users` включает шапку с переключателем «Показать удалённые»,
поле поиска помечено `hx-preserve`, иначе подмена теряет фокус и ввод.
`<select>` — `components.SelectClassFor` (своя галочка `.select-chevron`
в `input.css`, ширина как у кнопок).
Все `{id}`-маршруты идут через `sectionUser`: чужая роль и админ получают
404 — так «себя деактивировать нельзя» держится без отдельного кода. Логин
и пароль всегда генерирует `auth.CreateUser`, сброс — `ResetPassword`;
одноразовые логин и пароль живут в `credentialsStore`
(`created.go`) под ID сессии админа, 10 минут, `kind` меняет заголовок
страницы `/{id}/created`. Класс ученика — `school.SetStudentClass`,
проверка `CheckStudentClass` до создания пользователя.

**HTMX и редиректы.** `s.redirect` сам отличает HTMX-запрос (`isHTMX`) и отвечает
`HX-Redirect` + 204 вместо 303. Обработчики, отвечающие и фрагментом, и целой
страницей, ветвятся по тому же `isHTMX` — образец `renderLoginError`. Рендер
только через `s.render` (ставит Content-Type и логирует ошибку рендера).

**Валидация форм — только серверная.** На полях не ставить `required`,
`pattern` и подобное: браузерные подсказки выходят на языке браузера, а не
интерфейса. Пустые и неверные значения ловит сервис и возвращает
`validation.Errors` с русским текстом.

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
   оболочки — поверх `layout.Base`); страницы админа — `admin_<name>.templ`.
3. Обработчик в `internal/server/<name>_handler.go`: `s.shell(r, title, active)`
   собирает `view.Shell`, дальше собрать `view.*Page` и отдать через `s.render`.
   Заголовок страницы — `components.PageHeader`, пустое состояние —
   `components.EmptyState`, карточка — `components.CardClass`.
4. Маршрут в `Server.pages()`: под `s.requireAuth`, при ограничении по роли —
   ещё и `requireRole(...)`. Формы — по D-035 и D-041 (`docs/decisions.md`).
5. Пункт меню в `view.NavItems` для нужной роли (`Href` = `active`).
6. Тест в `internal/server/<name>_test.go` через `newTestEnv` из
   `testing_test.go` (реальная БД в `t.TempDir()`, миграции, админ из конфига,
   доступ к `env.school`/`env.auth`/`env.db` — моков HTTP нет). Пользователей
   других ролей создаёт `env.createUser`, вход — `env.loginAs`; редирект
   проверяет `assertRedirect`. Негативный тест на чужую роль обязателен.

## Стиль Go

Есть скилл `go-style` с соглашениями по коду — применять при написании и правке Go.
Линтеры и форматтеры заданы в `.golangci.yml` (`standard` + bodyclose, errorlint,
gosec, noctx, revive, rowserrcheck, sqlclosecheck и др.; goimports с
`local-prefixes: github.com/ruskiiamov/school` — импорты в три группы: stdlib,
внешние, локальные).
