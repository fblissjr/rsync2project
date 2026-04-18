---
name: add-subcommand
description: Scaffold a new top-level subcommand under the dest/repo/config pattern. Generates cmd_NAME.go, cmd_NAME_test.go, the dispatch case in main.go, usage text, and README/CHANGELOG entries. Use when growing the CLI's management surface.
---

# add-subcommand

You are adding a new subcommand to rsync2project. Read `cmd_dest.go`, `cmd_repo.go`, and `cmd_common.go` first — they are the template. Do NOT invent new patterns; the consistency is the point.

## Inputs to gather

Before generating code, ask the user (unless already specified in the invocation):

1. **Subcommand name** (e.g. `audit`, `backup`, `queue`). Lowercase, single word.
2. **Sub-operations** (e.g. `list`, `show`, `add`, `rm`, `path`). Which read-only, which mutating.
3. **Arg shape** of each mutating op — what positional args? Any flags beyond `-n`?
4. **One-line purpose** for the top-level `<name>Usage` help text.

## Files to create

### `cmd_NAME.go`

Structure it exactly like `cmd_dest.go`:

- Package `main`, imports `flag`, `fmt`, `io`, `os` (plus anything op-specific).
- Top-level `runNAMECmd(args []string) int` that dispatches on `args[0]`:
  - Default (no args) → the list/show op.
  - `-h`, `--help`, `help` → print usage, return 0.
  - Unknown → print usage to stderr, return 2.
- Per-op handlers `runNAMEOp(args []string) int`:
  - Own `flag.NewFlagSet(name, flag.ContinueOnError)` with `fs.SetOutput(os.Stderr)` and an `fs.Usage` closure.
  - Always call `parseSubFlags(fs, args)` instead of `fs.Parse` directly — otherwise `--help` exits 2.
  - For mutating ops, use `addDryRunFlag(fs, &dryRun)` from `cmd_common.go` so `-n`/`--dry-run` stay aliased.
  - Use `failMsg(err)` for errors instead of hand-writing `fmt.Fprintln(os.Stderr, "rsync2project:", err); return 1`.
- `NAMEUsage(w io.Writer)` function that prints a subcommand help block matching the style of `destUsage` / `repoUsage`.

### `cmd_NAME_test.go`

Minimum coverage (mirror `cmd_dest_test.go`):

- Help exit codes: every sub-op with `--help` returns 0.
- Unknown subcommand: `runNAMECmd([]string{"frobnicate"})` returns 2.
- Happy path: one test per op, asserting exit code and (where applicable) filesystem state.
- Dry-run: for every mutating op, assert `-n` leaves no side effects.

Use `t.Setenv("XDG_CONFIG_HOME", t.TempDir())` at the top of every test — the project convention ensures tests can't clobber real config.

### `main.go` dispatch

Add a case to the top-of-main() switch, ahead of `flag.Parse`:

```go
case "NAME":
    os.Exit(runNAMECmd(os.Args[2:]))
```

Keep the case order alphabetical.

### Top-level usage

Add a line to the `Subcommands:` block in the `usage()` function in `main.go`:

```
  NAME                  One-line purpose (see 'rsync2project NAME --help')
```

### README

Add a short subsection under `### Inspecting or cleaning up per-repo configs` (or wherever fits thematically) with example invocations.

### CHANGELOG

Add a bullet under the next unreleased version section (or create one if none exists) describing the new subcommand and its ops. Match existing tone: concise, past-tense, one line per op.

## Finally

Run:

```
go build -o rsync2project .
go test ./... -count=1
```

Both must pass. Then report: files created, tests added, and suggest the user review before committing. Don't auto-commit.

## Anti-patterns to avoid

- Skipping `parseSubFlags` → `--help` exits 2, breaks users' muscle memory.
- Calling `fail()` (the top-level helper) from a subcommand → `os.Exit` inside a test kills the test run.
- Reading state files directly when a resolver exists (`resolveDestination`, `resolveRepoConfigArg`). If your new subcommand operates on destinations or repo configs, route through those.
- Inventing a new format for machine-readable output. If one is needed, follow `repo show --format json`.
