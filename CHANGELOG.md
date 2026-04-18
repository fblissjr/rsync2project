# Changelog

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
