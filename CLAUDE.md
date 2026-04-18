# rsync2project

Public Go CLI at github.com/fblissjr/rsync2project. Never commit personal data (hostnames, user@host, real paths) — this is a public repo.

## Commands

- `go build -o rsync2project .` — builds binary (gitignored at repo root).
- `go test ./... -count=1` — runs unit + integration; integration tests skip when `rsync` isn't on PATH.
- `./install.sh` — OS-aware; builds and installs to first writable user-bin dir on PATH.

## Conventions

- `go.mod` pinned to `go 1.21` for server compat — no `slices.Sorted`, no iterator-form `maps.Keys`.
- Add a project type: new `projectType` const + marker entry in detect.go, optional entry in `projectTypeExcludes` (excludes.go). Typed constants keep the tables linked.
- `alwaysExclude` (excludes.go) is regenerable cruft only. User-content dirs (logs, models, data) go through `.gitignore` or per-repo `--include`.
- Per-repo config at `~/.config/rsync2project/repos/<basename>.conf` — never in the source tree (accidental-commit hazard).
- Reuse `forEachConfigLine(path, fn)` for new config readers.
- Integration tests: `t.Setenv("XDG_CONFIG_HOME", t.TempDir())` + `requireRsync(t)` + `setupFakeProject(t, files)` helpers.
- Default path semantic is NEST source under destination; `--contents` for legacy spill-into behavior.
- `.git/` included by default (backup intent); `--no-vcs` to skip.
- `internal/` is gitignored for private tutorials and session notes.
