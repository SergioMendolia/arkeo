# AGENTS.md

## Project

Arkeo is a Go CLI that aggregates daily activity from multiple service connectors (GitHub, GitLab, Google Calendar via iCal, YouTrack, macOS system logs, webhooks) into a chronological timeline for timesheeting. Single-module Go project: `github.com/arkeo/arkeo`.

## Commands

There is **no Makefile**. Use Go directly:

```bash
go build -o arkeo .              # build binary
go build ./...                    # build all packages
go test ./...                     # run tests
go test -race ./...               # tests with race detector (CI uses this)
go test ./internal/connectors/    # single package
go test -run TestName ./internal/connectors/  # single test
go vet ./...                      # vet (CI runs this before tests)
```

CI verification order (matches `.github/workflows/test.yml`): `go vet ./...` -> `go test -v -race -coverprofile=coverage.out ./...`.

## Layout

- `main.go` — entrypoint; sets `version` (injected via `-ldflags -X main.version=...` during release builds).
- `cmd/` — Cobra CLI commands: `root.go` (registry/config init), `timeline.go`, `connectors.go`. No `config.go` command file exists despite older docs.
- `internal/connectors/` — each connector implements the `Connector` interface; new connectors are registered in `cmd/root.go:initializeSystem` (`availableConnectors` slice).
- `internal/config/` — Viper-based config, stored at `~/.config/arkeo/config.yaml` (XDG_CONFIG_HOME respected). `DefaultConfig()` and `GenerateExampleConfigYAML()` are the sources of truth for connector config schema; `config.example.yaml` is generated from these.
- `internal/display/formatters/` — output formats (table/json/csv/taxi); new formatters are wired into the switch in `internal/display/timeline.go` (both `displaySingleDay` and `displayMultipleDays`).
- `internal/timeline/activity.go` — `Activity` and `Timeline` types.
- `internal/utils/parallel.go` — parallel connector fetching (always on).

## Conventions

- Errors: use `fmt.Errorf` with `%w` for wrapping; failed connectors log but don't abort the timeline fetch.
- Config access in connectors via `BaseConnector` helpers: `GetConfigString`, `GetConfigBool`, `GetConfigInt`.
- Connector config is nested under `connectors.<name>.config`; enable via `connectors.<name>.enabled`.
- Taxi formatter rounds to quarter-hours with continuation format (`-HH:MM`); see `internal/display/formatters/taxi.go` before changing rounding behavior.

## Environment / Debug

- `ARKEO_DEBUG` env var enables debug mode for connectors.
- `LOG_LEVEL` env var and `app.log_level` config control log level.
- `XDG_CONFIG_HOME` overrides config directory; `--config` flag overrides config file path.

## Releases

Tag-driven, fully automated via `.github/workflows/release.yml`:

```bash
git tag v1.0.0
git push origin v1.0.0
```

CI builds cross-platform (linux/darwin amd64+arm64, windows amd64) with `CGO_ENABLED=0` and `-ldflags="-s -w -X 'main.version=...'"`, archives include `README.md LICENSE config.example.yaml`, and creates the GitHub release. Prerelease if tag contains `rc`/`beta`/`alpha`.

## Notes for agents

- **Go version**: `go.mod` declares `go 1.24.0`; CI uses Go 1.25. Keep toolchain compatible with both.
- macOS connector only works on macOS (`log show` parsing) — tests for it are guarded accordingly.