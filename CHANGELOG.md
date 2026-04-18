# Changelog

## [0.5.0]

### Added
- Subcommand layer. Existing flag-style invocations are unaffected; dispatch for known subcommands runs ahead of top-level flag parsing so each subcommand owns its own flags.
- `rsync2project dest` — manage `~/.config/rsync2project/destinations` without hand-editing:
  - `dest` / `dest list` — list destinations (same output as `--list-dests`).
  - `dest add NAME VALUE` — add or update. Existing entries are replaced in place, preserving comments and surrounding lines; new entries are appended. Writes atomically via tmpfile + fsync + rename.
  - `dest rm NAME` — remove an entry. Errors if the name is unknown.
- `rsync2project repo` — inspect and clean up per-repo configs in `~/.config/rsync2project/repos/`:
  - `repo` / `repo list` — list saved repo configs with their source paths (read from each file's `# source:` header).
  - `repo show NAME|PATH` — print a repo config file.
  - `repo rm NAME|PATH` — remove a repo config file.
  - `repo path [NAME|PATH]` — print the repos directory or a specific config file path.
  - Saving stays on `--save-config` because it needs a live sync invocation for its context.
- `rsync2project config path` — print the config directory (`$XDG_CONFIG_HOME/rsync2project` or `~/.config/rsync2project`) for scripting and quick `cd`.
- Each mutating subcommand accepts `-n` / `--dry-run` to preview without writing, matching the project-wide "`-n` means zero side effects" convention.

## [0.4.1]

### Fixed
- `-n` (`--dry-run`) now also skips `--save-config` writes, matching the universal "dry-run means zero side effects" convention. Prints `dry-run: would save to ...` instead of writing.
- `saveRepoConfig` refuses to overwrite an existing per-repo file whose `# source:` header names a different absolute path. Protects against two source directories with the same basename (e.g. `~/work/myapp` and `~/play/myapp`) silently clobbering each other's config.

## [0.4.0]

### Added
- Per-repo config at `~/.config/rsync2project/repos/<basename>.conf`, stored centrally (not in the source tree — no risk of accidentally committing it). Supports:
  - `dest = NAME` directive: pins a default destination for this repo, so repeat syncs become `rsync2project <source>` with no other flags.
  - Non-comment lines are rsync include patterns that override `.gitignore` and baseline excludes. Trailing `/` on a pattern auto-expands to include directory contents (`X/` + `X/***`).
- `--include PATTERN` CLI flag (repeatable): one-off re-include of a path that `.gitignore` or the baseline excludes would otherwise drop.
- `--save-config` flag: writes the current `--dest` and `--include` choices to the repo config file, merging with any existing contents. Intended workflow: experiment with flags, then `--save-config` once the combination is right, then re-run with no flags forever after.

### Changed
- Destination priority is now: explicit `--dest` > positional > repo config `dest = ...`. Explicit flags always win.
- `--show-excludes` now also prints any active re-include patterns alongside excludes.

## [0.3.0]

### Changed
- **Breaking:** default path semantic flipped to match rsync's native behavior. The source directory is now preserved at the destination (nested under its own name) by default, instead of having its contents spilled directly into the destination. `rsync2project ~/code/myapp /backup/` now creates `/backup/myapp/` rather than `/backup/{main.go, ...}`.
- The `--keep-name` flag is removed. Its behavior is now the default.

### Added
- `--contents` flag to opt into the old "spill source contents directly into destination" behavior, for cases where the destination path already names the target (e.g. a dev mirror `ubuntu:~/code/myapp/`) or where you want to rename at the destination.
- `build/` and `dist/` added to the project-type excludes for detected Python projects.

## [0.2.0]

### Added
- `--keep-name` flag: pass the source through without an auto-appended trailing slash, so the source directory nests under the destination instead of spilling its contents.
- Integration tests that invoke real `rsync` to verify the exclude list and `--keep-name` behavior end-to-end; skipped automatically when `rsync` is not on `PATH`.

### Changed
- Clarified in-code comment on why `target/` is always excluded and what to do if a project has a legitimate top-level `target/` directory.

## [0.1.0]

### Added
- Initial release.
- Wraps `rsync` with curated excludes for Python, Node, Bun, Rust, Go, Java, .NET, Ruby, PHP, Xcode, and Elixir projects.
- Detects project type by scanning marker files up to two directory levels deep.
- Respects each project's `.gitignore` via rsync's per-directory filter.
- Named destinations from `~/.config/rsync2project/destinations`.
- Extra global excludes from `~/.config/rsync2project/excludes`.
- Flags: `--dry-run`, `--verbose`, `--delete`, `--no-gitignore`, `--no-vcs`, `--show-excludes`, `--extra`, `--dest`, `--list-dests`, `--version`.
- Auto-enables compression (`-z`) when the destination looks remote.
- Auto-appends trailing slash to source so contents flow into the destination.
