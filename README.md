# rsync2project

A small Go wrapper around `rsync` that omits regenerable junk
(`node_modules/`, `__pycache__/`, `.venv/`, `target/`, `.gradle/`, build
caches, OS and editor cruft) when copying code projects between machines.
Honors each project's `.gitignore`.

## Install

Requires `rsync` on both source and destination.

    go install github.com/fblissjr/rsync2project@latest

Or `go build -o rsync2project .` and drop the binary on your `PATH`.

## Usage

    rsync2project <source> <destination>
    rsync2project --dest NAME <source>
    rsync2project -n --show-excludes <source>

Flags: `-n`, `-v`, `--delete`, `--no-gitignore`, `--no-vcs`,
`--show-excludes`, `--extra PATTERN`, `-d/--dest NAME`, `--keep-name`,
`--list-dests`, `--version`.

A trailing slash is appended to the source by default, so the contents of
the source directory land directly in the destination. Pass `--keep-name`
if you want the source directory itself to appear under the destination.

### Named destinations

Create `~/.config/rsync2project/destinations`, one `name=target` per line:

    name=user@host:/path/

Then `--dest name`.

### Extra global excludes

Optional `~/.config/rsync2project/excludes`, one rsync pattern per line.

## License

See LICENSE.
