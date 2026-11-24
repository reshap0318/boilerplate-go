---
name: go-development-guide
description: Step-by-step development workflow and CRUD implementation guide for Go project — from planning to push. Use when creating new features, implementing CRUD operations, adding permission-based access control, or when user mentions models, DTOs, repositories, services, handlers, or routes.
---

## When to use me

Use this skill when:
- Implementing a new CRUD feature from scratch (Model → DTO → Repository → Service → Handler → Routes)
- Following the mandatory development workflow (plan → issue → branch → PR)
- Adding scoped permission patterns (satker/unit restrictions) to existing features
- You need a pre-push checklist before committing code

> 💡 **If `go-coding-rules` skill is available**, use it for coding conventions, helpers, and full code examples. This guide focuses on workflow and implementation order.

---

# Go Development Guide

## 🔄 Development Workflow (MANDATORY)

Every new feature or bug fix **MUST** follow this flow:

### Phase 1: Planning
1. **Create `plan.md`** in the `docs/` folder at project root:
   - Feature/bug description
   - Technical analysis (files to create/modify)
   - Implementation steps
   - Impact on existing code
2. **User Approval**: Wait for user confirmation.
3. **Create GitHub Issue** with format: `[BE] [Type] Title`
   - `[Feat]`, `[Bug]`, `[Fix]`, `[Refactor]`, `[Chore]`

### Phase 2: Implementation
1. **Create Branch**: `git checkout -b feat/feature-name`
2. **Implement**: Based on approved plan and issue.
3. **Create Pull Request**: Reference issue in description (`Closes #<id>`).
4. **User Approval**: Summary of changes → approval → merge → close issue.

---

## 🛠️ CRUD Implementation Flow

### 📋 Step-by-Step Order
Every new CRUD feature MUST follow this order:

| Step | Layer | File Path | Purpose |
|------|-------|-----------|---------|
| 1 | Model | `internal/models/{feature}.go` | GORM model + TableName() |
| 2 | Migration | `cmd/migration/migrations/{timestamp}_{feature}.sql` | CREATE TABLE SQL |
| 3 | DTO | `internal/dtos/{feature}_dto.go` | Request/Response structs + converter |
| 4 | Repository | `internal/repositories/{feature}_repository.go` | Extend GenericRepository |
| 5 | Register Repo | `internal/repositories/00_repository.go` | Add to Repositories struct |
| 6 | Service | `internal/services/{feature}_service.go` | Business logic + logging (+ notification, if the project has one) |
| 7 | Handler | `internal/handlers/{feature}_handler.go` | HTTP handler + validation |
| 8 | Routes | `internal/routes/{feature}_route.go` | Route registration + middleware |
| 9 | Register Routes | `internal/routes/00_route.go` | Add call inside RegisterAll() |

> For coding rules (naming, transactions, logging, notification, helpers), see `go-coding-rules` skill.

---

### Step 1: Model

**File:** `internal/models/{feature}.go`

Define GORM model with struct tags. See `go-coding-rules` for conventions.

---

### Step 2: Migration

**File:** `cmd/migration/migrations/{timestamp}_{feature}.sql`

Create the SQL migration file for the new table. Index every column the feature will filter, join or sort on (composite in filter-then-sort order). For UNIQUE business columns on a soft-deleted table, use the `alive` generated-column pattern instead of a plain `UNIQUE`; see `go-coding-rules` → "Soft delete + UNIQUE". Timestamp format: `YYYYMMDDHHMMSS` (goose parses the numeric prefix as the version — must be unique and greater than existing migrations). Files are loaded via `//go:embed migrations/*.sql` in `cmd/migration/main.go`, so they MUST live in that exact folder. See `go-coding-rules` for migration conventions.

---

### Step 3: DTOs

**File:** `internal/dtos/{feature}_dto.go`

Create Request struct, Response DTO, and converter functions (`To{Feature}DTO`, `To{Feature}DTOList`). See `go-coding-rules` for conventions.

---

### Step 4: Repository

**File:** `internal/repositories/{feature}_repository.go`

Extend `GenericRepository[Model]`. Decide per query with `go-coding-rules` → "Generic vs Custom Query — Decision Guide":
- Equality filter / list with sort + simple WHERE / by id → generic method, no custom code.
- JOIN, subquery, aggregate, partial columns, `IS NULL`, bulk update → custom method **in this file**, paginated ones via `r.findPaged(base, opts)`.
- Plan the relations the response renders now (`Preloads`/JOIN), so the service never queries inside a loop (N+1).

---

### Step 5: Register Repository

**File:** `internal/repositories/00_repository.go`

Add repository field to `Repositories` struct and initialize in `NewRepositories()`.

---

### Step 6: Service

**File:** `internal/services/{feature}_service.go`

Implement CRUD methods on `(s *Services)`. The service holds the logic and calls repository methods; it contains **no query code**. If you need a query that no repository method provides, go back to Step 4 and add it there. Feature errors are `helpers.CustomError` / `helpers.FieldError`, not new sentinels in `error_helper.go`. Write ops use transaction. Notification (if the project has one — see `go-coding-rules` "Service Topology") created **after** transaction completes. Read ops return DTOs. See `go-coding-rules` for full patterns.

---

### Step 7: Handler

**File:** `internal/handlers/{feature}_handler.go`

Implement HTTP handlers on `(h *Handlers)`. Bind JSON, validate, call service, return response. See `go-coding-rules` for full patterns.

---

### Step 8: Routes

**File:** `internal/routes/{feature}_route.go`

Register routes with permission middleware via `middleware.RequirePermission(acc, "permission.name")` — this is the fullservice default. In a microservice behind an api-gateway, permission checks may instead belong in the service layer via `s.Access.HasPermission(ctx, "...")`; check existing `{feature}_route.go` files in the project for the pattern actually in use before assuming route-level middleware.

---

### Step 9: Register Routes

**File:** `internal/routes/00_route.go`

Add the new route registration call inside `RegisterAll()`. `cmd/api/main.go` never needs to be changed for new features.

---

## ✅ Pre-Push Checklist
- [ ] Model has `TableName()` method
- [ ] Migration file created in `cmd/migration/migrations/` (timestamp prefix `YYYYMMDDHHMMSS`)
- [ ] DTO variables use feature prefix
- [ ] Request DTO: merged for simple features OR separate (`{Feature}CreateRequest` + `{Feature}UpdateRequest`) for complex features
- [ ] Service functions use feature prefix (`{Feature}{Action}`)
- [ ] Handler functions use feature prefix (`{Feature}{Action}`)
- [ ] Write operations use `TxManager.WithinTransaction()` or `WithinTransactionWithResult()`
- [ ] Read operations use `nil` (NOT `.DB`)
- [ ] Service contains no gorm query code; custom queries live in the repository
- [ ] No N+1: no repository call inside a loop; relations via `Preloads` or JOIN
- [ ] New filter/sort columns are indexed in the migration
- [ ] Errors use `CustomError`/`FieldError` (no new sentinel in `error_helper.go`)
- [ ] Logging on Create/Update/Delete
- [ ] Notification created **after** transaction completes for Create/Update/Delete — only if the project has a notification system (skip for services without one, e.g. most microservices behind an api-gateway)
- [ ] Repository registered in `00_repository.go`
- [ ] Routes registered in `internal/routes/00_route.go`
- [ ] Build success (`go build ./...`)
- [ ] Vet clean (`go vet ./...`)
- [ ] Tests pass (`go test ./...`)
