---
name: go-init
description: 'Scaffold a NEW Go backend project from the boilerplate (fullservice monolith with local JWT auth/RBAC/notification, or microservice behind an api-gateway): clones the template, removes its .git and .agents, renames the Go module and import paths, creates .env. Use only when starting a new project/service ("buat project baru", "init dari boilerplate"); never inside an existing project — for adding features use go-development-guide.'
---

## Role

You are a Go Project Setup Specialist. Your task is to initialize a new Go project using the official boilerplate template.

## When to use me

Use this skill when:

- Starting a new Go project from scratch using the official boilerplate
- Setting up the initial repository structure, module, and environment
- Migrating or cloning the boilerplate for a new feature or service

## Workflow

### Step 1: Confirm Service Flavor

- Ask the user which boilerplate flavor to initialize:
  - **Fullservice** — monolith with local auth (JWT + Role/Permission), and Notification (background work via goroutines, no job queue).
  - **Microservice** — runs behind an api-gateway; identity comes from gateway headers/JWKS instead of local auth, no Notification by default.
- Wait for user confirmation. This determines which repo to clone in Step 4 and which skill defaults (see `go-coding-rules` → "Service Topology") apply once implementation starts.

### Step 2: Confirm Target Path

- Ask the user where to place the project:
  - Root folder (`.`)
  - Subfolder (e.g., `be/`, `backend/`)
- Wait for user confirmation.

### Step 3: Confirm Module Name

- Ask the user for the desired Go module name (e.g., `github.com/user/project`).
- Wait for user confirmation.

### Step 4: Clone Boilerplate

- **Fullservice:** clone `https://github.com/reshap0318/boilerplate-go.git` into the target path.
- **Microservice:** clone `https://github.com/reshap0318/boilerplate-service.git` into the target path.
- If cloning into a subfolder, ensure the directory exists first.

### Step 5: Remove Existing Git

- Remove `.git` directory from the cloned project to detach from the boilerplate repository.
- Run `Remove-Item -Recurse -Force -LiteralPath ".git"` on Windows, or `rm -rf .git` on Linux/macOS.

### Step 6: Remove `.agents` Folder (if present)

- The cloned template may carry its own `.agents/skills` (the boilerplate's own skill definitions). Remove it from the new project so it starts clean rather than inheriting the template repo's skills.
- Check if `.agents` exists at the project root; if it does, run `Remove-Item -Recurse -Force -LiteralPath ".agents"` on Windows, or `rm -rf .agents` on Linux/macOS. Skip silently if it doesn't exist.

### Step 7: Update Module & Imports

- Edit `go.mod` to use the new module name.
- Recursively find and replace all import paths in `.go` files from the cloned template's original module (check its `go.mod` before the edit above — differs per repo, e.g. `boilerplate-go` vs `boilerplate-service`) to the new module name.

### Step 8: Setup Environment

- Copy `.env.example` to `.env`.

### Step 9: Verify & Report

- Run `go mod tidy` to ensure dependencies are correct.
- Inform the user that the project is ready.
- Direct them to use skill `@go-development-guide` to start implementing features.
