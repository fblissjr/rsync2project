# Changelog

## [0.7.0]

### Fixed
- `.gitignore` is now honored by asking git, not by replaying the file through rsync's filter engine. The old `--filter=':- .gitignore'` was an approximation that diverged in two ways, both of which *lost* files:
  - **Negation.** Git's `!pattern` re-includes a path. rsync's exclude-only dir-merge has no negation, so the line was read as an exclude of a file literally named `!pattern`, and anything a project deliberately un-ignored was silently dropped.
  - **Tracked files.** Git never ignores a file it is tracking, even when a pattern matches it. rsync has no notion of the index, so a committed file matching an ignore rule vanished from the copy — the worst failure mode for a backup tool.
- Literal path metacharacters are escaped. An ignored file named `weird[1].txt` previously produced a character class that excluded `w1.txt` and left the actual file behind, so the exclude landed on the wrong file entirely.
- Git-derived patterns are anchored to the transfer root, which in the default (nest) mode sits one level above the source. An unanchored pattern matches nothing there, so the exclude would have silently no-opped.
- Sources reached through a symlink (`/tmp` → `/private/tmp` on macOS, many home-directory setups) are resolved before being related to the repo root. Without this the path mapping fails and the ignore set comes back empty, which reads as "nothing is ignored".

### Added
- Asking git also picks up `.git/info/exclude`, `core.excludesFile`, and nested `.gitignore` precedence, none of which the old approximation handled the same way.
- `--show-excludes` lists the git-derived ignore set — the direct answer to "why didn't this file copy?".
- The banner reports which mode ran: `on (via git)` or `on (approximate: rsync filter, no git repo)`.

### Changed
- Sources that are not a git work tree, or systems without `git`, fall back to the previous approximate filter rather than failing. The fallback is disclosed in the banner and in `--show-excludes`, since it changes which files transfer.
- When an enclosing repo ignores the source directory itself, git has nothing to say about its contents. That is reported and everything is copied, rather than emitting a pattern that would exclude the entire transfer.

## [0.6.0]

### Added
- `--all` — copy the tree verbatim. Implies `--no-gitignore` and `--no-excludes`. This is the flag for "I want the gitignored folders too"; `--no-gitignore` alone never was.
- `--no-excludes` — drop the builtin, project-type, and user `excludes`-file layers while leaving the `.gitignore` filter alone.
- `-q` / `--quiet` — progress-bar-only output (the pre-0.6 default) for very large transfers.
- Run banner on stderr: the source, the path the tree will *actually* land in, and which filter layers are live. Neither fact was previously observable — rsync reports neither the nested landing path nor why a directory failed to appear at the far end.
- Note when the destination's last segment already repeats the source basename (`rsync2project . host:/path/myapp` → `/path/myapp/myapp/`), pointing at `--contents`. A warning, not an error: `myapp/myapp/` is a legal layout.

### Changed
- Default output now itemizes each changed path (`-i`) instead of showing only a progress counter. A run that transferred nothing used to look identical to one that transferred everything.
- `-n` always lists what would move, including under `-q`. A preview whose file list is suppressed has no content left.
- `--no-vcs` and `--extra PATTERN` survive `--no-excludes`/`--all` — both are explicit requests made on the same command line, so "copy everything" does not silently undo them.
- `--show-excludes` reports the builtin-exclude layer state alongside the gitignore state.

### Fixed
- `--no-gitignore` silently kept applying ~35 builtin excludes, so gitignored directories that overlapped that list (`__pycache__/`, `.venv/`, `node_modules/`, `target/`, and `build/`+`dist/` on Python projects) still never transferred, with nothing printed to say why. The two layers are now separately controllable and their state is reported on every run.

## [0.5.1]

### Added
- `rsync2project dest show NAME` — print a single destination's value. Useful for scripting (e.g. `$(rsync2project dest show mac)`) and fills the symmetry gap with `repo show`.
- `rsync2project repo show --format json NAME|PATH` — machine-readable output with `path`, `source` (from the `# source:` header), and `content` fields. Default remains human-readable text.

### Fixed
- `rsync2project dest add --help` (and all other subcommand `--help` variants) now exit 0 instead of 2. The `flag.ContinueOnError` mode returns `flag.ErrHelp` from `Parse`, which was being treated as a generic parse failure.

### Changed
- Extracted `parseSubFlags` / `failMsg` into `cmd_common.go` so every subcommand shares the help-exit-code handling and error formatting.
- `config` subcommand lifted to the same per-subcommand FlagSet shape as `dest`/`repo`; adding a second `config` op is now a single case.

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
