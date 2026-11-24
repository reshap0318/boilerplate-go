---
name: api-documentation
description: Generate or update API documentation (Markdown, OpenAPI/Swagger, Postman collection) from Go/Gin routes, handlers and DTOs. Use when asked to document endpoints, write an API spec or swagger/openapi file, list routes, or share request/response examples with frontend/mobile developers.
---

## What I do
- Scan route definitions in `internal/routes/` and `cmd/api/main.go`
- Extract HTTP methods, paths, and handler mappings
- Parse request/response DTOs from `internal/dtos/`
- Generate documentation in Markdown, OpenAPI/Swagger, or Postman collection format
- Include authentication requirements (JWT protected vs public routes)

## When to use me
Use this when:
- You want to generate or update API documentation
- You need an OpenAPI/Swagger spec for the project
- You want to document a new feature's endpoints
- You need to share API specs with frontend/mobile developers

## Output Format

Default output is Markdown with this structure:

### `{Feature} Endpoints`

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/permissions` | JWT | Create permission |
| GET | `/api/permissions` | JWT | List all permissions |
| GET | `/api/permissions/:id` | JWT | Get permission by ID |
| PUT | `/api/permissions/:id` | JWT | Update permission |
| DELETE | `/api/permissions/:id` | JWT | Delete permission |

### Request/Response Examples

Include JSON examples for:
- Request body (from `{Feature}Request` DTO)
- Response body (from `{Feature}DTO` and response helpers)
- Error responses (400, 401, 404, 422, 500)

## Rules
- No API versioning — base path is `/api` (NOT `/api/v1`), per `cmd/api/main.go` (`r.Group("/api")`)
- Always check `cmd/api/main.go` for route registration and middleware groups
- Identify which routes are in the `protected` group (JWT required) vs public
- Use DTO field tags (`json`, `binding`) to document required/optional fields
- Reference `helpers.*` response functions for status codes
- Include pagination info if `FindAllWithOpts` is used
