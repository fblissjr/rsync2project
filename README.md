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
`--show-excludes`, `--extra PATTERN`, `-d/--dest NAME`, `--contents`,
`--list-dests`, `--version`.

By default the source directory is preserved at the destination (rsync's
native behavior). `rsync2project ~/code/myapp /backup/` creates
`/backup/myapp/`. Pass `--contents` to spill the source's files directly
into the destination without the intermediate directory.

### Named destinations

Create `~/.config/rsync2project/destinations`, one `name=target` per line:

    name=user@host:/path/

Then `--dest name`.

### Extra global excludes

Optional `~/.config/rsync2project/excludes`, one rsync pattern per line.

## License

See LICENSE.
