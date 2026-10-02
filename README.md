# Blog API

![CI](https://github.com/nikolaykonkin/go-blog-api/actions/workflows/ci.yml/badge.svg)

REST API для блог-платформы на Go: JWT-аутентификация, CRUD постов и комментариев, проверка прав автора. Слоистая архитектура Handler → Service → Repository, PostgreSQL, chi.

## Стек

- Go 1.26
- PostgreSQL 15
- [chi](https://github.com/go-chi/chi) — HTTP-роутер
- [golang-jwt/jwt/v5](https://github.com/golang-jwt/jwt) — JWT (HS256)
- `golang.org/x/crypto/bcrypt` — хеширование паролей
- Docker Compose — локальный PostgreSQL и Adminer

## Возможности

- Регистрация и вход с выдачей JWT (HS256). Срок жизни задается `JWT_EXPIRY_HOURS` (по умолчанию 24).
- CRUD постов и комментариев. Редактировать и удалять может только автор — иначе `403 Forbidden`.
- Валидация входных данных. Все ошибки валидации оборачиваются в единый sentinel `service.ErrValidation` и переводятся в `400 Bad Request`.
- Пагинация через query-параметры `limit` и `offset`. Значения по умолчанию и верхняя граница применяются в сервисном слое: `defaultLimit = 10`, `maxLimit = 100`, отрицательный `offset` превращается в `0`.
- Обогащение ответов данными автора (`PostResponse.Author`, `CommentResponse.Author`). Отдаются только `id` и `username`, без email — эти ответы доступны анонимным клиентам.
- Middleware: JWT (`RequireAuth`, `OptionalAuth`), логирование запросов, CORS с обработкой preflight (`OPTIONS` → `204`), recovery после паник. В `internal/middleware` также реализованы `RequestID`, `ContentTypeJSON` и `RateLimiter`; в `main.go` к роутеру подключены только `Recovery`, `CORS`, `Logger`.
- Автоматические миграции при старте (`database.Migrate`). Все запросы идемпотентные (`CREATE TABLE IF NOT EXISTS`), запуск при каждом старте безопасен.
- Конфигурация через переменные окружения. `.env` подхватывается, если файл есть; иначе берутся переменные окружения или дефолты. `JWT_SECRET` обязателен — приложение не стартует без него (см. «Конфигурация»).

## Структура проекта

```
go-blog-api/
├── cmd/api/main.go               # Точка входа: конфиг, БД, слои, маршруты
├── internal/
│   ├── model/                    # Модели и DTO + тесты
│   ├── handler/                  # HTTP-хендлеры + тесты (httptest + chi)
│   ├── service/                  # Бизнес-логика + тесты (мок-репозитории)
│   ├── repository/               # Работа с БД; interfaces.go описывает контракты
│   └── middleware/               # JWT, логирование, CORS, recovery, request ID, rate limiter
├── pkg/
│   ├── auth/                     # bcrypt + JWT + тесты
│   └── database/                 # Подключение к PostgreSQL и миграции
├── migrations/001_init_schema.sql
├── docker-compose.yml            # PostgreSQL + Adminer
├── .env.example
├── Makefile
└── go.mod
```

## Быстрый старт

```bash
# 1. Зависимости
go mod download

# 2. Конфигурация: скопировать пример и задать свой JWT_SECRET
#    Пример генерации секрета: openssl rand -hex 32
cp .env.example .env

# 3. PostgreSQL и Adminer
docker compose up -d

# 4. Приложение (миграции применяются автоматически при старте)
go run ./cmd/api
```

Одной командой через Makefile:

```bash
make dev
```

### Миграции и Docker

`docker compose up -d` поднимает PostgreSQL и Adminer. SQL из `./migrations/001_init_schema.sql` монтируется в `docker-entrypoint-initdb.d` и применяется **один раз — при первом создании тома** `postgres_data`.

Дополнительно `database.Migrate` в `cmd/api/main.go` идемпотентно применяет ту же схему при **каждом старте приложения**. Из-за `CREATE TABLE IF NOT EXISTS` он добавляет только новые таблицы и индексы; изменения существующих таблиц он не применит.

Если после правки `./migrations/001_init_schema.sql` нужно применить изменения с нуля:

```bash
docker compose down -v && docker compose up -d
```

## Конфигурация

Переменные окружения и их значения по умолчанию (из `loadConfig` в `cmd/api/main.go`):

| Переменная | По умолчанию | Назначение |
|---|---|---|
| `SERVER_HOST` | `0.0.0.0` | адрес HTTP-сервера |
| `SERVER_PORT` | `8080` | порт HTTP-сервера |
| `DB_HOST` | `localhost` | хост PostgreSQL |
| `DB_PORT` | `5432` | порт PostgreSQL |
| `DB_USER` | `bloguser` | пользователь БД |
| `DB_PASSWORD` | `blogpassword` | пароль БД |
| `DB_NAME` | `blogdb` | имя БД |
| `DB_SSLMODE` | `disable` | sslmode для `lib/pq` |
| `JWT_SECRET` | — (обязательна) | секрет подписи JWT; минимум 32 символа |
| `JWT_EXPIRY_HOURS` | `24` | срок жизни токена в часах |

> `JWT_SECRET` обязателен. `validateConfig` в `cmd/api/main.go` останавливает запуск, если переменная пустая, короче 32 символов или равна заглушке `change-this-secret-in-production-min-32-chars` (значение по умолчанию в `loadConfig`). Пример секрета: `openssl rand -hex 32`.

## API

Базовый префикс всех эндпоинтов — `/api`. Защищенные эндпоинты требуют заголовок `Authorization: Bearer <token>`.

### Публичные

| Метод | Путь | Описание |
|---|---|---|
| GET | `/api/health` | Проверка состояния сервиса |
| POST | `/api/register` | Регистрация пользователя |
| POST | `/api/login` | Вход, выдача JWT |
| GET | `/api/posts?limit=&offset=` | Список постов |
| GET | `/api/posts/{id}` | Пост по ID |
| GET | `/api/posts/{id}/comments?limit=&offset=` | Комментарии к посту |
| GET | `/api/posts/author/{authorID}` | Посты конкретного автора |
| GET | `/api/comments/{id}` | Комментарий по ID |

### Защищенные

| Метод | Путь | Описание |
|---|---|---|
| GET | `/api/profile` | Профиль текущего пользователя |
| POST | `/api/posts` | Создать пост |
| PUT | `/api/posts/{id}` | Обновить пост (только автор) |
| DELETE | `/api/posts/{id}` | Удалить пост (только автор) |
| POST | `/api/posts/{postId}/comments` | Добавить комментарий |
| PUT | `/api/comments/{id}` | Обновить комментарий (только автор) |
| DELETE | `/api/comments/{id}` | Удалить комментарий (только автор) |

### Примеры

```bash
# Регистрация
curl -X POST http://localhost:8080/api/register \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","email":"test@example.com","password":"password123"}'

# Вход
curl -X POST http://localhost:8080/api/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"password123"}'

# Создание поста
curl -X POST http://localhost:8080/api/posts \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"title":"My Post","content":"Post content"}'

# Список постов с пагинацией
curl "http://localhost:8080/api/posts?limit=5&offset=0"

# Комментарий: post_id берется из URL, не из тела запроса
curl -X POST http://localhost:8080/api/posts/1/comments \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"content":"Great post!"}'
```

## Как это устроено

### Слои

`Handler → Service → Repository`. Сервисы не зависят от `database/sql` напрямую — они принимают интерфейсы `repository.UserRepository`, `repository.PostRepository`, `repository.CommentRepository` (см. `internal/repository/interfaces.go`). Это позволяет подменять реализацию хранилища и тестировать бизнес-логику без реальной БД.

- **Handler** отвечает за HTTP: разбор запроса, коды ответов, единый формат ошибки (`errorResponse{Error string}`), обогащение ответов данными автора (`buildPostResponse`, `buildCommentResponse`).
- **Service** отвечает за правила: валидация, нормализация пагинации, проверка прав через `CanBeEditedBy`/`CanBeDeletedBy`.
- **Repository** отвечает только за SQL: параметризованные запросы с `$1…$n`, без конкатенации строк.

### Обработка ошибок

Все ошибки валидации оборачиваются в `service.ErrValidation`. Хендлеры через `errors.Is` переводят sentinel-ошибки в HTTP-статусы в одном месте — `mapServiceError` (`internal/handler/response.go`):

| Sentinel | HTTP |
|---|---|
| `ErrValidation` | 400 |
| `ErrPostNotFound`, `ErrCommentNotFound`, `ErrUserNotFound`, `ErrPostNotExists` | 404 |
| `ErrForbidden` | 403 |
| `ErrUnauthorized` | 401 |
| `ErrUserAlreadyExists` | 409 |
| `ErrInvalidCredentials` | 401 |
| (любая другая) | 500 |

### Безопасность

- Пароли хранятся как bcrypt-хеши, `bcryptCost = 12` (`pkg/auth/password.go`). Открытый пароль не попадает ни в БД, ни в JSON: на `User.Password` стоит тег `json:"-"`.
- SQL-инъекции исключены: все запросы в `user_repo.go`, `post_repo.go`, `comment_repo.go` параметризованы.
- JWT подписывается и проверяется **только HS256**. `ValidateToken` отвергает любой другой `token.Method` — защита от атаки alg confusion (`pkg/auth/jwt.go`).
- Сообщение об ошибке входа не различает «email не найден» и «неверный пароль»: оба случая дают `ErrInvalidCredentials` → `401`. Это защита от перебора email.
- `CommentCreateRequest.PostID` помечен `json:"-"`: ID поста берется только из URL маршрута, клиент не может подменить его телом запроса.
- `JWT_SECRET` обязателен: приложение не стартует, если переменная пустая, короче 32 символов или равна заглушке из `loadConfig` (`validateConfig` в `cmd/api/main.go`). Это исключает запуск в продакшене со «заводским» секретом.

### Пагинация

`parsePagination` в хендлере не падает на некорректных значениях: мусор в `limit`/`offset` превращается в `0`. `normalizePagination` в сервисном слое подставляет `defaultLimit = 10`, режет до `maxLimit = 100` и не допускает отрицательный `offset`. В ответе клиент видит именно примененные значения (`postsListResponse.Limit/Offset`), а не то, что он прислал.

### Middleware

Порядок применения — `appMiddleware.Chain(router.ServeHTTP, Recovery, CORS, Logger)` (`cmd/api/main.go`). Сначала `Recovery` (перехватывает паники на всех уровнях), затем `CORS` (отвечает `204` на `OPTIONS` до chi, иначе chi вернет `405`), затем `Logger`. `RequireAuth` навешивается на конкретные маршруты как `authMW.RequireAuth`.

В пакете также реализованы `RequestID`, `ContentTypeJSON` и `RateLimiter` (`internal/middleware/logging.go`); в `main.go` к роутеру они не подключены.

## Тестирование

```bash
go test ./...            # все тесты
go test -v ./...         # подробный вывод
go test -cover ./...     # покрытие по пакетам
go test -race ./...      # проверка гонок
```

Тесты идут полностью in-memory: реальные `service.*` и `handler.*` работают с `mockUserRepo`/`mockPostRepo`/`mockCommentRepo`, реализующими интерфейсы `repository`. PostgreSQL и Docker для тестов не нужны.

### Покрытие

```
go test -cover ./...
blog-api/cmd/api                 coverage: 0.0% of statements
blog-api/internal/handler        coverage: 82.0% of statements
blog-api/internal/middleware     coverage: 99.1% of statements
blog-api/internal/model          coverage: 83.3% of statements
blog-api/internal/repository     coverage: 0.0% of statements
blog-api/internal/service        coverage: 80.6% of statements
blog-api/pkg/auth                coverage: 90.5% of statements
blog-api/pkg/database            coverage: 0.0% of statements
```

Среднее по пяти тестируемым пакетам (`handler`, `middleware`, `model`, `service`, `auth`) — **≈87%**. Это не покрытие всего кода: `cmd/api`, `internal/repository` и `pkg/database` без автотестов.

```bash
go test -race ./...
?   blog-api/cmd/api                 [no test files]
ok  blog-api/internal/handler        15.223s
ok  blog-api/internal/middleware      2.151s
ok  blog-api/internal/model           3.202s
?   blog-api/internal/repository     [no test files]
ok  blog-api/internal/service        21.335s
ok  blog-api/pkg/auth                19.534s
?   blog-api/pkg/database            [no test files]
```

Гонок не обнаружено.

### Что покрыто

| Пакет | Покрытие | Что тестируется |
|---|---|---|
| `pkg/auth` | 90.5% | bcrypt (`HashPassword`, `CheckPassword`, salt), `ValidatePasswordStrength`, JWT (generate/validate, expired, invalid signature, malformed, `RefreshToken`, `GetUserIDFromToken`) |
| `internal/model` | 83.3% | `ToResponse` без утечки пароля в JSON, `CanBeEditedBy`/`CanBeDeletedBy`, `CommentCreateRequest.PostID` игнорируется при JSON-десериализации |
| `internal/service` | 80.6% | `UserService.Register/Login` (валидация, дубликаты, единая ошибка входа), `PostService`/`CommentService` CRUD, нормализация пагинации, `ErrForbidden` |
| `internal/middleware` | 99.1% | `RequireAuth`/`OptionalAuth`, `Recovery`, `CORS` + preflight, `Chain`, `getClientIP`, `RequestID`, `RateLimiter`, `responseWriter` |
| `internal/handler` | 82.0% | HTTP-статусы и тела: 201/200/400/401/403/404/409/204 для регистрации, входа, CRUD постов и комментариев; проверка, что `postId` берется из URL, а не из тела |
| `cmd/api`, `internal/repository`, `pkg/database` | 0.0% | Автотестов нет: `main` — склейка зависимостей, остальное — SQL-обертки. Проверялось вручную (`go run`, `curl`, `psql`), автоматических интеграционных тестов в проекте нет |

### CI

GitHub Actions (`.github/workflows/ci.yml`) на каждый `push` и `pull_request`:

1. `go vet ./...`
2. `go test -race -cover ./...`

Версия Go в CI — `1.26`, как и в `go.mod`.

## Команды

Все команды доступны через `make`:

| Команда | Что делает |
|---|---|
| `make help` | Список всех команд |
| `make run` | `go run cmd/api/main.go` |
| `make build` | Сборка бинарника в `build/blog-api` |
| `make test` | `go test -v ./...` |
| `make fmt` | `go fmt ./...` |
| `make deps` | `go mod download && go mod tidy` |
| `make docker-up` | Поднимает PostgreSQL и Adminer в Docker |
| `make docker-down` | Останавливает контейнеры |
| `make db-shell` | `psql` внутрь контейнера `blog_postgres` |
| `make dev` | Поднимает PostgreSQL и запускает приложение |
| `make clean` | Удаляет `build/` |