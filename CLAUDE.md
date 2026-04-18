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
- Reuse `forEachConfigLine(path, fn)` for config readers that consume values; use `readLines` for readers that need to preserve blanks/comments for rewrite.
- Reuse `writeLinesAtomic(path, lines)` for any new config writer — it handles tmpfile + fsync + rename so a crash mid-write can't publish a torn file.
- Integration tests: `t.Setenv("XDG_CONFIG_HOME", t.TempDir())` + `requireRsync(t)` + `setupFakeProject(t, files)` helpers.
- Default path semantic is NEST source under destination; `--contents` for legacy spill-into behavior.
- `.git/` included by default (backup intent); `--no-vcs` to skip.
- `-n` means zero side effects. Applies to `--save-config` writes and to every mutating subcommand (`dest add`, `dest rm`, `repo rm`). Prints "dry-run: would ..." instead of acting.
- `saveRepoConfig` refuses to overwrite when the existing file's `# source:` header names a different absolute path (basename-collision guard).
- `internal/` is gitignored for private tutorials and session notes.

## Subcommand conventions

- New management commands go under an `{area}` subcommand (`dest`, `repo`, `config`), dispatched from the top of `main()` ahead of `flag.Parse` so each subcommand owns its flags.
- Subcommand handlers return `int` (exit code); `main` calls `os.Exit`. Inside subcommands, use `failMsg(err)` (prints `"rsync2project: <err>"` to stderr and returns 1) instead of the top-level `fail()` (which `os.Exit`s and short-circuits test coverage).
- Each subcommand uses its own `flag.NewFlagSet(name, flag.ContinueOnError)` so `-n` etc. bind only inside that subcommand's scope.
- For arguments that can be either a repo name or a source path, use `resolveRepoConfigArg` — it normalizes both forms to the canonical `.conf` path.
