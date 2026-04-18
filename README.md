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
`--show-excludes`, `--extra PATTERN`, `--include PATTERN`,
`--save-config`, `-d/--dest NAME`, `--contents`, `--list-dests`,
`--version`.

By default the source directory is preserved at the destination (rsync's
native behavior). `rsync2project ~/code/myapp /backup/` creates
`/backup/myapp/`. Pass `--contents` to spill the source's files directly
into the destination without the intermediate directory.

### Named destinations

Add a destination from the CLI:

    rsync2project dest add mac fred@mac.local:/Users/fred/backup/

Then `--dest mac` (or `-d mac`). Other `dest` subcommands:

    rsync2project dest              # list (same as --list-dests)
    rsync2project dest add NAME VAL # add or update
    rsync2project dest rm NAME      # remove
    rsync2project dest add -n ...   # dry-run; prints what would be written

These edit `~/.config/rsync2project/destinations` in place, preserving
comments. You can also hand-edit the file directly — one `name=target`
per line:

    name=user@host:/path/

### Inspecting or cleaning up per-repo configs

    rsync2project repo              # list saved repo configs
    rsync2project repo show myapp   # print one
    rsync2project repo rm myapp     # delete one
    rsync2project repo path myapp   # print its file path
    rsync2project config path       # print the config dir

Saving per-repo config still goes through `--save-config` on a sync
invocation — see the next section.

### Per-repo config (persist your settings)

Figure out the right flags once, then save them so future syncs are a
single command:

    rsync2project --save-config --dest nas --include internal/ ~/code/myapp

Writes `~/.config/rsync2project/repos/myapp.conf`. Subsequent runs can
omit the flags:

    rsync2project ~/code/myapp

The file lives in the central config dir — not in the source tree — so
it can't be accidentally committed. Format:

    # directives
    dest = nas

    # rsync include patterns (override .gitignore and baseline excludes)
    internal/
    models/weights.bin

A trailing `/` on a pattern auto-expands to include the directory's
contents. Command-line `--dest` / `--include` always override anything
in the file.

Safety: `--save-config` refuses to overwrite an existing per-repo file
whose header names a different absolute source path (protects against
two repos with the same basename clobbering each other), and `-n`
makes `--save-config` a no-op (prints what would be saved, writes
nothing).

### Re-including gitignored content

`--include PATTERN` (or a line in the per-repo config) re-includes paths
that `.gitignore` or the baseline excludes would otherwise drop. Useful
for personal backups: `models/`, `data/raw/`, and `.env` files stay out
of GitHub but still land on your NAS.

### Extra global excludes

Optional `~/.config/rsync2project/excludes`, one rsync pattern per line.

## License

See LICENSE.
