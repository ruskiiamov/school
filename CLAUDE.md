# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## О проекте

Учебный монолит — школьный электронный журнал. Модуль `github.com/ruskiiamov/school`,
Go 1.27. Сервер рендерит HTML на templ, интерактивность — HTMX, стили — Tailwind v4
(standalone-бинарник, без npm), данные — SQLite через `modernc.org/sqlite`.

Интерфейс полностью на русском; вёрстка должна быть одинаково пригодна на телефоне
и на десктопе. Готовы итерации 1 (вход, дашборд), 2 (справочники админа),
3 (журнал учителя), 4 (дневник), 5 (домашнее задание), 6 (сводные
представления) и 7 (перевод классов и прошлые годы); следующей в
`docs/roadmap.md` нет. Меню строится по роли в
`view.NavItems(role, active)`; заглушек больше нет.

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
`internal/server` → {`server/admin`, `server/account`, `server/journal`,
`server/diary`, `server/files`} → `server/web` → {`internal/auth`,
`internal/school`, `internal/journal`} → {`internal/storage`,
`internal/files`}. `auth`, `school` и `journal` друг о друге не знают;
`internal/view` не знает ни о `server`, ни о сервисах; `storage` и `files`
не знают о HTTP.

- `cmd/server/main.go` — флаг `-config` (или `CONFIG_PATH`), логгер, `signal.NotifyContext`,
  порядок закрытия ресурсов.
- `internal/app` — единственное место сборки зависимостей: открыть БД, применить
  миграции, создать сервисы, гарантировать админа, собрать `http.Server`.
  `Run` держит четыре горутины в `errgroup`: сервер, уборка сессий
  (`session.cleanup_interval`), уборка осиротевших строк журнала
  (`journal.cleanup_interval`, `journal.RunCleanup`), graceful shutdown по `ctx`.
- `internal/config` — YAML с дефолтами в `Load` и списком проверок в `validate`
  (все ошибки собираются через `errors.Join`). Профилей окружения (`app.env`,
  `dev`/`prod`) нет и не вводить: каждое поведение — отдельное явное поле конфига.
  `timezone` проверяется через `time.LoadLocation` и отдаётся как `Config.Location`;
  `cmd/server` импортирует `time/tzdata`, чтобы статический бинарник не зависел
  от системных zoneinfo. `school.year_start_month` — номер месяца (1–12), год начинается с его первого числа.
  `journal.cleanup_interval` — период уборки осиротевших строк журнала.
  `files` — каталог файлов ДЗ (`dir`, создаётся при старте), лимиты
  `max_file_size_mb`, `max_per_lesson` и `transfer_timeout` — на сколько
  обработчики загрузки и скачивания продлевают дедлайн соединения через
  `http.ResponseController` (глобальные `http.*_timeout` не трогать).
- `internal/server` — корень HTTP (D-057): `Server` держит `*web.Base`, сервисы и
  обработчики подпакетов. `Handler()` собирает два `http.ServeMux`: внешний со
  служебными маршрутами (статика, `/healthz`) и внутренний `pages()` с маршрутами
  приложения (точное совпадение корня — `"GET /{$}"`), смонтированный под `"/"`
  как `chain(s.pages(), s.logRequests, s.recoverPanic)`. Внешний обёрнут
  `chain(mux, requestID, secureHeaders, s.crossOriginProtection)`. Порядок значим:
  `requestID` снаружи всего, потому что логгер — `contextHandler`, который достаёт
  `request_id` из контекста; `recoverPanic` внутри `logRequests`, чтобы паника попала
  в лог запроса. В корне остались middleware (`middleware.go`) и дашборд
  (`home.go`, подписи карточек ролей — `sectionNote`); `pages()`
  регистрирует их и зовёт `Routes(mux)` подпакетов — так все страницы
  логируются автоматически; служебные маршруты без access-лога — на
  внешнем mux.
  - `server/web` — общий инструментарий страниц, единственное место, где HTTP
    знает про сессии и оболочку: `Base` (сервис `auth`, имя школы, cookie,
    логгер) с методами `Render`, `Shell`, `ServerError`, `HandleServiceError`
    (`ErrNotFound` → 404), `Authenticate`, `SessionID`, `SetSessionCookie`,
    `ClearSessionCookie`, `RequireAuth`; свободные `Redirect`, `IsHTMX`,
    `RequireRole`, `PathID`, `PathValue`, `FormValue`, `FormInt64`,
    `FormErrors`, `UserFromContext`; период сводок — `Period`,
    `ParsePeriod(r, today, min, max)` (две даты `from`/`to`, по умолчанию
    месяц «сегодня», зажим в границы года), `PeriodForm` строит
    `view.PeriodForm` с ссылками по месяцам (`period.go`); учебный год —
    `year.go`: `YearSelection(r, school)` (годы `school.ClassYears` —
    годы классов, текущий и следующий; `?year=` вне списка → текущий,
    селект показывается всегда),
    `WithYear` добавляет `year` в скрытые параметры ссылок, `YearPeriod`
    зажимает период в границы выбранного года с якорем «сегодня» или
    началом прошлого года. `HandleServiceError` превращает в 404
    `ErrNotFound` всех трёх сервисов и `journal.ErrForbidden`. Обработчиков
    в `web` нет и не добавлять.
  - `server/admin` — всё под `/admin`: `Handler` (`handler.go`: `New`,
    `Routes`, обёртка `h.admin(fn)` = `RequireAuth` + `RequireRole(admin)`,
    `activeUser`), по файлу на раздел (`classes.go`, `class_card.go`,
    `subjects.go`, `work_types.go`, `users.go`, `parent_children.go`,
    `password_reset.go`, `substitutions.go`, `transfer.go` — перевод
    классов `/admin/classes/transfer`, обычная форма без HTMX), общие для
    строчных списков `catalog.go` (`rowEdit`, `editingID`, `catalogListURL`)
    и одноразовые пароли `created.go`.
  - `server/account` — вход, выход и свой пароль (`login.go`, `password.go`);
    `Routes` сам оборачивает `/account/password` в `RequireAuth` +
    `RequireRole` трёх ролей.
  - `server/journal` — журнал учителя под `/journal`: `handler.go` (`New`,
    `Routes`, обёртка `h.teacher(fn)`, форма пары и последние уроки),
    `lesson.go` (страница урока, тема, удаление, `renderLesson` с режимами
    `renderPage`/`renderTopic`/`renderBlock`/`renderActions`/`renderRecord`/
    `renderHomework`), `summary.go` (`/journal/summary` учителя и
    `/admin/journal/summary` админа: `pairOptions` по году, `gridView` с
    ссылкой на урок через параметр, `cellText`; год — `web.YearSelection`),
    `marks.go` (оценки и сборка
    блока `#lesson`: `lessonBlock`, `studentPanel`, `markFields`),
    `records.go` (отсутствие и комментарий), `homework.go` (ДЗ: текст и
    срок, потоковая загрузка файлов через `r.MultipartReader` с
    `http.MaxBytesReader`, удаление файла, `homeworkView`, статус
    `homeworkStatus`), `admin.go` (`/admin/journal` и
    `/admin/journal/lessons/{id}` только на чтение под обёрткой `h.admin`,
    D-066 — исключение из «всё под `/admin` в `admin`», как `/admin/diary`;
    свои шаблоны `pages.AdminJournal`/`AdminLesson`, список учеников
    `lessonStudentItem` общий с учителем). Год и «сегодня» берёт из
    `school` (`CurrentYear`, `Today`) и передаёт в `journal` параметрами.
  - `server/diary` — дневник: `handler.go` (`/diary` для ученика и
    родителя: `index`, `parentDiary` с вкладками детей через `selectChild`
    (404 на чужого ребёнка), общий `renderDiary(diaryView)` и `diaryURL`,
    который тянет скрытые параметры `child`/`q` через все ссылки),
    `admin.go` (`/admin/diary?q=&year=&class=` поиск ученика по ФИО и
    состав класса выбранного года — `adminFilter` тянет `q`/`year`/`class`
    в ссылки, `/admin/diary/{id}` тот же дневник, `adminStudent(year)` —
    404 не ученику, класс в заголовке за выбранный год; D-061 —
    исключение из «всё под `/admin` в `admin`»), `summary.go`
    (`/diary/summary` и `/admin/diary/{id}/summary` — оценки по предметам
    за период с селектом года, `renderMarks`, чипы оценок ведут на день
    дневника). Только чтение, фрагменты `#diary` и `#marks`.
  - `server/files` — `GET /files/{id}`: скачивание файла ДЗ для всех ролей
    с проверкой доступа в обработчике (`allowed`: админ всегда, учитель —
    `LessonForTeacher`, ученик — `LessonVisibleToStudent`, родитель — через
    детей из `school.Children`), `Content-Disposition: attachment`,
    `http.ServeContent`.
  - `server/servertest` — окружение для тестов (см. «Как добавить страницу»);
    `env.Files` — хранилище файлов во временном каталоге с лимитами 1 МБ и
    3 файла на урок.
  Маршруты для одной роли — `base.RequireAuth(web.RequireRole(role...)(h))`
  на каждом маршруте внутри своего пакета: аноним уходит на `/login`, чужая
  роль получает 404 (не 403) — образец `h.teacher` в `server/journal`,
  `h.owner` и `h.admin` в `server/diary`.
- `internal/auth` — сервисный слой: логин/логаут/аутентификация, bcrypt, серверные
  сессии, роли, управление пользователями (`users.go`: создание с генерацией
  логина `login.go` и пароля `password.go`, правка, смена пароля и
  деактивация с удалением сессий, списки с фильтром подстроки в Go — `lower()`
  в SQLite не складывает кириллицу). Зависит от `storage` через локальные
  интерфейсы `userRepo`/`sessionRepo` (объявлены в `service.go`); тесты идут
  через реальную БД. «Не найдено» — `auth.ErrNotFound`. Неактивный пользователь
  (`users.active = 0`) не входит и теряет сессию при следующем запросе.
- `internal/school` — справочники: предметы (`subject.go`), типы работ
  (`work_type.go`), классы (`class.go`), состав класса (`student.go`,
  `StudentClassIn` по году), нагрузка (`assignment.go`), замены
  (`substitution.go`), дети родителя (`parent.go`), перевод классов
  (`transfer.go`: `CanTransfer`, `TransferPlan` — активные классы
  `текущий − 1`, `NextClassName` (+1 к ведущему числу), `Graduating` у
  строки плана — только «ведущее число 11, галочка снята»; ученики с
  флагами активности и «уже в классе»; `Transfer` сверяет ввод с планом,
  невключённые классы пропускает, ключи ошибок
  `name-{id}`/`students-{id}`/`form`, пишет одной транзакцией
  `ClassRepo.Transfer`; колонка `classes.graduating` в схеме есть, но не
  используется — D-071), число учеников по классам года
  (`ClassSizes`), счётчики дашборда (`stats.go`), общая проверка
  названий (`name.go`). Классы из репозитория идут в естественном
  порядке (`ORDER BY CAST(name AS INTEGER), name`), `journal.Pairs`
  держит тот же порядок. Держит конкретные `*storage.*Repo`. Учебный год — не
  сущность (D-042): `Service.CurrentYear()` (`year.go`) считает его по
  «сегодня» в `Location` и месяцу `year_start_month`, имя даёт
  `school.YearName`; `Service.Today()` — календарная дата «сегодня» в
  UTC-полночь (`DateOf`). «Не найдено» — `school.ErrNotFound`; ошибки
  ввода — `validation.Errors` (D-041), тексты сообщений — константы `msg*`
  рядом с проверкой (в `school`, `auth`, `journal`, а для роли и
  активности пользователя из формы — в `server`).
- `internal/journal` — журнал: пары «класс, предмет» учителя из нагрузки и
  действующих замен (`pair.go`), уроки с правами D-031 (`lesson.go`:
  `OpenLesson` создаёт или открывает, `LessonForTeacher`, `UpdateTopic`,
  `DeleteLesson` только пустой, `RecentLessons`), оценки и список учеников
  урока по D-023 (`mark.go`: `LessonStudents`, `AddMark`/`UpdateMark`/
  `DeleteMark`), записи об уроке (`record.go`: `SaveRecord`, пустая запись
  удаляет строку), уборка осиротевших строк (`cleanup.go`), чтение для
  дневника (`diary.go`: `DayLessons` — уроки дня ученика с его оценками,
  отсутствием, комментарием и ДЗ), сводки (`summary.go`: `TeacherGrid`
  по видимым учителю урокам пары, `PairGrid` для админа, `StudentSummary`
  по предметам; среднее считается в сервисе, форматирует
  `view.FormatAverage`), домашнее задание (`homework.go`:
  `Homework` — строка `homework` плюс файлы по `lesson_id`, `SaveHomework`
  удаляет строку при пустых тексте и сроке, `AddHomeworkFile` пишет на
  диск через `files.Store` и только потом строку, `DeleteHomeworkFile` —
  сначала строку, потом файл, `OpenHomeworkFile`, `LessonVisibleToStudent`;
  лимиты — `FileLimits`). Времени не
  считает: год и «сегодня» приходят параметрами (Q-31 → D-058). Держит свои
  `*storage.*Repo`; ФИО учеников не знает — их подставляет обработчик из
  `auth.Users`. «Нет доступа» — `journal.ErrForbidden`, «не найдено» —
  `journal.ErrNotFound`; обработчик на оба отвечает 404.
- `internal/validation` — `Errors map[string]string` (реализует `error`),
  `NormalizeSpaces`, `NormalizeLines` (многострочный текст: пробелы в
  строках схлопнуты, не больше одной пустой строки подряд),
  `ParseDate`/`DateLayout` (`YYYY-MM-DD`, как у
  `<input type="date">`), общие для `auth`, `school` и `journal`.
- `internal/files` — `Store` поверх каталога: `Save(reader, maxSize)`
  пишет во временно открытый файл со случайным hex-именем, определяет
  `Content-Type` по первым 512 байтам и возвращает `ErrTooLarge` при
  превышении (файл удаляется), `Open`, `Delete`, `IDs` для уборки; чужие
  имена (не 32 hex-символа) — `ErrNotFound`, так что обход каталога
  невозможен.
- `internal/storage` — репозитории на `database/sql`, `ErrNotFound` вместо
  `sql.ErrNoRows` наружу, миграции goose из `embed.FS`. Транзакции — внутри
  одного метода репозитория: все запросы через `tx`, потому что при
  `SetMaxOpenConns(1)` обращение к `db` изнутри транзакции повиснет.
  Календарные даты — `TEXT` `YYYY-MM-DD` через `toDate`/`fromDate`
  (`time.go`), открытый конец замены — NULL (`toNullDate`); в Go —
  `time.Time` в UTC-полночь.
- `internal/view` — view-модели (`models.go`), данные для них (`data.go`), форматирование
  (`format.go`); templ-шаблоны в `layout/`, `pages/`, `components/`; статика в
  `static/` через `embed.FS`.

## Конвенции, важные при доработке

**Три отдельных типа пользователя.** `storage.User` (строка таблицы, с хешем пароля),
`auth.User` (домен, с типизированной `Role`), `view.User` (только то, что рисуется).
Конвертация на границах: `auth.toUser`, `web.toViewUser` (внутри `Shell`). Не протаскивать
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

**Формы справочников (D-041, D-043, D-072, D-073).** Простые справочники (предметы, типы
работ) — одна страница: форма добавления и строки текстом, вся строка —
ссылка на тот же список с `?edit={id}` (`editingID`), и только эта строка
рендерится формой; сложные формы — отдельные страницы (D-035). Кнопок в
строке нет (D-072, D-073): строка — `components.RowLink` на `?edit={id}` с
карандашом справа (у классов строка ведёт на карточку, а на `?edit=` —
`EditIconLink`; у типов работ стрелки порядка стоят перед ссылкой), а
действия живут в форме правки: «Сохранить», `CancelLink` (`data-cancel`),
«Удалить»/«Восстановить» — `components.ActiveButton` (`formaction`, у
`EditForm` через `{ children... }`), удаление с подтверждением —
`components.DeleteButton` (`formaction` + `hx-post` + `hx-confirm` на
самой кнопке). Ошибка активации держит строку в режиме правки
(`rowEdit.open`). Так устроены предметы, типы работ, классы, пользователи,
замены и оценки на странице урока. Обработчик читает поля через `web.FormValue`
(с `TrimSpace`), `id` из пути — через `web.PathID` (404 при мусоре), зовёт сервис
и ветвится: `web.FormErrors(err)` → перерисовать страницу с введённым значением и
ошибкой у своей строки (`rowEdit`, строка остаётся в режиме правки), статус 200; иначе `h.base.HandleServiceError`
(`ErrNotFound` → 404, прочее → `h.base.ServerError` с логом и 500); успех —
`subjectsDone`: HTMX получает фрагмент списка (`pages.SubjectsList`), обычный
запрос — `web.Redirect` на список с сохранением `?inactive=1`
(`catalogListURL`). Стрелки порядка — своя форма POST; общие куски —
`components.EditForm`, `CreateForm`, `RowLink`, `ActiveButton`,
`DeleteButton`, `CancelLink`, `EditIconLink`. Маршруты админа регистрируются в
`admin.Routes` через `h.admin(fn)`. Образец — `admin/subjects.go` и
`pages/admin_subjects.templ`; общие компоненты — `components/form.templ`,
классы полей и кнопок — константы в `components/classes.go`. Классы (D-044)
идут по тому же строчному паттерну поверх вкладок лет: список и редиректы
берут год из `?year=` (`queryYear`, `classesListURL`), создание — всегда в
`CurrentYear()`, форма добавления только на вкладке текущего года
(`CanCreate`), карточка `/admin/classes/{id}` — отдельная страница;
образец — `admin/classes.go`.

**Пользователи (D-045…D-048, D-056).** Три раздела
`/admin/{teachers|students|parents}` обслуживает один набор обработчиков в
`admin/users.go`, параметризованный `userSection` (роль, путь,
русские подписи); маршруты регистрируются циклом по `userSections` в
`admin.Routes`. Тот же строчный паттерн, что у предметов: строка добавления
сверху (`userFields`), строка — `RowLink` на `?edit={id}`, который рендерит
её формой (поля в том же порядке, что в обычной строке: ФИО, класс, логин;
«Удалить»/«Восстановить» — `ActiveButton` в форме); query списка
(`q`, `class`, `inactive`) — `page.ListQuery`
в шаблоне и `rawListQuery` в редиректах. HTMX-фрагмент — `pages.UsersPage`,
контейнер `#users` включает шапку с переключателем «Показать удалённые»,
поле поиска помечено `hx-preserve`, иначе подмена теряет фокус и ввод.
`<select>` — `components.SelectClassFor` (своя галочка `.select-chevron`
в `input.css`, ширина как у кнопок).
Все `{id}`-маршруты идут через `sectionUser`: чужая роль и админ получают
404 — так «себя деактивировать нельзя» держится без отдельного кода. Логин
и пароль всегда генерирует `auth.CreateUser`, сброс — `ResetPassword` со
страницы «Сброс пароля» `/admin/password-reset`
(`admin/password_reset.go`, D-056): поиск по ФИО по всем трём
`userSections`, `resettableUser` даёт 404 админу и неизвестному ID,
подтверждение — `hx-confirm` с ФИО в вопросе. Одноразовые логин и пароль
живут в `credentialsStore` (`admin/created.go`) под ID сессии админа, 10 минут;
`take` требует свой `kind`, так что страница `/{id}/created` раздела
показывает только созданного, а `/admin/password-reset/{id}/created` —
только сброс. Класс ученика — `school.SetStudentClass`,
проверка `CheckStudentClass` до создания пользователя.

**Состав класса, нагрузка, дети родителя (D-049).** Карточка класса
(`admin/class_card.go`) — два самостоятельных HTMX-блока
`pages.ClassStudents` (`#class-students`) и `pages.ClassAssignments`
(`#class-assignments`); успех — фрагмент блока или редирект на карточку,
ошибка формы — блок (HTMX) или вся карточка со статусом 200. Формы
показываются только при `classEditable` (активный класс текущего года);
сервис держит то же правило через `CheckStudentClass`. Имена учеников и
учителей обработчик берёт из `auth.Users` и соединяет по ID в Go —
`school` не знает `auth`; роль и активность пользователя из формы
проверяет `activeUser` (`admin/handler.go`) и превращает в `validation.Errors`,
класс и предмет проверяет `school`. Учитель на пару (класс, предмет) —
`AssignmentRepo.Upsert`. Дети родителя — в строке правки раздела
«Родители» (`admin/parent_children.go`): `renderUsers` при роли
`parent` грузит `childrenData` и для строки в `?edit=` собирает
`view.ChildrenBlock` с живым поиском `?child=`; действия несут `edit` и
`child` в query (`childrenValues`), чтобы строка не выходила из режима
правки. Уборка осиротевших связей не нужна: ничего не удаляется.

**Свой пароль и дашборд (D-050, D-051).** `/account/password`
(`account/password.go`) — для учителя, ученика и родителя, админу
404 (его пароль из конфига); пункт «Сменить пароль» — последний в
`view.NavItems` этих ролей. `auth.ChangePassword` проверяет текущий пароль,
длину нового (8 символов…72 байта) и повтор, затем
`SessionRepo.DeleteByUserExcept` оставляет только текущую сессию (ID — из
cookie). Ошибка: полная страница с введёнными значениями
(`PasswordPage.Values`), а для HTMX — `pages.PasswordErrors`: только
`hx-swap-oob`-фрагменты ошибок под полями (`components.FieldErrorSlot`),
`<input>` не пересоздаются, иначе Chrome считает исчезновение полей
успехом и предлагает сохранить пароль (D-054); красная рамка — CSS
`:has()` в слое utilities `input.css`. Успех — редирект на `?done=1`. Класс с учениками не деактивируется
(D-052): `SetClassActive(false)` возвращает `validation.Errors`, обработчик
рисует ошибку у строки.
Дашборд (`server/home.go`): админу `school.Stats` (классы, ученики в
классах, предметы текущего года через `COUNT` в репозиториях) плюс
`auth.CountActiveUsers(RoleTeacher)` → `view.AdminStats` (карточки-ссылки)
и баннер `NoClasses`; остальным — `view.SectionItem(role)` карточкой.
Константы с «password» в имени ловит gosec G101 — называть по полю
(`msgNewTooShort`).

**Журнал учителя (D-058, D-059).** `/journal`: один `<select name="pair">`
со значением `{class}-{subject}` (`pairValue`/`parsePair`) и дата (по
умолчанию `school.Today()`), «Открыть» → `OpenLesson` → редирект на
`/journal/lessons/{id}`; под формой последние 15 видимых уроков; форма и
список — фрагмент `#journal`, ошибка формы под HTMX подменяет только его. Страница
урока: тема — форма `#lesson-topic` (HTMX подменяет только её), блок
`#lesson` — слева ученики (на телефоне чипы с `view.ShortName`, на
десктопе колонка с ФИО) с краткими отметками («5, 4 · Н»), справа панель
выбранного `?student={id}` (неизвестный — первый по ФИО): оценки с правкой
в строке `?mark={id}`, форма «Поставить», кнопка-переключатель
«Отсутствовал» в строке с ФИО (сохраняет сразу, ответ — `#lesson`) и
поле комментария с автосохранением по ходу набора (ответ — только
`#lesson-record`, режим `renderRecord` по заголовку `HX-Target`;
`hx-preserve` с ID ученика в `id`, иначе текст переехал бы к соседу),
обе формы несут скрытым полем текущее значение друг друга; тема тоже
сохраняется по ходу набора (`#lesson-topic`, `hx-preserve`), кнопок
«Сохранить» у темы и комментария нет (D-064);
у ученика «не в классе» форм нет, но оценки правятся и удаляются. Все
действия панели отвечают HTMX фрагментом `#lesson`, без JS — редирект на
`?student={sid}`; ссылки учеников — `hx-get` + `hx-push-url`. Кнопка
«Удалить урок» и ошибка удаления — фрагмент `#lesson-actions` в шапке:
форма целится в него, а `pages.LessonBlockUpdate` отдаёт его вместе с
`#lesson` через `hx-swap-oob`, чтобы кнопка пропадала после первой оценки.
Замены (`admin/substitutions.go`) — строчный паттерн с `?ended=1` и
правкой только дат; учителя проверяет `activeUser`, класс и предмет —
`school.CreateSubstitution`.

**Домашнее задание (D-065).** На странице урока сворачиваемый
`<details id="lesson-homework">` (`pages.LessonHomework`) между темой и
`#lesson`: свёрнут на полной странице, раскрыт в HTMX-ответе и при ошибке
формы (`homeworkView(..., open)`); в `<summary>` статус «не задано» /
«задано» / «к 18.09.2026 · 2 файла». Текст и срок — одна форма с
автосохранением (`hx-preserve` на полях), файлы — `<input type="file"
multiple>` в `<label>`-кнопке с `hx-trigger="change"` и
`hx-encoding="multipart/form-data"`, без кнопки «Загрузить» (без JS файлы
не загрузить, как и комментарий); у каждого файла своя форма удаления с
`hx-confirm`. Все три POST отвечают HTMX фрагментом `#lesson-homework`,
без JS — редиректом на урок. Файлы на диске — `internal/files`, ID
случайный hex и он же имя файла; скачивание только через `/files/{id}`.
Удаление урока (`LessonRepo.DeleteEmpty`) стирает `homework` и
`homework_files` в одной транзакции, файлы с диска — сервис после.
Уборка `CleanupOrphans` чистит строки без урока, файлы без строки и
логирует строки без файла. Срок ДЗ — только информация, списков «по
сроку» нет. В дневнике ДЗ — блок под комментарием урока и метка «ДЗ» в
списке уроков дня.

**Сводки (D-067, D-068).** `/journal/summary` (учитель), `/admin/journal/summary`
(админ), `/diary/summary` (ученик, родитель), `/admin/diary/{id}/summary`
(админ): GET-формы с `hx-trigger="submit, change"` и `hx-push-url`,
фрагменты `#summary` и `#marks`. Период зажат в выбранный учебный год
(`web.YearPeriod`); селект `year` (годы классов, текущий и следующий)
показывается всегда, и `year` всегда идёт в ссылки периода и вкладок
(`web.WithYear`). Сводка учителя без пар в году показывает форму
с селектом и текстом, а не пустое состояние. Таблица сводки — `overflow-x-auto` с закреплённым
первым столбцом (`sticky left-0`), даты столбцов «16.09», ячейка — ссылка
на урок с учеником. Четвертей и весов нет.

**Перевод классов (D-068).** Кнопка «Создать классы из прошлого года» на
вкладке текущего года в «Классах» (`CanTransfer`), страница
`/admin/classes/transfer` — форма без HTMX: на каждый класс прошлого года
поля `transfer-{id}`, `name-{id}`, `students-{id}` (чекбоксы; у 11-х
«Переводить» снята с подписью «выпускной»); обработчик собирает
`school.TransferInput` по плану (`transferInputFromForm`), ошибка —
полная страница со введённым, статус 200, успех — редирект на список
текущего года. Пометки «выпуск» нет (D-071). Вкладки «Классов» —
`ClassYears` (годы классов, текущий, следующий); на вкладке следующего
года ни формы добавления, ни кнопки перевода.

**Дневник (D-061, D-062).** `GET /diary?date=&child=&lesson=` для ученика
и родителя, `GET /admin/diary/{id}?date=&lesson=&q=` для админа — один
рендер `renderDiary(diaryView)`: форма даты (GET, «Показать») и ссылки
«Вчера/Сегодня/Завтра», у родителя вкладки детей, слева уроки дня (на
телефоне чипы), справа выбранный урок с оценками, «Отсутствовал» и
комментарием. Всё в фрагменте `#diary` с `hx-push-url`; форм, меняющих
состояние, нет. Чужое (ребёнок, урок, не ученик) — 404 до рендера; в
тестах хэш-ссылки с `&` сравнивать через `html.EscapeString`.

**HTMX и редиректы.** `web.Redirect` сам отличает HTMX-запрос (`web.IsHTMX`) и отвечает
`HX-Redirect` + 204 вместо 303. Обработчики, отвечающие и фрагментом, и целой
страницей, ветвятся по тому же `IsHTMX` — образец `renderLoginError` в `account`. Рендер
только через `base.Render` (ставит Content-Type и логирует ошибку рендера).
Формы с полями пароля при ошибке не пересоздают `<input>` (D-054).

**Валидация форм — только серверная.** На полях не ставить `required`,
`pattern` и подобное: браузерные подсказки выходят на языке браузера, а не
интерфейса. Пустые и неверные значения ловит сервис и возвращает
`validation.Errors` с русским текстом.

**CSP жёсткий** (`script-src 'self'`, без `unsafe-inline`) — никакого инлайнового JS
и inline-стилей в шаблонах; поведение живёт в `static/js/app.js` и в data-атрибутах:
`data-toggle-password` (глаз у `components.PasswordInput`), `hx-confirm` +
`data-confirm-ok` (окно `components.ConfirmDialog` `#confirm` в `layout.App`,
`app.js` перехватывает `htmx:confirm`), `data-state` у сайдбара,
`data-edit-form` на форме правки строки + `data-cancel` на «Отмена»
(клик по неинтерактивному месту вне `<li>` строки или Escape в форме
нажимает «Отмена»). Обработчики
делегированы на `document`, чтобы работать после HTMX-подмен (D-053).

**Статика версионирована**: ссылки в шаблонах только через `static.URL("css/app.css")`
→ `/static/<hash>/css/app.css`, где hash считается по содержимому embed-файлов при
старте. Совпавшая версия отдаётся с `immutable` на год, чужая — с `no-cache`.

**Формы защищены проверкой origin**, а не CSRF-токеном: весь mux обёрнут в
`http.CrossOriginProtection` (`s.crossOriginProtection` в `chain`), отдельные маршруты
оборачивать не нужно. Формы с `hx-post` обязаны иметь и `method="post" action="..."`,
чтобы работать без JS.

**Логирование** — `slog` в JSON, всегда `*Context`-варианты везде, где есть `ctx`
(и в `server` с подпакетами, и в `auth`), иначе потеряется `request_id`; ошибка как
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
3. Обработчик — метод `Handler` своего подпакета (`server/admin/<name>.go`,
   `server/account/<name>.go`; новый раздел роли — новый подпакет с
   `Handler`, `New`, `Routes` по образцу `account/handler.go`):
   `h.base.Shell(r, title, active)` собирает `view.Shell`, дальше собрать
   `view.*Page` и отдать через `h.base.Render`. Заголовок страницы —
   `components.PageHeader`, пустое состояние — `components.EmptyState`,
   карточка — `components.CardClass`.
4. Маршрут в `Routes(mux)` подпакета: под `h.base.RequireAuth`, при
   ограничении по роли — ещё и `web.RequireRole(...)` (в `admin` — обёртка
   `h.admin`). Новый подпакет монтируется в `Server.pages()` вызовом
   `Routes`. Формы — по D-035 и D-041 (`docs/decisions.md`).
5. Пункт меню в `view.NavItems` для нужной роли (`Href` = `active`).
6. Тест рядом с обработчиком, во внешнем пакете (`package admin_test`),
   через `servertest.New(t)` (реальная БД в `t.TempDir()`, миграции, админ из
   конфига, доступ к `env.School`/`env.Auth`/`env.DB` — моков HTTP нет; из
   внутреннего теста `servertest` не импортировать — цикл через `server`).
   Пользователей других ролей создаёт `env.CreateUser`, вход —
   `env.LoginAs` и `servertest.Login`; запросы — `servertest.Get`/`PostForm`,
   редирект проверяет `servertest.AssertRedirect`, cookie ответа —
   `servertest.Cookies`. Негативный тест на чужую роль обязателен.

## Стиль Go

Есть скилл `go-style` с соглашениями по коду — применять при написании и правке Go.
Линтеры и форматтеры заданы в `.golangci.yml` (`standard` + bodyclose, errorlint,
gosec, noctx, revive, rowserrcheck, sqlclosecheck и др.; goimports с
`local-prefixes: github.com/ruskiiamov/school` — импорты в три группы: stdlib,
внешние, локальные).
