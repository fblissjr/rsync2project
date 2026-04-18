package main

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

type projectType string

const (
	ptPython projectType = "python"
	ptNode   projectType = "node"
	ptBun    projectType = "bun"
	ptGo     projectType = "go"
	ptRust   projectType = "rust"
	ptJava   projectType = "java"
	ptDotnet projectType = "dotnet"
	ptRuby   projectType = "ruby"
	ptPHP    projectType = "php"
	ptXcode  projectType = "xcode"
	ptElixir projectType = "elixir"
)

type marker struct {
	pattern string // glob matched against the basename
	typ     projectType
}

// markers maps filename/directory patterns to project type labels.
var markers = []marker{
	{"pyproject.toml", ptPython},
	{"setup.py", ptPython},
	{"setup.cfg", ptPython},
	{"requirements.txt", ptPython},
	{"Pipfile", ptPython},
	{"package.json", ptNode},
	{"bun.lockb", ptBun},
	{"pnpm-lock.yaml", ptNode},
	{"yarn.lock", ptNode},
	{"go.mod", ptGo},
	{"Cargo.toml", ptRust},
	{"pom.xml", ptJava},
	{"build.gradle", ptJava},
	{"build.gradle.kts", ptJava},
	{"*.csproj", ptDotnet},
	{"*.fsproj", ptDotnet},
	{"*.sln", ptDotnet},
	{"Gemfile", ptRuby},
	{"composer.json", ptPHP},
	{"*.xcodeproj", ptXcode},
	{"*.xcworkspace", ptXcode},
	{"mix.exs", ptElixir},
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
func detectProjectTypes(root string) []projectType {
	const maxDepth = 3
	seen := make(map[projectType]struct{})

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

	out := make([]projectType, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
