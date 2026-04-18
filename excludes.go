package main

// alwaysExclude is the baseline set of patterns excluded regardless of project
// type. These are universally regenerable, OS-specific, or editor cruft and
// should never need to be transferred between machines.
var alwaysExclude = []string{
	// OS / filesystem metadata
	".DS_Store",
	"._*",
	"Thumbs.db",
	"desktop.ini",

	// Editor swap / temp
	"*.swp",
	"*.swo",
	"*~",

	// Python virtualenvs
	".venv/",
	"venv/",

	// Python compiled / caches
	"__pycache__/",
	"*.pyc",
	"*.pyo",
	"*.pyd",
	".pytest_cache/",
	".mypy_cache/",
	".ruff_cache/",
	".tox/",
	".nox/",
	".coverage",
	".coverage.*",
	"htmlcov/",
	"*.egg-info/",
	".eggs/",

	// JS / TS / Node / Bun / PNPM / Yarn
	"node_modules/",
	".next/",
	".nuxt/",
	".turbo/",
	".svelte-kit/",
	".parcel-cache/",
	".expo/",
	".pnpm-store/",
	".yarn/cache/",
	".yarn/install-state.gz",

	// Rust / Maven / SBT build output. Tradeoff: always excluded because
	// it's overwhelmingly a build artifact directory. A project with a
	// legitimate top-level target/ source folder isn't well-served by this
	// tool; use rsync directly with a custom filter.
	"target/",

	// Gradle
	".gradle/",
}

// vcsExclude excludes version-control metadata. Applied only when --no-vcs.
var vcsExclude = []string{
	".git/",
	".svn/",
	".hg/",
}

// projectTypeExcludes are additions applied only if the project type is
// detected. These patterns are ambiguous enough that we don't want to include
// them unconditionally (e.g. bin/ is a build dir for .NET but may be source
// code for other projects).
var projectTypeExcludes = map[string][]string{
	"dotnet": {"bin/", "obj/"},
	"xcode": {
		"DerivedData/",
		"*.xcworkspace/xcuserdata/",
		"*.xcodeproj/xcuserdata/",
	},
}

func buildExcludes(detected []string, opts *options) []string {
	out := make([]string, 0, len(alwaysExclude)+16)
	out = append(out, alwaysExclude...)
	for _, t := range detected {
		if extra, ok := projectTypeExcludes[t]; ok {
			out = append(out, extra...)
		}
	}
	if opts.excludeVCS {
		out = append(out, vcsExclude...)
	}
	return out
}
