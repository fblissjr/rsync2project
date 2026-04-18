---
name: bump-version
description: Bump rsync2project version across main.go, CHANGELOG.md, and (if present) internal/ personal docs. Use when preparing a release or finishing a feature pass. Takes the new semver version as the argument.
disable-model-invocation: true
---

# bump-version

You are performing the version-bump ritual documented in `CLAUDE.md`. The user has passed a target semver version (e.g. `0.5.2`) as the skill argument.

## Steps

Do these in order. Stop and ask the user only if something unexpected comes up; otherwise execute straight through.

### 1. Confirm the jump makes sense

- Read the current version from `main.go` (the `const version = "X.Y.Z"` line).
- If the target is identical, stop and tell the user.
- If the target is a major bump from the current version, stop and confirm — the global convention is no major bumps without explicit permission.

### 2. Update `main.go`

Replace the `const version = "..."` line with the new version.

### 3. Update `CHANGELOG.md`

Insert a new `## [X.Y.Z]` section immediately under the top `# Changelog` heading, above the previous entry. If the user supplied changelog bullets in the invocation, use those. Otherwise, check the git log since the previous tag/version for unreleased commits:

```
git log --oneline $(git describe --tags --abbrev=0 2>/dev/null || echo main)..HEAD
```

Summarize the commits into `### Added` / `### Changed` / `### Fixed` sections. No dates (per the project's CHANGELOG convention — semver only).

### 4. Refresh `internal/` if present

`internal/` is gitignored; skip this step if the directory doesn't exist.

- **`internal/tutorial.md`**: update the `last updated: YYYY-MM-DD (vX.Y.Z)` line and any `rsync2project X.Y.Z` version references and the Reference-section version line.
- **`internal/log/log_<today>.md`**: if today's log exists, append a short timeline entry describing the bump and what shipped. If no log for today exists, don't create one — the user writes session logs on their own cadence.

### 5. Verify the build

Run:

```
go build -o rsync2project .
go test ./... -count=1
```

Both must pass. If either fails, surface the error and stop.

### 6. Report

Print a short summary: which files changed, which version was set, and a reminder that the user still needs to review and commit. Do NOT auto-commit — the user's global convention is "committing is fine without asking, but never push unless explicitly told."

## Notes

- Follow the existing CHANGELOG bullet style (concise, past-tense, grouped under Added/Changed/Fixed).
- Preserve exact formatting quirks of `CHANGELOG.md` (blank lines between sections, etc.).
- Do not touch `README.md` unless a user-visible feature changed — the README tracks features, not version numbers.
