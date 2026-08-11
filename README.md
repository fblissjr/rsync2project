# rsync2project

A small Go wrapper around `rsync` that omits regenerable junk
(`node_modules/`, `__pycache__/`, `.venv/`, `target/`, `.gradle/`, build
caches, OS and editor cruft) when copying code projects between machines.
Honors each project's `.gitignore` by asking git, so negations and
tracked-but-matched files behave exactly as they do in git.

## Install

Requires `rsync` on both source and destination. `git` is optional — used
where available to resolve `.gitignore` exactly, with a documented
fallback when it isn't.

    go install github.com/fblissjr/rsync2project@latest

Or `go build -o rsync2project .` and drop the binary on your `PATH`.

## Usage

    rsync2project <source> <destination>
    rsync2project --dest NAME <source>
    rsync2project -n --show-excludes <source>

Flags: `-n`, `-v`, `-q/--quiet`, `--delete`, `--no-gitignore`,
`--no-excludes`, `--all`, `--no-vcs`, `--show-excludes`,
`--extra PATTERN`, `--include PATTERN`, `--save-config`,
`-d/--dest NAME`, `--contents`, `--list-dests`, `--version`.

Subcommands: `dest`, `repo`, `config path` — see sections below or
`rsync2project <subcmd> --help`.

By default the source directory is preserved at the destination (rsync's
native behavior). `rsync2project src/myapp /backup/` creates
`/backup/myapp/`. Pass `--contents` to spill the source's files directly
into the destination without the intermediate directory.

If the destination's last segment already repeats the source name — as in
`rsync2project . user@host:/path/myapp`, which lands the tree in
`/path/myapp/myapp/` — the run prints a note pointing at `--contents`
before starting. It's a note, not an error: a genuine `myapp/myapp/`
layout is legal, so the transfer proceeds.

### Two filter layers

Files get dropped by two independent mechanisms, and turning off one does
not turn off the other:

| Layer | What it drops | Turn off with |
| --- | --- | --- |
| The project's `.gitignore` | Exactly what `git` ignores | `--no-gitignore` |
| Builtin excludes | Regenerable junk: `node_modules/`, `__pycache__/`, `.venv/`, `target/`, `.gradle/`, plus per-project-type additions like `build/`+`dist/` for Python — and your own `excludes` file | `--no-excludes` |

`--all` turns off both. That is the flag for "copy the tree verbatim":

    rsync2project --contents --all . user@host:/path/myapp

`--no-gitignore` on its own is a common trap. In a Python project whose
`.gitignore` lists `models/ output/ __pycache__/ .venv/`, it brings back
`models/` and `output/` but *not* `__pycache__/` or `.venv/` — those are
in the builtin list, which is still live. Use `--all` when you mean
everything, or `--include PATTERN` when you mean a specific directory.

`--all` deliberately does not override `--no-vcs` or `--extra PATTERN`:
both are explicit requests made on the same command line.

`--show-excludes` prints the resolved state without transferring
anything, including the exact list of paths git reports as ignored.

### How `.gitignore` is honored

The ignore set comes from git itself (`git ls-files --others --ignored`),
not from replaying `.gitignore` through rsync's own filter engine. That
matters because rsync's filters are not gitignore, and the differences
lose files rather than copy extra ones:

- **Negation.** `!keep.log` re-includes a path in git. rsync's
  exclude-only dir-merge has no negation and reads the line as an
  exclude of a file literally named `!keep.log`, so anything you
  deliberately un-ignored disappears.
- **Tracked files.** Git never ignores a file it is tracking, even when
  a pattern matches it. rsync has no notion of the index, so a committed
  file matching an ignore rule silently vanishes from the copy.

Asking git also picks up `.git/info/exclude`, `core.excludesFile`, and
nested `.gitignore` precedence for free.

If the source is not a git work tree, or `git` isn't installed,
rsync2project falls back to the older approximate filter and says which
reason applied:

    rsync2project: .gitignore on (approximate rsync filter: not a git repo) | ...

The fallback changes which files transfer, which is why it's disclosed
rather than silent.

Two things worth knowing:

- **`--delete` still protects ignored content that only exists at the
  destination.** The git-derived set only covers paths present in the
  source, so something synced earlier and since deleted locally would
  otherwise be wiped. A receiver-side filter reading the destination's
  own `.gitignore` prevents that, matching the deliberate choice not to
  pass `--delete-excluded`.
- **Ignored content inside a *nested* git repo is not filtered.** A
  vendored checkout (a real repo, not a submodule) is opaque to
  `git ls-files`, so its ignored directories are copied. This errs
  toward copying too much, never too little; `--extra PATTERN` trims it.

### Seeing what moved

Every run prints, on stderr, where the tree is actually landing and which
filter layers are live:

    rsync2project: /src/myapp -> user@host:/path/myapp
    rsync2project: .gitignore off | builtin excludes off | 0 include, 0 exclude patterns

By default the transfer itself is itemized per changed path (rsync's
`-i` codes). `-n` always lists what *would* move, so it works as a real
preview. `-q/--quiet` restores the older progress-bar-only output, which
is quieter for very large transfers; `-n` still lists files under `-q`.

### Named destinations

Add a destination from the CLI:

    rsync2project dest add mac fred@mac.local:/Users/fred/backup/

Then `--dest mac` (or `-d mac`). Other `dest` subcommands:

    rsync2project dest              # list (same as --list-dests)
    rsync2project dest show NAME    # print one destination's value
    rsync2project dest add NAME VAL # add or update
    rsync2project dest rm NAME      # remove
    rsync2project dest add -n ...   # dry-run; prints what would be written

These edit `~/.config/rsync2project/destinations` in place, preserving
comments. You can also hand-edit the file directly — one `name=target`
per line:

    name=user@host:/path/

### Inspecting or cleaning up per-repo configs

    rsync2project repo                            # list saved repo configs
    rsync2project repo show myapp                 # print one (text)
    rsync2project repo show --format json myapp   # ... as JSON for scripts
    rsync2project repo rm myapp                   # delete one
    rsync2project repo path myapp                 # print its file path
    rsync2project config path                     # print the config dir

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

Prefer `--include` over `--all` when you want a named directory back but
still want the junk filtered — and `--save-config` so you only decide
once.

### Extra global excludes

Optional `~/.config/rsync2project/excludes`, one rsync pattern per line.

## License

See LICENSE.
