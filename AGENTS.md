# AGENTS.md

## Project

Arkeo is a Go CLI that aggregates daily activity from multiple service connectors (GitHub, GitLab, Google Calendar via iCal, YouTrack, macOS system logs, browser history via SQLite, webhooks) into a chronological timeline for timesheeting. Single-module Go project: `github.com/arkeo/arkeo`.

## Commands

There is **no Makefile**. Use Go directly:

```bash
go build -o arkeo .              # build binary
go build ./...                    # build all packages
CGO_ENABLED=0 go build ./...      # verify release constraint (no CGO)
go test ./...                     # run tests
go test -race ./...               # tests with race detector (CI uses this)
go test ./internal/connectors/    # single package
go test -run TestName ./internal/connectors/  # single test
go vet ./...                      # vet (CI runs this before tests)
```

CI verification order (matches `.github/workflows/test.yml`): `go vet ./...` -> `CGO_ENABLED=0 go build` -> `go test -v -race -coverprofile=coverage.out ./...`.

## Layout

- `main.go` — entrypoint; sets `version` (injected via `-ldflags -X main.version=...` during release builds).
- `cmd/` — Cobra CLI commands: `root.go` (registry/config init), `timeline.go` (timeline + cache + range), `connectors.go` (list/enable/disable/info/test), `browser.go` (browser domains TUI).
- `internal/connectors/` — each connector implements the `Connector` interface; new connectors are registered in `cmd/root.go:initializeSystem` (`availableConnectors` slice).
- `internal/config/` — Viper-based config, stored at `~/.config/arkeo/config.yaml` (XDG_CONFIG_HOME respected). `DefaultConfig()` and `GenerateExampleConfigYAML()` are the sources of truth for connector config schema; `config.example.yaml` is generated from these.
- `internal/cache/` — SQLite-based activity cache at `~/.config/arkeo/cache.db`. Stores activities per (date, connector) pair as JSON blobs.
- `internal/display/` — timeline display logic and format dispatch. `timeline.go` routes to formatters based on `opts.Format` ("table" or "json").
- `internal/display/formatters/` — output formats: `table.go` (one-line-per-activity, colored, truncated) and `json.go` (metadata-free JSON projection).
- `internal/display/colors/` — ANSI color codes, source labels, and helpers.
- `internal/timeline/activity.go` — `Activity` and `Timeline` types.
- `internal/utils/parallel.go` — parallel connector fetching (always on, semaphore-bounded).

## Conventions

- **CGO_ENABLED=0**: All builds must work without CGO (release constraint). Use `modernc.org/sqlite` (pure-Go) for SQLite, never `mattn/go-sqlite3`.
- **Errors**: use `fmt.Errorf` with `%w` for wrapping; failed connectors log but don't abort the timeline fetch.
- **Config access** in connectors via `BaseConnector` helpers: `GetConfigString`, `GetConfigBool`, `GetConfigInt`.
- **Connector config** is nested under `connectors.<name>.config`; enable via `connectors.<name>.enabled`.
- **Date filtering** in connectors should normalize to `time.Local` before computing day boundaries (see H3 fix in GitLab/YouTrack connectors).
- **Domain normalization** (`normalizeDomain` in `browser_history.go`): strips `www.`, collapses subdomains to registrable domain, handles multi-part TLDs (e.g. `co.uk`).
- **Table formatter**: one activity per line (`HH:MM  SRC  Title — Description`), truncated to 120 chars with `…`. ANSI-aware truncation.
- **JSON formatter**: excludes `Metadata` field from output (uses a projection struct).
- **Cache**: timeline checks cache first per day; cache miss → fetch from connectors → store per-connector results. `--reset-cache` clears range, `--no-cache` skips entirely.

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
- Browser history connector uses `modernc.org/sqlite` and copies the DB to a temp dir before querying (browser locks the file while running).
- TUI (`arkeo browser domains`) uses `charmbracelet/bubbletea` and `charmbracelet/lipgloss` — both pure-Go, CGO-free.
- `capitalizeFirst` in `connector.go` replaces the deprecated `strings.Title` (no `golang.org/x/text` dependency).
- `strings.Title` is deprecated — do not use it. Use `capitalizeFirst` instead.
- Output formats are **table** and **json** only. CSV and taxi formats have been removed.