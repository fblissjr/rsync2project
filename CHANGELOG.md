# Changelog

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
