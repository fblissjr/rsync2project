package main

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

type marker struct {
	pattern string // glob matched against the basename
	typ     string
}

// markers maps filename/directory patterns to project type labels. Multiple
// patterns can map to the same type; detection deduplicates.
var markers = []marker{
	{"pyproject.toml", "python"},
	{"setup.py", "python"},
	{"setup.cfg", "python"},
	{"requirements.txt", "python"},
	{"Pipfile", "python"},
	{"package.json", "node"},
	{"bun.lockb", "bun"},
	{"pnpm-lock.yaml", "node"},
	{"yarn.lock", "node"},
	{"go.mod", "go"},
	{"Cargo.toml", "rust"},
	{"pom.xml", "java"},
	{"build.gradle", "java"},
	{"build.gradle.kts", "java"},
	{"*.csproj", "dotnet"},
	{"*.fsproj", "dotnet"},
	{"*.sln", "dotnet"},
	{"Gemfile", "ruby"},
	{"composer.json", "php"},
	{"*.xcodeproj", "xcode"},
	{"*.xcworkspace", "xcode"},
	{"mix.exs", "elixir"},
}

// heavyDirs are never worth descending into while detecting project types.
// They're expensive to walk and cannot themselves indicate a new project type
// (a package.json inside node_modules doesn't mean the whole tree is a Node
// project — the parent already signalled that).
var heavyDirs = map[string]struct{}{
	"node_modules":  {},
	".venv":         {},
	"venv":          {},
	"target":        {},
	".git":          {},
	".gradle":       {},
	".next":         {},
	".nuxt":         {},
	".turbo":        {},
	".svelte-kit":   {},
	".parcel-cache": {},
	".pnpm-store":   {},
	"__pycache__":   {},
	".pytest_cache": {},
	".mypy_cache":   {},
	".ruff_cache":   {},
	".tox":          {},
	".nox":          {},
}

// detectProjectTypes scans up to three directory levels deep under root for
// known project markers and returns the unique set of detected types, sorted
// alphabetically. Three levels is enough for typical monorepo layouts like
// apps/<name>/package.json or services/<name>/go.mod.
func detectProjectTypes(root string) []string {
	const maxDepth = 3
	seen := make(map[string]struct{})

	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		depth := 0
		if rel != "." {
			depth = strings.Count(rel, string(filepath.Separator)) + 1
		}

		if rel != "." {
			for _, m := range markers {
				if matched, _ := filepath.Match(m.pattern, d.Name()); matched {
					seen[m.typ] = struct{}{}
				}
			}
		}

		if d.IsDir() {
			if rel != "." {
				if _, skip := heavyDirs[d.Name()]; skip {
					return filepath.SkipDir
				}
			}
			if depth >= maxDepth {
				return filepath.SkipDir
			}
		}
		return nil
	})

	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
