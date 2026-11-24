---
name: go-coding-rules
description: 'Mandatory code conventions for this Go/Gin/GORM project — layer responsibilities (handler = HTTP, service = logic, repository = all queries), GenericRepository vs custom query decision guide, N+1 prevention (Preload/JOIN/subquery), errors (CustomError/FieldError instead of new sentinels), transactions, logging, notifications, access control, background work (s.runAsync), compact-code and migration rules. Load whenever writing, modifying or reviewing Go code here, including small edits. For the step order of a new feature, see go-development-guide.'
---

## When to use me

Use this skill when:

- Writing or modifying Go code — follow rules below, see `references/` for examples
- Adding features to existing codebase — ensure consistency with project conventions
- Reviewing code — check for anti-patterns and rule violations
- Need reference for helpers, packages, or GenericRepository methods

---

# Go Coding Rules

> **READ THIS ENTIRELY before writing any code.** These rules are MANDATORY.

## Service Topology

These rules default to the **fullservice** architecture (local auth, Notification). Some projects are a **microservice** running behind an api-gateway instead. Before applying Auth/Access or Notification sections below, check which topology applies:

| Signal | Fullservice | Microservice |
| ------ | ----------- | ------------ |
| `internal/middleware/gateway_auth.go` exists | No | Yes |
| Local `User`/`Role`/`Permission` models + login endpoint | Yes | No — identity comes from api-gateway headers |
| `internal/models/notification.go` or similar exists | Usually | Only if explicitly added |

Sections below marked **"Microservice Variant"** describe the deviation. If a project has neither the models/dirs above, the section is fullservice-only — skip it rather than forcing the pattern in.

## Project Identity

| Item        | Value                              |
| ----------- | ---------------------------------- |
| Module      | `github.com/reshap0318/go-boilerplate` (fullservice) — **check `go.mod` for the actual module name**, microservices use their own |
| Go Version  | 1.25.0+                            |
| Framework   | Gin, GORM, JWT, Redis, bcrypt      |
| Database    | MySQL (default) / PostgreSQL       |
| Entry Point | `cmd/api/main.go`                  |

## Architecture

```
Routes → Handlers → Services → Repositories → Database/Redis
              ↑
         Middleware (JWT, CORS, Rate Limit)
```

All layers use **Dependency Injection** via `internal/di/container.go`.

## Layer Responsibilities (STRICT)

Each layer has ONE job. Putting code in the wrong layer is a rule violation, not a style choice.

| Layer | Does | Never does |
| ----- | ---- | ---------- |
| **Handler** | Bind JSON / query params, validate, call ONE service method, map error → response via `HandleError` | Business rules, DB access, calling repositories |
| **Service** | Business logic: rules, permission/scope decisions, orchestration of repository calls, transactions (`TxManager`), hashing, tokens, cache invalidation, notifications, logging, DTO conversion | Building SQL: no `s.repo.X.DB...`, no `tx.Where/Joins/Raw/Table/Select/Group` |
| **Repository** | ALL database queries: generic methods + custom methods for JOIN / subquery / aggregate / partial select / bulk update | Business decisions (permission checks, "if role == admin"), logging, notifications, DTOs |

The only gorm call allowed in a service is `tx.Model(&x).Association("Rel").Append/Replace/Clear(...)` for many-to-many writes **inside** a `TxManager` transaction. Everything else that touches the DB goes through a repository method — if no method fits, **add one to the repository**, don't inline the query in the service.

```go
// WRONG — query built in the service
func (s *Services) UserGetAll(...) {
    ids := []uint{}
    s.repo.User.DB.Table("user_has_roles").Where("role_id = ?", roleID).Pluck("user_id", &ids)
    ...
}

// RIGHT — service decides, repository queries
func (s *Services) UserGetAll(ctx context.Context, opts *repositories.QueryOptions, roleID uint) (...) {
    if roleID > 0 {
        result, err = s.repo.User.FindAllByRoleWithOpts(nil, roleID, opts) // subquery lives in the repo
    } else {
        result, err = s.repo.User.FindAllWithOpts(nil, opts)
    }
    ...
}
```

## Code Examples

All examples are in `references/` folder:

- `references/model.go` — Model with TableName()
- `references/dto.go` — Request/Response DTOs
- `references/repository.go` — Repository registration + custom repo
- `references/service.go` — CRUD methods with logging + notification
- `references/handler.go` — HTTP handlers with validation
- `references/route.go` — Routes with permission middleware

> Reference files import `github.com/reshap0318/go-boilerplate/...` (the fullservice module). Never copy that path literally — substitute the current project's actual module name from `go.mod`.

---

## CRITICAL RULES

### 1. Shared base files — change only on purpose

- `internal/repositories/00_generic.go`
- `internal/repositories/00_transaction.go`

Every repository depends on these. Do NOT edit them as a side effect of a feature. A change is allowed only as its own explicit task (e.g. "optimize the generic repository"), keeping existing method signatures backward compatible, with `go test ./internal/repositories/` passing. Feature-specific queries go in the feature's repository, never into the generic one.

### 2. Models MUST have `TableName()` and `DeletedAt` (soft delete)

```go
func (Permission) TableName() string { return "permissions" }
```

Every model MUST include `DeletedAt gorm.DeletedAt \`gorm:"index" json:"-"\`` — `GenericRepository.Delete()` relies on this field for soft delete via GORM. Without it, `Delete()` performs a hard delete instead.

**Soft delete + UNIQUE:** a plain `UNIQUE (email)` breaks re-creating a value whose row was soft-deleted. GORM hides deleted rows, so the "exists?" check passes, then the INSERT hits the deleted row's key and fails with a 500. For any unique business column on a soft-deleted table, use the `alive` generated column (see `20260708000001_create_users_table.sql`):

```sql
alive TINYINT(1) GENERATED ALWAYS AS (IF(deleted_at IS NULL, 1, NULL)) VIRTUAL,
UNIQUE INDEX uq_{table}_{column}_alive ({column}, alive)
```

In the service, keep the friendly pre-check (`ExistsWithMap`) AND wrap the transaction error with `duplicateAsFieldError(err, "field", "message")`. That turns `gorm.ErrDuplicatedKey` (from a concurrent insert) into a 422 instead of a 500.

### 2b. Keep code compact — no needless splitting

First ask: **is this a general utility, or a step of one specific feature?**

| Kind | Example | Rule |
| ---- | ------- | ---- |
| **General utility**: domain-agnostic, input → output, makes sense outside this feature | parse/format a date, read an env int, hash a token, build a pagination meta, apply preloads | Keep it a named function in the right shared place (`internal/helpers/`, or the package's shared file) **even if it is used only once today** |
| **Feature-specific step**: only meaningful inside one flow | "clear expired buckets of this limiter", "wrap this one repo call", "build this service's notification payload" | Used in one place → write it inline. Extract only when a 2nd caller appears |

- Before writing a general utility, search `internal/helpers/` first and reuse what exists. Don't create a second `GetEnvInt`.
- Recursive functions and exported APIs another layer must call are always their own function.
- Extract a feature-specific helper only when it is used in **2+ places** or removes real duplication (e.g. `duplicateAsFieldError`, `issueTokens`).
- Two near-identical functions that differ by one value → ONE function with a parameter (e.g. `GenerateToken(..., tokenType, ...)`, not `GenerateToken` + `GenerateRefreshToken`).
- No one-line wrappers that only forward to another method. Call the target directly.
- Delete code nobody calls. Don't keep "might be useful later" methods.

### 3. Single Struct Pattern

- ALL service methods → `func (s *Services) ...`
- ALL handler methods → `func (h *Handlers) ...`
- NEVER create `type PermissionService struct`

### 4. Naming: `{Feature}{Action}` — Feature FIRST

```go
// CORRECT
func (s *Services) PermissionCreate(...)
func (h *Handlers) PermissionGetByID(...)

// WRONG
func (s *Services) CreatePermission(...)
```

### 5. Write Operations → MUST use Transaction

```go
s.repo.TxManager.WithinTransaction(func(tx *gorm.DB) error {
    result, err = s.repo.Permission.Create(tx, permission)
    return err
})
```

### 6. Read Operations → MUST use `nil`

```go
s.repo.Permission.FindByID(nil, id)  // CORRECT
s.repo.Permission.FindByID(s.repo.Permission.DB, id)  // WRONG
```

### 7. Read Operations → Return DTOs, NOT Models

```go
// Convert models to DTOs using To{Feature}DTO()
func (s *Services) PermissionGetByID(ctx context.Context, id uint) (*dtos.PermissionDTO, error) {
    permission, err := s.repo.Permission.FindByID(nil, id, "Roles")
    if err != nil {
        return nil, err
    }
    return dtos.ToPermissionDTO(permission), nil
}
```

### 8. Paginated Read → Handle nil opts

```go
func (s *Services) PermissionGetAll(ctx context.Context, opts *repositories.QueryOptions) (*repositories.PagedResult[dtos.PermissionDTO], error) {
    if opts == nil {
        opts = &repositories.QueryOptions{}
    }
    // ... convert to DTOs, return PagedResult[DTO]
}
```

### 9. File Naming

| Layer      | Pattern                   | Example                    |
| ---------- | ------------------------- | -------------------------- |
| Model      | `{feature}.go`            | `permission.go`            |
| DTO        | `{feature}_dto.go`        | `permission_dto.go`        |
| Repository | `{feature}_repository.go` | `permission_repository.go` |
| Service    | `{feature}_service.go`    | `permission_service.go`    |
| Handler    | `{feature}_handler.go`    | `permission_handler.go`    |
| Route      | `{feature}_route.go`      | `permission_route.go`      |

### 10. DTO Field Naming — Request vs Response

**Request DTO** — use entity name without `_id` suffix:

```go
type RoleRequest struct {
    Permissions []uint `json:"permissions"` // NOT "permission_ids"
}

type UserRequest struct {
    Roles []uint `json:"roles"` // NOT "role_ids"
}
```

**Response DTO** — use `_id` suffix for foreign keys:

```go
type NotificationDTO struct {
    UserID uint `json:"user_id"` // keep _id in response
}
```

---

## Logging

Logger available via `s.Logger`. CREATE/UPDATE/DELETE MUST log. GET simple: NO log.

```go
s.Logger.LogStart("FuncName", "Message: %s", value)
s.Logger.LogStep("FuncName", "Step: %s", value)
s.Logger.LogEnd("FuncName", "Success: %s", value)
s.Logger.LogEndWithError("Func", "Error: %v", err)
s.Logger.LogError("FuncName", "Error: %v", err)
s.Logger.LogWarn("FuncName", "Warning: %s", value)
s.Logger.LogInfo("FuncName", "Info: %s", value)
```

**Microservice Variant:** if the project has a `middleware.TraceID` (propagating `X-Trace-Id` from the gateway so logs correlate across services), prefer the context-aware form so every log line carries the trace id automatically:

```go
s.Logger.LogCtx(ctx, "FuncName", "Message: %s", value)
```

See `references/service.go` for full examples.

---

## Notification

**Microservice Variant:** skip this whole section if the project has no notification model/table (check `internal/models/` and `internal/dtos/`) — not every microservice owns a notification system. Don't add one just to satisfy this rule.

Every CREATE/UPDATE/DELETE MUST create notification via `s.NotificationCreate()`:

```go
_ = s.NotificationCreate(ctx, &services.NotificationCreateParams{
    Type:    "success",  // info, warning, success, error
    Title:   "Title",
    Message: "Message",
    Data:    map[string]interface{}{"id": result.ID, "name": result.Name}, // identifier data only
})
```

**Rules:**

- Errors MUST NOT fail the transaction → use `_ = s.NotificationCreate(...)`
- Created **AFTER** transaction completes — NOT inside transaction
- Data = identifier data only (id, name, status, etc) — NOT full object

See `references/service.go` for full examples.

---

## GenericRepository Methods

All repos extend `GenericRepository[T]`. Do NOT re-implement:

| Method                   | Signature                                                                        |
| ------------------------ | -------------------------------------------------------------------------------- |
| `FindByID`               | `(tx *gorm.DB, id uint, preloads ...string)`                                     |
| `FindByIDWithOpts`       | `(tx *gorm.DB, id uint, opts *QueryOptions) (*T, error)`                         |
| `Create`                 | `(tx *gorm.DB, request *T) (*T, error)`                                          |
| `CreateMany`             | `(tx *gorm.DB, request []T) error`                                               |
| `Update`                 | `(tx *gorm.DB, filter *T, update *T) (*T, error)`                                |
| `UpdateMap`              | `(tx *gorm.DB, filter *T, update map[string]interface{})`                        |
| `Delete`                 | `(tx *gorm.DB, id uint) (*T, error)`                                             |
| `FindAll`                | `(tx *gorm.DB, preloads ...string)`                                               |
| `FindAllWithOpts`        | `(tx *gorm.DB, opts *QueryOptions) (*PagedResult[T], error)`                     |
| `FindByField`            | `(tx *gorm.DB, filter *T, preloads ...string)`                                   |
| `FindByFieldMap`         | `(tx *gorm.DB, filter map[string]interface{}, preloads ...string)`               |
| `Count`                  | `(tx *gorm.DB) (int64, error)`                                                   |
| `Exists`                 | `(tx *gorm.DB, filter *T) (bool, error)`                                         |
| `ExistsWithMap`          | `(tx *gorm.DB, filter map[string]interface{}) (bool, error)`                     |

### QueryOptions Fields

```go
type QueryOptions struct {
    Page            int              // Page number (default: 1)
    PageSize        int              // Items per page (0/unset = default 10; negative = no pagination, returns all rows)
    SortBy          string           // Field to sort by
    Order           string           // "ASC" | "DESC" (default: "ASC")
    Preloads        []string         // Relations to preload
    Omits           []string         // Columns to omit from SELECT
    ConditionGroups []ConditionGroup // WHERE groups, AND-ed together
}
```

`ConditionGroups` — groups joined by AND; conditions within each group use group `Logic`:

```go
// Example: (name LIKE '%john%' OR email LIKE '%john%') AND (status = 1)
opts := &repositories.QueryOptions{
    ConditionGroups: []repositories.ConditionGroup{
        {
            Logic: "OR",
            Conditions: []repositories.QueryCondition{
                {Column: "name", Operator: "LIKE", Value: "%john%"},
                {Column: "email", Operator: "LIKE", Value: "%john%"},
            },
        },
        {
            Logic: "AND",
            Conditions: []repositories.QueryCondition{
                {Column: "status", Operator: "=", Value: 1},
            },
        },
    },
}
```

`SortBy` and `QueryCondition.Column` are concatenated into SQL, so the generic layer validates them (`column` / `table.column` only) and whitelists `Operator` (`=, !=, <>, <, <=, >, >=, LIKE, NOT LIKE, ILIKE, IN, NOT IN`). Anything else returns `repositories.ErrInvalidQueryOption`. If a sort/filter column comes from a query param, the **service** must still map it from an allow-list (`map[string]string{"name": "users.name"}`) — never pass user input through as-is.

### Generic vs Custom Query — Decision Guide (MANDATORY)

Go down the list and stop at the first match:

| # | Need | Use |
| - | ---- | --- |
| 1 | One row by id (+ relations) | `FindByID(nil, id, "Rel")` / `FindByIDWithOpts` |
| 2 | Rows of ONE table filtered by equality (`col = value`, incl. zero values) | `FindByFieldMap` / `ExistsWithMap` |
| 3 | List page: sort + WHERE with `= != < > LIKE IN` on this table's columns + relations to render | `FindAllWithOpts` + `QueryOptions` |
| 4 | Filter/sort by **another table's** column (JOIN), subquery, `EXISTS`, `IS NULL`, `GROUP BY`/aggregate, `DISTINCT`, only some columns, bulk update/delete with a non-id WHERE | **Custom repository method** |
| 5 | #4 but paginated | Custom method that builds the base query and returns `r.findPaged(base, opts)` |

Rules:

- **Don't** write a custom method for something rows 1–3 already do (e.g. `FindByName` = `FindByFieldMap`).
- **Don't** force a generic method where it does not fit. For example, don't load all rows and filter in Go, or call `FindByID` in a loop. That is the case for a custom method.
- **Don't** re-implement pagination (COUNT/LIMIT/OFFSET/TotalPages) in a custom method. Use `findPaged`.
- Custom methods use `r.getDB(tx).Model(&models.X{})...` so they join transactions and keep the soft-delete scope. A raw `Table("x")` skips `deleted_at IS NULL`, so add that condition yourself for soft-deleted tables.
- Name custom methods after what they return: `FindAllByRoleWithOpts`, `FindAccessNames`, `CountPerRole`.

See `references/repository.go` for one example per case.

### Query Performance & N+1 (MANDATORY)

An N+1 is one query for a list, then one more query **per row**. It is forbidden.

| Situation | Do | Don't |
| --------- | -- | ----- |
| Response shows a has-many / many-to-many relation | `Preloads: []string{"Roles"}`: 1 extra query per relation level | Loop over rows calling `FindByID`/`FindRolesByUserID` |
| Filter or sort by a belongs-to column (`satker.code`) | Custom method with `Joins(...)` | Preload then filter in Go |
| Need data for a set of ids | One query `Where("id IN ?", ids)`, then map in Go | One query per id |
| Only names/ids needed (checks, dropdowns) | Custom method with `Select("a, b")` + `Scan` into a small struct | `Preload("Roles.Permissions")` of full rows |
| Filter by membership (users with role X) | Subquery: `Where("users.id IN (?)", sub)` | `Pluck` all ids, then `IN (...)` with thousands of values |
| Existence check | `ExistsWithMap` (`SELECT 1 ... LIMIT 1`) | `Count` / `FindAll` + `len()` |

Also:

- Preload **only** what the response renders. Nested preloads (`A.B.C`) on list endpoints multiply rows fast.
- Every column used in `WHERE`/`ORDER BY`/`JOIN` on a growing table needs an index in the migration. Use a composite index in filter-then-sort order, e.g. `(user_id, deleted_at, created_at)`.
- `PageSize < 0` (no pagination) is only for small, bounded tables (permissions, roles). Never use it for users, notifications or transactions.

### Transaction Manager

```go
s.repo.TxManager.WithinTransaction(func(tx *gorm.DB) error { ... })
result, err := s.repo.TxManager.WithinTransactionWithResult(func(tx *gorm.DB) (interface{}, error) { ... })
```

---

## Helpers

### Which error to return (MANDATORY)

Pick the first one that fits:

| Case | Return | HTTP |
| ---- | ------ | ---- |
| Problem with one input field (duplicate email, invalid reference id, value not allowed) | `&helpers.FieldError{Field: "email", Message: "user already exists"}` | 422 |
| Any other business-rule failure (wrong state, limit reached, conflict, feature-specific message) | `&helpers.CustomError{Status: http.StatusConflict, Message: "..."}` | the `Status` you set (default 400) |
| Record not found / forbidden / auth-token failures | existing sentinels: `helpers.ErrNotFound`, `helpers.ErrForbidden`, `helpers.ErrInvalidToken`, ... | 404 / 403 / 401 |

**Do NOT add new entries to the "Common application errors" `var (...)` block in `error_helper.go`.** That list is only for generic, cross-feature conditions that `HandleError` maps to a status. A feature-specific error such as `ErrRoleInUse`, `ErrInvoiceAlreadyPaid` or `ErrStockNotEnough` must be a `CustomError` or `FieldError` returned from the service. `HandleError` renders both automatically, with no switch case and no handler change. A new sentinel that `HandleError` does not know falls through to **500**.

```go
// WRONG — feature-specific sentinel added to error_helper.go
var ErrRoleInUse = errors.New("role is still assigned to users")

// RIGHT — returned from the service, rendered by HandleError
return &helpers.CustomError{Status: http.StatusConflict, Message: "role is still assigned to users"}

// RIGHT — tied to an input field
return &helpers.FieldError{Field: "permissions", Message: "permission 42 does not exist"}
```

Return them **unwrapped**. `HandleError` uses a type assertion, so `fmt.Errorf("...: %w", fieldErr)` becomes a 500.

See `internal/helpers/error_helper.go` for the existing sentinels; do not copy the list here, it would go stale.

### Custom Error — `CustomError`

Use `helpers.CustomError` to return custom HTTP status and message from service layer:

```go
// Default status 400 (Bad Request)
return &helpers.CustomError{
    Message: "ID tidak valid",
}

// With custom status
return &helpers.CustomError{
    Status:  http.StatusConflict, // 409
    Message: "Data sudah ada",
}
```

Handler handles it automatically via `HandleError()`:

```go
err := s.Services.SomeAction(id)
if helpers.HandleError(c, err, "Fallback message") {
    return
}
```

| Scenario | Status | Message |
|----------|--------|---------|
| `Status` not set | 400 | Custom message |
| `Status` set | Custom | Custom message |

### Response Helpers

```go
helpers.OK(c, "msg", data)               // 200
helpers.OKWithMetadata(c, "msg", result) // 200 — for paginated PagedResult
helpers.Created(c, "msg", data)          // 201
helpers.BadRequest(c, "msg")             // 400
helpers.Unauthorized(c, "msg")           // 401
helpers.Forbidden(c, "msg")              // 403
helpers.NotFound(c, "msg")               // 404
helpers.InternalServerError(c, "msg")    // 500
helpers.ValidationResponse(c, errs)      // 422
helpers.ValidationError(c, field, msg)   // 422
```

### Validation Pattern (Handler)

```go
if err := c.BindJSON(&req); err != nil {
    helpers.BadRequest(c, "Invalid JSON payload")
    return
}
if err := h.Validate.Struct(&req); err != nil {
    helpers.ValidationResponse(c, h.getErrorsMap(err))
    return
}

// ALL service errors MUST go through HandleError:
if helpers.HandleError(c, err, "Failed to process request") {
    return
}
```

See `references/handler.go` for full examples.

---

## Packages (`internal/pkg/`)

### Redis — `s.RedisClient`

Always check `IsCacheAvailable()` first. Redis errors MUST NOT fail operations.

```go
if s.RedisClient.IsCacheAvailable() {
    s.RedisClient.SetJSON("key", value, time.Hour*24)
    s.RedisClient.GetJSON("key", &dest)
    s.RedisClient.Delete("key")
}
```

### Email — `s.EmailClient`

**Fullservice only** — a microservice behind an api-gateway typically has no `EmailClient` on `Services` at all (email sending, if needed, is its own concern or delegated elsewhere). Don't add it unless the project already has `internal/pkg/email`.

```go
s.EmailClient.IsConfigured()

// Sending email must not block the response — run it with s.runAsync (see "Background Work")
s.runAsync("AuthForgetPassword.sendEmail", func() {
    if err := s.EmailClient.SendResetPasswordEmail(email, token, resetURL); err != nil {
        s.Logger.LogError("AuthForgetPassword", "Failed to send reset email: %v", err)
    }
})
```

### Access — `s.Access` (Permission/Role)

**Fullservice:** Redis when available, otherwise an in-memory cache (5 min TTL), then the DB. The DB load is ONE JOIN query, `UserRoleRepository.FindAccessNames`, injected into `helpers.NewAccess` by the DI container. `helpers` never builds SQL itself.

```go
s.Access.HasPermission(ctx, "user.delete")  // Check ANY permission
s.Access.HasRole(ctx, "admin")              // Check role
s.Access.Invalidate(userID)                 // Clear cache
```

**Microservice Variant:** if `internal/middleware/gateway_auth.go` exists, `Access` has no cache and no `Invalidate()` — it's a stateless reader over roles/permissions the api-gateway already resolved and put in the request context (via `helpers.GetCallerRoles`/`GetCallerPermissions`). The `HasPermission`/`HasRole` call signatures stay the same, so business code reads identically either way — only what backs them differs. Route-level `middleware.RequirePermission(...)` may not exist in this variant; permission checks can instead live in the service layer (call `s.Access.HasPermission(ctx, "...")` directly where the business rule needs it) — check `internal/routes/{feature}_route.go` for a comment confirming this before assuming route middleware enforces it.

Permission naming: `{resource}.{action}` (e.g., `user.create`, `user.delete`)

### Scoped Permissions (Satker/Unit Restriction)

When a role must only access data within their own unit/satker, use scoped permission naming:

| Pattern                      | Example             | Description                                  |
| ---------------------------- | ------------------- | -------------------------------------------- |
| `{resource}.{action}`        | `user.index`        | Full access — view all data                  |
| `{resource}.{action}-satker` | `user.index-satker` | Scoped access — view data in own satker only |

**Route Usage (Access Control):**

```go
// Allow BOTH permissions — middleware checks if user has ANY of them
users.GET("", middleware.RequirePermission(acc, "user.index", "user.index-satker"), handlers.UserGetAll)
```

**Service Usage (Data Filtering):**

```go
func (s *Services) UserGetAll(ctx context.Context, opts *repositories.QueryOptions) (*repositories.PagedResult[dtos.UserDTO], error) {
    if opts == nil {
        opts = &repositories.QueryOptions{}
    }

    // Scoped access — filter by caller's satker. Route already requires user.index OR user.index-satker,
    // so reaching here without the satker permission means the caller has full access (user.index).
    if s.Access.HasPermission(ctx, "user.index-satker") {
        callerID := helpers.GetCallerID(ctx)
        caller, err := s.repo.User.FindByID(nil, callerID, "Satker")
        if err != nil {
            return nil, helpers.ErrForbidden
        }
        opts.ConditionGroups = append(opts.ConditionGroups, repositories.ConditionGroup{
            Logic:      "AND",
            Conditions: []repositories.QueryCondition{{Column: "satker_id", Operator: "=", Value: caller.SatkerID}},
        })
    }

    result, err := s.repo.User.FindAllWithOpts(nil, opts)
    if err != nil {
        return nil, err
    }

    userDTOs := make([]dtos.UserDTO, len(result.Data))
    for i, u := range result.Data {
        userDTOs[i] = dtos.ToUserDTO(&u)
    }

    return &repositories.PagedResult[dtos.UserDTO]{
        Data: userDTOs, Total: result.Total, Page: result.Page, PageSize: result.PageSize, TotalPages: result.TotalPages,
    }, nil
}
```

**Rules:**

- Routes allow BOTH permissions (full OR scoped)
- Service decides data scope based on which permission user actually has
- Filtering logic belongs in Service layer, NOT handler or route

See `references/route.go` for full examples.

---

## Services & Handlers Struct

```go
type Services struct {
    repo         *repositories.Repositories
    RedisClient  *database.RedisCache
    EmailClient  *pkgEmail.EmailClient    // internal/pkg/email — fullservice only, see Packages section
    Access       *helpers.Access
    Logger       *helpers.Logger
    // ...
}

type Handlers struct {
    svcs     *services.Services
    Validate *validator.Validate
    trans    ut.Translator
}
```

Don't assume every field above exists — check `internal/services/00_services.go` for the actual `ServicesConfig`/`Services` struct of the project at hand before wiring in a dependency.

---

## Background Work

There is **no job queue** in this project (Asynq was removed). Two patterns only:

| Need | Use |
| ---- | --- |
| Fire-and-forget after a request (send email, warm a cache) | `s.runAsync("Func.step", func() { ... })` |
| Periodic maintenance (clean `storage/tmp`) | Standalone CLI in `cmd/{name}/main.go` (e.g. `cmd/cleartmp`), scheduled by OS cron / Task Scheduler / k8s CronJob |

`s.runAsync` (in `internal/services/00_services.go`) starts a goroutine with `recover()` and logs any panic. Rules:

- NEVER write a bare `go func() { ... }()` in a service. A panic in it kills the whole API process. Always use `s.runAsync`.
- The closure must not use the request `ctx` (it is cancelled when the response is sent) or a `tx` (already committed or rolled back). Copy the values you need into local variables first.
- Work done this way has no retry and is lost if the process restarts. Only use it where that is acceptable (notification email, cache warm-up). Anything that must happen belongs in the DB transaction itself.
- Log success/failure inside the closure with `s.Logger`.

---

## Anti-Patterns

| WRONG                                           | CORRECT                                               |
| ----------------------------------------------- | ----------------------------------------------------- |
| `type PermissionService struct`                 | Method on `(s *Services)`                             |
| `func (s *Services) CreatePermission(...)`      | `func (s *Services) PermissionCreate(...)`            |
| `FindByID(s.repo.Permission.DB, id)`            | `FindByID(nil, id)`                                   |
| Direct DB write without transaction             | `TxManager.WithinTransaction(...)`                    |
| `c.ShouldBindJSON(&req)`                        | `c.BindJSON(&req)` + `h.Validate.Struct(&req)`        |
| `helpers.BadRequest(c, "msg")` for field errors | `helpers.ValidationError(c, "field", "msg")` |
| Manual error check `if err == helpers.ErrX`     | `helpers.HandleError(c, err, "fallback")`            |
| New feature sentinel `var ErrXxx = errors.New(...)` in `error_helper.go` | `&helpers.CustomError{Status: ..., Message: ...}` / `&helpers.FieldError{...}` |
| `s.repo.User.DB.Where(...)` / `tx.Joins(...)` in a service | Custom repository method |
| Repository method that checks permissions / logs / builds DTOs | Keep it in the service |
| Calling a repository inside a `for` loop over rows (N+1) | `Preloads`, `JOIN`, or one `IN ?` query |
| `Preload("A.B")` just to read names / check membership | Custom method with `Select` + `JOIN` |
| Custom method re-implementing COUNT + LIMIT/OFFSET | `r.findPaged(baseQuery, opts)` |
| Custom method duplicating `FindByFieldMap` (`WHERE col = ?`) | Use the generic method |
| Pluck ids then `IN (ids...)` | Subquery `Where("x.id IN (?)", sub)` |
| Bare `go func()` in a service | `s.runAsync("Name", func() {...})` |
| `UNIQUE (col)` on a soft-deleted table | `UNIQUE (col, alive)` generated column |

---

## Pre-Implementation Checklist

- [ ] Model has `TableName()`
- [ ] DTO variables use feature prefix
- [ ] Request DTO: no `_id` suffix (e.g. `roles`), Response DTO: keep `_id` (e.g. `user_id`)
- [ ] Service/Handler: `{Feature}{Action}` on single struct
- [ ] Write ops use `TxManager.WithinTransaction()`
- [ ] Read ops use `nil` parameter
- [ ] Read ops return DTOs, NOT models
- [ ] Paginated reads handle nil opts
- [ ] CREATE/UPDATE/DELETE have logging (+ notification, only if the project has a notification system — see Notification section)
- [ ] Notification Data = identifier only (id, name, status) — if applicable
- [ ] Repository registered in `00_repository.go`
- [ ] Service has no gorm query code (only repository calls + `Association` inside tx)
- [ ] Generic vs custom chosen with the Decision Guide; paginated custom queries use `findPaged`
- [ ] No repository call inside a loop (N+1); relations via `Preloads` or JOIN
- [ ] New WHERE/ORDER BY columns on growing tables have an index in the migration
- [ ] Unique columns on soft-deleted tables use the `alive` pattern + `duplicateAsFieldError`
- [ ] Feature errors are `CustomError`/`FieldError`, NOT new sentinels in `error_helper.go`
- [ ] Async work uses `s.runAsync`, no bare `go func()`
- [ ] Handler uses `c.BindJSON()` + `h.Validate.Struct()`
- [ ] ALL errors handled via `HandleError()`, NO manual error checks
- [ ] Build: `go build ./...` | Vet: `go vet ./...` | Test: `go test ./...`
