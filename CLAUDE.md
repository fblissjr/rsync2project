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
- Version bump ritual: update `main.go` version const + `CHANGELOG.md` entry, and (if `internal/` exists locally) refresh `internal/tutorial.md` version refs and append a timeline entry to today's `internal/log/log_<date>.md`.
- Legacy CLI flags replaced by subcommands (e.g. `--list-dests` vs `dest list`) are kept as working aliases on purpose — don't remove without a soft-deprecation plan across all of them in one release.

## Subcommand conventions

- New management commands go under an `{area}` subcommand (`dest`, `repo`, `config`), dispatched from the top of `main()` ahead of `flag.Parse` so each subcommand owns its flags.
- Subcommand handlers return `int` (exit code); `main` calls `os.Exit`. Shared helpers live in `cmd_common.go`:
  - `failMsg(err) int` — prints `"rsync2project: <err>"` to stderr and returns 1. Use this in subcommands instead of the top-level `fail()` (which `os.Exit`s and short-circuits test coverage).
  - `parseSubFlags(fs, args) (stop bool, code int)` — wraps `fs.Parse` so `--help` returns exit 0 instead of 2. Every subcommand handler should use it in place of a direct `fs.Parse`.
  - `addDryRunFlag(fs, &dryRun)` — binds both `-n` and `--dry-run` to one bool. Use for any mutating subcommand so the alias contract stays in one place.
- Each subcommand uses its own `flag.NewFlagSet(name, flag.ContinueOnError)` so `-n`, `--format`, etc. bind only inside that subcommand's scope.
- For arguments that can be either a repo name or a source path, use `resolveRepoConfigArg` — it normalizes both forms to the canonical `.conf` path.
- Machine-readable output: prefer a `--format json` flag on the read-side subcommand rather than reshaping the default text output. Current precedent: `repo show --format json`.
