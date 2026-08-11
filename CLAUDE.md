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
- Two independent filter layers, and a flag that disables one must not disable the other: `.gitignore` (`--no-gitignore`) and the builtin/project-type/user-file excludes (`--no-excludes`). `--all` is sugar for both, resolved right after `flag.Parse` so nothing downstream tests three booleans. `--no-vcs` and `--extra` survive `--all` — both are explicit requests on the same command line.
- Assemble excludes with `buildExcludeList(types, userExcludes, opts)`, not by appending to `buildExcludes` at the call site. It owns the layer gating.
- Default output itemizes (`-i`); `-q/--quiet` restores progress-only; `-n` always lists regardless of `-q`. A dry run that hides the file list has no content left.
- Every non-quiet run prints a two-line banner: resolved landing path, then filter state. Anything that changes which files transfer must be visible there — a silent mode switch is the bug this exists to prevent.
- Per-repo config at `~/.config/rsync2project/repos/<basename>.conf` — never in the source tree (accidental-commit hazard).
- Reuse `forEachConfigLine(path, fn)` for config readers that consume values; use `readLines` for readers that need to preserve blanks/comments for rewrite.
- Reuse `writeLinesAtomic(path, lines)` for any new config writer — it handles tmpfile + fsync + rename so a crash mid-write can't publish a torn file.
- Integration tests: `t.Setenv("XDG_CONFIG_HOME", t.TempDir())` + `requireRsync(t)` + `setupFakeProject(t, files)` helpers. For the git-backed path use `requireGit(t)` + `setupGitRepo(t, files, tracked)` (force-adds `tracked`, so a file can be both committed and ignore-matched) and assert with `assertPresent`/`assertAbsent`. `setupGitRepo` points `GIT_CONFIG_GLOBAL`/`GIT_CONFIG_SYSTEM` at `os.DevNull` — without that a developer's `core.excludesFile` leaks in and results go machine-dependent.
- Filter behavior is verified by mutation, not by assertion count: break the branch, confirm the test goes red with a message that names the real symptom, restore. Several tests here exist specifically because a mutation showed the previous ones were vacuous.
- Default path semantic is NEST source under destination; `--contents` for legacy spill-into behavior. When the destination's last segment repeats the source basename (`nestCollides`), warn and proceed — never auto-switch to `--contents` and never error: `myapp/myapp/` is a legal layout, and implicit mode-switching would be wrong for anyone backing up `proj/` into a directory named `proj/`.
- `runRsync(source, dest, includes, excludes, gitignoreFilter, opts)` — the filter mode is resolved once by `resolveGitignoreFilter` in main.go and passed in, so tests exercise the real resolution path instead of hand-building a mode.
- `.git/` included by default (backup intent); `--no-vcs` to skip.
- `-n` means zero side effects. Applies to `--save-config` writes and to every mutating subcommand (`dest add`, `dest rm`, `repo rm`). Prints "dry-run: would ..." instead of acting.
- `saveRepoConfig` refuses to overwrite when the existing file's `# source:` header names a different absolute path (basename-collision guard).
- `internal/` is gitignored for private tutorials and session notes.
- Version bump ritual: update `main.go` version const + `CHANGELOG.md` entry, and (if `internal/` exists locally) refresh `internal/tutorial.md` version refs and append a timeline entry to today's `internal/log/log_<date>.md`.
- Legacy CLI flags replaced by subcommands (e.g. `--list-dests` vs `dest list`) are kept as working aliases on purpose — don't remove without a soft-deprecation plan across all of them in one release.

## .gitignore handling (gitignore.go)

- **Ask git; never translate patterns.** The ignore set comes from `git ls-files --others --ignored --exclude-standard --directory --full-name`. Do not "improve" this by parsing `.gitignore` into rsync filter rules: git is last-match-wins, rsync is first-match-wins, and anchoring, `**`, and nested-file precedence all differ. The old `--filter=':- .gitignore'` approximation silently dropped negated (`!pattern`) re-includes and *tracked* files that an ignore rule matched — losing files, which is the wrong failure direction for a backup tool.
- **A failure must never look like an empty ignore set.** Empty is indistinguishable from "this repo ignores nothing" and quietly disables filtering. Every bail-out returns `gitIgnoreSet{why: "..."}`, and the reason is surfaced in the banner. Never collapse distinct failures into one message — reporting "not a git repo" for a path plainly inside one asserts something false about the user's project.
- **Resolve symlinks before relating source to repo root.** git reports a resolved toplevel; the source may not be (`/tmp` is a symlink on macOS). Skipping this makes `filepath.Rel` return `../..`, every prefix check fails, and the set comes back empty.
- **Patterns are anchored to the transfer root**, which in the default nest mode sits one level *above* the source — so every path arrives as `<basename>/...` and a bare `/build/` matches nothing. `rsyncExcludes(source, contents)` owns this; don't hand-build patterns elsewhere.
- **Escape the assembled pattern, once** (`escapeRsyncPattern`), not its parts. Whether a backslash needs escaping is a property of the whole pattern: rsync only parses a pattern as a wildcard when it contains `*`, `?` or `[`, and compares literally otherwise — so escaping `\` unconditionally makes wildcard-free patterns stop matching.
- **`--from0` is banned here.** It would fix newlines in the pattern file but also switches the `.gitignore` dir-merge to NUL parsing, silently disabling the `--delete` protection below. Newline-bearing paths go to `unrepresentable` and are reported and copied instead.
- **`--filter=':-r .gitignore'` is load-bearing, not redundant.** The git-derived set only covers paths that exist in the source *now*, so ignored content living only at the destination (synced earlier, since deleted locally) would be wiped by `--delete`. The `r` modifier keeps it receiver-side so it never re-enters the sending decision. Same reason `--delete-excluded` is deliberately not passed.
- Known and accepted: ignored content inside a *nested* (non-submodule) git repo is opaque to `git ls-files` and gets copied. It fails safe; `--extra` trims it. Documented in README/CHANGELOG rather than fixed.

## Verified rsync behaviors (don't re-derive from the man page)

Checked against rsync 3.4.4; re-verify before relying on any of these elsewhere.

- Exclude patterns are matched against paths relative to the transfer root, which *includes* the source directory name unless `--contents` is used.
- `--info=progress2` combined with `-i` is unusable: the progress line rewrites with `\r` and shreds the itemized output. Pick one.
- An unescaped `[` is a character class even in an `--exclude-from` file, so a literal `weird[1].txt` pattern excludes `w1.txt` and misses its own file. Escaping the opening bracket alone is enough; `]` needs nothing.
- A receiver-side (`r`) filter rule protects destination files from `--delete` without affecting what gets sent.

## Subcommand conventions

- New management commands go under an `{area}` subcommand (`dest`, `repo`, `config`), dispatched from the top of `main()` ahead of `flag.Parse` so each subcommand owns its flags.
- Subcommand handlers return `int` (exit code); `main` calls `os.Exit`. Shared helpers live in `cmd_common.go`:
  - `failMsg(err) int` — prints `"rsync2project: <err>"` to stderr and returns 1. Use this in subcommands instead of the top-level `fail()` (which `os.Exit`s and short-circuits test coverage).
  - `parseSubFlags(fs, args) (stop bool, code int)` — wraps `fs.Parse` so `--help` returns exit 0 instead of 2. Every subcommand handler should use it in place of a direct `fs.Parse`.
  - `addDryRunFlag(fs, &dryRun)` — binds both `-n` and `--dry-run` to one bool. Use for any mutating subcommand so the alias contract stays in one place.
- Each subcommand uses its own `flag.NewFlagSet(name, flag.ContinueOnError)` so `-n`, `--format`, etc. bind only inside that subcommand's scope.
- For arguments that can be either a repo name or a source path, use `resolveRepoConfigArg` — it normalizes both forms to the canonical `.conf` path.
- Machine-readable output: prefer a `--format json` flag on the read-side subcommand rather than reshaping the default text output. Current precedent: `repo show --format json`.
