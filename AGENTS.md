# Repository Guidelines

## Project Structure & Module Organization
`main.go` is the application entrypoint and wires HTTP routes, background workers, and startup configuration. Core code is split into top-level Go packages: `db/` for SQLite access and Mirakurun sync, `handlers/` for HTTP endpoints, `services/` for long-running business logic such as auto reservation, and `models/` for shared types and helpers. UI assets live in `static/`, API-related reference material lives in `docs/`, and runtime database files belong in `data/` and should not be committed.

## Build, Test, and Development Commands
Use Go 1.23.x.

```bash
go build ./...
go run main.go
go test -v ./db
go test -v ./...
docker build -t iepg-server-test -f Dockerfile.test .
docker run --rm iepg-server-test
docker compose up -d --build
```

`go build ./...` validates all packages. `go run main.go` starts the server locally on `PORT` (default `40870`). `go test -v ./db` matches the current CI baseline; `go test -v ./...` is recommended before larger changes. The Docker test image runs the same database-focused test suite, and `docker compose up -d --build` starts the full service with persistent `./data`.

## Coding Style & Naming Conventions
Follow standard Go formatting with tabs via `gofmt` before review. Keep packages focused and functions reasonably short. Exported names use Go-style `PascalCase`; internal helpers use `camelCase`. Match existing file naming such as `auto_reservation.go`, `reservation_test.go`, and keep HTTP handlers in `handlers/` rather than `main.go`.

## Testing Guidelines
Tests use Go’s built-in `testing` package and are colocated as `*_test.go`. Prefer real SQLite-backed tests (`:memory:` or temp DB files) over mocks. Add targeted coverage for database migrations, handler behavior, and reservation logic when changing those areas. If a feature affects Mirakurun-derived IDs or search behavior, add regression cases near the relevant package tests.

## Commit & Pull Request Guidelines
Recent history uses concise Conventional Commit prefixes such as `feat:` and `fix:` with short Japanese summaries. Keep commits scoped to one change. Open PRs against `main`, describe behavior changes, list verification commands, and attach screenshots when editing files in `static/`. Reference related issues and note any required environment variables or API dependencies.

## Agent Collaboration Notes
Respond to collaborators in Japanese, but keep code comments in English.

Prefer real integrations and production behavior. Do not add demo modes, and do not use mocks outside tests.

Write or update tests before implementation when practical. Do not sidestep breakage by disabling or ignoring failing tests.

When changing long-running server behavior, run the service in the background when needed, keep logs available, and verify behavior from the logs.

Use `gh` when GitHub state or review context is relevant.
