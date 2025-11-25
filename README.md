# Сервис назначения ревьюеров (PR Reviewer Assignment Service)

Сервис, который назначает ревьюеров на PR из команды автора, позволяет выполнять переназначение ревьюверов и получать список PR’ов, назначенных конкретному пользователю, а также управлять командами и активностью пользователей.

## Запуск

Сервис и база данных поднимаются через make или docker-compose:

```bash
make up
# или
docker-compose up --build -d
```

Доступ к сервису по адресу:
`http://localhost:8080`

### Основные команды

| Команда | Описание |
|---------|----------|
| `make up` | Сборка и запуск контейнеров |
| `make down` | Остановка контейнеров |
| `make down-v` | Остановка с удалением volumes |
| `make logs` | Просмотр логов |
| `make test` | Запуск интеграционных тестов в изолированной среде |
| `make loadtest` | Запуск нагрузочного тестирования в изолированной среде|
| `make lint` | Проверка кода линтером |
| `make lint-fix` | Автоисправление линтером |

## Принятые решения

### users/getReview без 404
В OpenAPI спецификации для `GET /users/getReview` не описан ответ с кодом 404. `GET /users/getReview` возвращает пустой список если пользователь не найден или не назначен ревьювером.

### Опечатка в openapi.yml
```yml
application/json:
  schema:
    type: object
    required: [ pull_request_id, old_user_id ]
    properties:
      pull_request_id: { type: string }
      old_user_id: { type: string }
  example:
    pull_request_id: pr-1001
    old_reviewer_id: u2   <- old_reviewer_id заменил на old_user_id
```
---

## Дополнительные задания 

Каждый пункт выполнен:

### Эндпоинт статистики (`GET /stats`)
Возвращает:
- Количество команд, пользователей, PR
- Количество открытых/смерженных PR
- Статистика назначений пользователей
- Статистика ревьюверов

### Массовая деактивация (`POST /team/deactivate`)
Метод массовой деактивации пользователей команды

### Конфигурация линтера (`.golangci.yml`)
Конфиг, на основе Golden config:
```yml
  version: "2"

run:
  timeout: 3m
  go: "1.25"

formatters:
  enable:
    - goimports
    - golines

  settings:
    golines:
      max-len: 120

linters:
  enable:
    - asasalint
    - asciicheck
    - bidichk
    - bodyclose
    - canonicalheader
    - copyloopvar
    - cyclop
    - dupl
    - durationcheck
    - embeddedstructfieldcheck
    - errcheck
    - errname
    - errorlint
    - exhaustive
    - exptostd
    - fatcontext
    - forbidigo
    - funcorder
    - funlen
    - gocheckcompilerdirectives
    - gochecknoinits
    - gochecksumtype
    - gocognit
    - goconst
    - gocritic
    - gocyclo
    - godot
    - gomoddirectives
    - goprintffuncname
    - govet
    - iface
    - ineffassign
    - intrange
    - loggercheck
    - makezero
    - mirror
    - mnd
    - musttag
    - nakedret
    - nestif
    - nilerr
    - nilnesserr
    - nilnil
    - noctx
    - nolintlint
    - nonamedreturns
    - nosprintfhostport
    - perfsprint
    - predeclared
    - promlinter
    - protogetter
    - reassign
    - recvcheck
    - rowserrcheck
    - sloglint
    - spancheck
    - sqlclosecheck
    - staticcheck
    - testableexamples
    - testifylint
    - testpackage
    - tparallel
    - unconvert
    - unparam
    - unused
    - usestdlibvars
    - usetesting
    - wastedassign
    - whitespace

linters-settings:
  errcheck:
    check-type-assertions: true
    check-blank: true
  govet:
    check-shadowing: false
  gofmt:
    simplify: true

issues:
  exclude-rules:
    - path: ".*_test\\.go"
      linters:
        - gochecknoinits
        - gocritic
        - mnd
        - gocognit

  exclude-dirs:
    - "loadtest"
    
output:
  formats:
    colored-line-number: {}
  sort-results: true
```

### Интеграционное тестирование (`integration_test/`)
Все endpoints полностью покрыты тестами, проверяется идемпотентность merge

### Результаты нагрузочного тестирования

5000 запросов на операцию, 500 параллельных соединений
| Операция | RPS (запросов/сек) | Средняя задержка (ms) | Success Rate |
|:---|:---:|:---:|:---:|
| Создание PR | ~4461 | 108 ms | 100% |
| Чтение (Get Team) | ~20452 | 23 ms | 100% |
| Статистика | ~505 | 943 ms |  100% |
| Массовая деактивация| - | 14 ms | 100% |

---

## Структура проекта

```
├── cmd/server/main.go           # точка входа, HTTP server
├── internal/
│   ├── apierr/errors.go         # типы ошибок API
│   ├── handlers/handlers.go     # HTTP handlers
│   ├── models/models.go         # модели данных
│   ├── repo/repo.go             # слой БД
│   └── service/service.go       # бизнес-логика
├── migrations/                  # SQL миграции  
├── integration_test/            # интеграционные тесты
├── loadtest/                    # нагрузочное тестирование
├── .golangci.yml                # конфиг линтера
├── docker-compose.yml           # основной сервис (8080)
└── docker-compose.test.yml      # тестовый сервис (8081)
```

---

## Визуализация

## Архитектура системы

Проект реализован по принципам чистой архитектуры:
```mermaid
graph LR
    Client[HTTP Client] -->|JSON Request| Handler
    
    subgraph Application [Сервис]
        Handler[Handler Layer: <br/>Transport & Validation]
        Service[Service Layer: <br/>Business Logic]
        Repo[Repository Layer: <br/>Data Access]
    end
    
    Handler -->|Models| Service
    Service -->|Entities| Repo
    Repo -->|SQL| DB[(PostgreSQL)]

```

### Слои приложения

**Handler** (internal/handlers) - Transport Layer
- Парсинг HTTP запросов
- Валидация входных данных
- Маппинг ошибок в HTTP статусы

**Service** (internal/service) - Business Logic
- Бизнес-логика
- Проверка бизнес-правил

**Repository** (internal/repo) - Data Access
- SQL запросы
- Транзакции
- Работа с БД

### Диаграмма последовательности: Создание PR

Логика обработки запроса на создание PR:

```mermaid
sequenceDiagram
    participant Client
    participant Handler
    participant Service
    participant Repo
    participant DB

    Client->>Handler: POST /pullRequest/create
    Handler->>Service: CreatePullRequest(prID, name, authorID)
    
    Note over Service: 1. Валидация данных
    Service->>Repo: GetUser(authorID)
    Repo->>DB: SELECT FROM users WHERE user_id=?
    DB-->>Repo: User{team_name}
    Repo-->>Service: User (TeamID)
    
    Service->>Repo: PRExists(prID)
    Repo->>DB: SELECT EXISTS(...)
    DB-->>Repo: false
    Repo-->>Service: false
    
    Note over Service: 2. Поиск кандидатов
    Service->>Repo: GetActiveTeamMembers(TeamID, authorID)
    Repo->>DB: SELECT ... WHERE active=true AND user_id != author
    DB-->>Repo: [User2, User3, User5]
    Repo-->>Service: Candidates List
    
    Note over Service: 3. Выбор до 2-х
    
    Service->>Repo: CreatePR(PR + Reviewers)
    Repo->>DB: BEGIN TRANSACTION
    Repo->>DB: INSERT pr
    Repo->>DB: INSERT reviewers
    Repo->>DB: COMMIT
    DB-->>Repo: OK
    Repo-->>Service: Created Entity
    
    Service-->>Handler: PR Entity
    Handler-->>Client: 201 Created
```

---

## Схема базы данных

Используется PostgreSQL:

```mermaid
erDiagram
    TEAMS ||--o{ USERS : contains
    USERS ||--o{ PULL_REQUESTS : authors
    PULL_REQUESTS ||--o{ PR_REVIEWERS : assigned_to
    USERS ||--o{ PR_REVIEWERS : performs_review

    TEAMS {
        varchar team_name PK "Уникальное имя команды"
    }
    
    USERS {
        varchar user_id PK "Уникальный ID пользователя"
        varchar username "Имя пользователя"
        varchar team_name FK "Ссылка на команду"
        boolean is_active "Флаг доступности для ревью"
    }
    
    PULL_REQUESTS {
        varchar pull_request_id PK "Уникальный ID PR"
        varchar pull_request_name "Название PR"
        varchar author_id FK "Ссылка на автора"
        varchar status "OPEN или MERGED"
        timestamp created_at "Время создания"
        timestamp merged_at "Время слияния"
    }
    
    PR_REVIEWERS {
        varchar pull_request_id FK "Ссылка на PR"
        varchar user_id FK "Ссылка на ревьювера"
    }
```

- **PK** (Primary Key) — первичный ключ, уникальный идентификатор записи
- **FK** (Foreign Key) — внешний ключ, ссылка на запись в другой таблице

- Таблица `pr_reviewers` использует **составной PRIMARY KEY** из `(pull_request_id, user_id)`. Это гарантирует, что один пользователь не может быть назначен на один PR дважды, эти поля также являются внешними ключами

**Индексы:**
- `idx_users_team` на `users(team_name)` — для быстрого поиска участников команды
- PRIMARY KEY constraints автоматически создают индексы на всех ключевых полях

---

## Алгоритмы принятия решений

### Алгоритм назначения:

```mermaid
flowchart TD
    Start([Начало]) --> GetAuthor[Получить автора PR]
    GetAuthor --> GetTeam[Найти команду автора]
    GetTeam --> GetUsers[Получить активных участников команды]
    GetUsers --> Exclude[Исключить автора PR]
    Exclude --> Count{Кол-во кандидатов}
    
    Count -->|>= 2| Shuffle[Random shuffle списка]
    Shuffle --> Pick2[Взять первых 2]
    
    Count -->|== 1| Pick1[Взять единственного]
    
    Count -->|0| Pick0[Список пуст]
    
    Pick2 --> Save[Создать PR с ревьюверами]
    Pick1 --> Save
    Pick0 --> Save
    Save --> End([Конец: Вернуть PR])
```

### Логика замены ревьювера:

```mermaid
flowchart TD
    Start([Начало]) --> GetPR[Получить PR по ID]
    GetPR --> CheckMerged{Status == MERGED?}
    CheckMerged -->|Да| ErrorMerged[Ошибка: PR_MERGED]
    CheckMerged -->|Нет| CheckAssigned{old_user_id в ревьюверах?}
    CheckAssigned -->|Нет| ErrorNotAssigned[Ошибка: NOT_ASSIGNED]
    CheckAssigned -->|Да| GetTeam[Получить команду old_user_id]
    GetTeam --> GetCandidates[Найти активных в команде]
    GetCandidates --> ExcludeList[Исключить: автор + текущие ревьюверы]
    ExcludeList --> CheckCandidates{Есть кандидаты?}
    CheckCandidates -->|Нет| ErrorNoCandidate[Ошибка: NO_CANDIDATE]
    CheckCandidates -->|Да| PickRandom[Выбрать случайного]
    PickRandom --> Replace[Заменить в БД]
    Replace --> End([Конец: Вернуть обновленный PR])
    
    ErrorMerged --> EndError([Завершить с ошибкой])
    ErrorNotAssigned --> EndError
    ErrorNoCandidate --> EndError
```

### Жизненный цикл PR

```mermaid
stateDiagram-v2
    [*] --> OPEN : create with auto-assignment
    OPEN --> OPEN : reassign reviewer
    OPEN --> MERGED : merge
    MERGED --> MERGED : merge (idempotent)
    MERGED --> [*]
    
    note right of OPEN
        - Can reassign reviewers
        - Max 2 reviewers
        - Only active team members
    end note
    
    note right of MERGED
        - No modifications allowed
        - Timestamps frozen
        - Idempotent operation
    end note
```

---

## Стек технологий

| Компонент | Технология | Версия |
|-----------|------------|--------|
| Язык | Go | 1.25 |
| HTTP Router | chi | v5.0.10 |
| Database | PostgreSQL | 15-alpine |
| DB Driver | pgx/v5 | v5.5.0 |
| Migrations | golang-migrate | v4.16.2 |
| Linter | golangci-lint | 2.6.2 |
| Container | Docker + Docker Compose | - |

---

## Лицензия

MIT