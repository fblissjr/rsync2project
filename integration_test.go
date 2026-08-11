package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func requireRsync(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync not available on PATH; skipping integration test")
	}
}

// setupFakeProject creates a project directory named "testproj" inside a
// fresh temp parent and populates it with the given relative paths. Returns
// the absolute source path.
func setupFakeProject(t *testing.T, files map[string]string) string {
	t.Helper()
	parent := t.TempDir()
	src := filepath.Join(parent, "testproj")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	for rel, content := range files {
		mustWrite(t, filepath.Join(src, rel), content)
	}
	return src
}

// TestIntegrationSyncDefault shells out to rsync and verifies both the
// curated exclude list and the default "nest under source name" behavior.
func TestIntegrationSyncDefault(t *testing.T) {
	requireRsync(t)

	src := setupFakeProject(t, map[string]string{
		"pyproject.toml":                  "[project]\nname=\"x\"\n",
		"main.py":                         "print('hello')\n",
		"package.json":                    "{}\n",
		"src/app.ts":                      "export {};\n",
		".venv/bin/python":                "binary",
		"node_modules/left-pad/index.js":  "module.exports = x => x;\n",
		"src/__pycache__/app.cpython.pyc": "compiled",
		"pkg.egg-info/PKG-INFO":           "Metadata-Version: 2.1\n",
		".DS_Store":                       "mac-metadata",
		"build/lib/module.py":             "build artifact",
		"dist/wheel.whl":                  "wheel artifact",
	})
	dst := t.TempDir()

	types := detectProjectTypes(src)
	opts := &options{}
	excludes := buildExcludes(types, opts.excludeVCS)

	if err := runRsync(src, dst+"/", nil, excludes, resolveGitignoreFilter(src, opts), opts); err != nil {
		t.Fatalf("runRsync: %v", err)
	}

	// With the default (nest) behavior, everything lands under dst/testproj/.
	nest := filepath.Join(dst, "testproj")

	mustExist := []string{"main.py", "package.json", "src/app.ts"}
	mustMiss := []string{
		".venv",
		"node_modules",
		"src/__pycache__",
		"pkg.egg-info",
		".DS_Store",
		"build",
		"dist",
	}
	for _, rel := range mustExist {
		if _, err := os.Stat(filepath.Join(nest, rel)); err != nil {
			t.Errorf("expected %s to be copied under nest, stat error: %v", rel, err)
		}
	}
	for _, rel := range mustMiss {
		if _, err := os.Stat(filepath.Join(nest, rel)); err == nil {
			t.Errorf("expected %s to be excluded, but it exists at destination", rel)
		} else if !os.IsNotExist(err) {
			t.Errorf("unexpected error checking %s: %v", rel, err)
		}
	}
}

// TestIntegrationContents verifies --contents spills the source's files
// directly into the destination without the intermediate directory.
func TestIntegrationContents(t *testing.T) {
	requireRsync(t)

	src := setupFakeProject(t, map[string]string{"file.txt": "hi\n"})
	dst := t.TempDir()

	opts := &options{contents: true}
	if err := runRsync(src, dst+"/", nil, nil, resolveGitignoreFilter(src, opts), opts); err != nil {
		t.Fatalf("runRsync: %v", err)
	}

	direct := filepath.Join(dst, "file.txt")
	if _, err := os.Stat(direct); err != nil {
		t.Errorf("expected file directly in dest at %s, got: %v", direct, err)
	}
	nested := filepath.Join(dst, "testproj")
	if _, err := os.Stat(nested); err == nil {
		t.Errorf("did not expect nested dir %s with --contents, but it exists", nested)
	}
}

// TestIntegrationRepoIncludeBeatsGitignore simulates the real case: a
// directory is gitignored because it doesn't belong on GitHub, but the user
// wants it copied to a personal backup destination. A repo config file at
// ~/.config/rsync2project/repos/<basename>.conf re-includes it, while
// baseline regenerable excludes still fire.
func TestIntegrationRepoIncludeBeatsGitignore(t *testing.T) {
	requireRsync(t)

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	src := setupFakeProject(t, map[string]string{
		"pyproject.toml":     "[project]\nname=\"x\"\n",
		".gitignore":         "models/\ninternal/\n.venv/\n",
		"main.py":            "print('hi')\n",
		"models/weights.bin": "fake weights",
		"internal/notes.md":  "private\n",
		".venv/bin/python":   "regenerable",
		"__pycache__/a.pyc":  "regenerable",
	})
	dst := t.TempDir()

	// Simulate the "configure once, run many times" workflow: save a repo
	// config that re-includes models/ and internal/, then reload and run.
	if err := saveRepoConfig(src, &repoConfig{}, "", []string{"models/", "internal/"}); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadRepoConfig(src)
	if err != nil {
		t.Fatal(err)
	}
	includes := expandIncludePatterns(cfg.rawIncludes)
	excludes := buildExcludes(detectProjectTypes(src), false)

	if err := runRsync(src, dst+"/", includes, excludes, resolveGitignoreFilter(src, &options{}), &options{}); err != nil {
		t.Fatalf("runRsync: %v", err)
	}

	nest := filepath.Join(dst, "testproj")

	mustExist := []string{
		"main.py",
		"models/weights.bin",
		"internal/notes.md",
	}
	mustMiss := []string{
		".venv",
		"__pycache__",
	}
	for _, rel := range mustExist {
		if _, err := os.Stat(filepath.Join(nest, rel)); err != nil {
			t.Errorf("expected %s to be copied (re-included), stat error: %v", rel, err)
		}
	}
	for _, rel := range mustMiss {
		if _, err := os.Stat(filepath.Join(nest, rel)); err == nil {
			t.Errorf("expected %s to be excluded as regenerable, but it exists", rel)
		} else if !os.IsNotExist(err) {
			t.Errorf("unexpected error checking %s: %v", rel, err)
		}
	}
}

// TestIntegrationAllCopiesEverything covers the case --no-gitignore alone
// does not: a project whose gitignored directories overlap the builtin
// exclude tables. --no-gitignore lifts only the .gitignore layer, so
// __pycache__/ and .venv/ still vanish; --all lifts both and the tree
// arrives verbatim.
func TestIntegrationAllCopiesEverything(t *testing.T) {
	requireRsync(t)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	files := map[string]string{
		"requirements.txt":   "torch\n",
		".gitignore":         "models/\noutput/\n__pycache__/\n.venv/\n",
		"nodes.py":           "print('hi')\n",
		"models/weights.bin": "fake weights",
		"output/run1.png":    "fake png",
		"__pycache__/a.pyc":  "compiled",
		".venv/bin/python":   "regenerable",
	}
	types := detectProjectTypes(setupFakeProject(t, files))

	// Overlapping with .gitignore only: models/ and output/ come back,
	// but the builtin tables still swallow __pycache__/ and .venv/.
	srcA := setupFakeProject(t, files)
	dstA := t.TempDir()
	optsA := &options{noGitignore: true}
	if err := runRsync(srcA, dstA+"/", nil, buildExcludeList(types, nil, optsA), resolveGitignoreFilter(srcA, optsA), optsA); err != nil {
		t.Fatalf("runRsync (--no-gitignore): %v", err)
	}
	nestA := filepath.Join(dstA, "testproj")
	assertPresent(t, nestA, "--no-gitignore", []string{"models/weights.bin", "output/run1.png"})
	assertAbsent(t, nestA, "--no-gitignore", []string{"__pycache__", ".venv"})

	// --all lifts both layers: nothing is filtered out.
	srcB := setupFakeProject(t, files)
	dstB := t.TempDir()
	optsB := &options{noGitignore: true, noExcludes: true}
	if err := runRsync(srcB, dstB+"/", nil, buildExcludeList(types, nil, optsB), resolveGitignoreFilter(srcB, optsB), optsB); err != nil {
		t.Fatalf("runRsync (--all): %v", err)
	}
	nestB := filepath.Join(dstB, "testproj")
	assertPresent(t, nestB, "--all", []string{
		"nodes.py",
		"models/weights.bin",
		"output/run1.png",
		"__pycache__/a.pyc",
		".venv/bin/python",
	})
}

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available on PATH; skipping git-backed test")
	}
}

// setupGitRepo builds a real repo so the git-backed ignore path is
// exercised for real rather than mocked. tracked paths are force-added, so
// a file can be both committed and matched by an ignore rule — the case
// that distinguishes git's semantics from rsync's.
func setupGitRepo(t *testing.T, files map[string]string, tracked []string) string {
	t.Helper()
	// Neutralize the developer's own git config: a global core.excludesFile
	// would otherwise leak into the ignore set and make results
	// machine-dependent.
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	src := setupFakeProject(t, files)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = src
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run(append([]string{"add", "-f", "--"}, tracked...)...)
	run("-c", "user.email=test@example.invalid", "-c", "user.name=test",
		"commit", "-qm", "init")
	return src
}

// TestIntegrationGitignoreFidelity pins the two ways rsync's dir-merge
// approximation diverges from git, both of which silently lose files:
// a negated ("!") re-include, and a tracked file that an ignore rule
// matches. Git keeps both; the old --filter=':- .gitignore' dropped both.
func TestIntegrationGitignoreFidelity(t *testing.T) {
	requireRsync(t)
	requireGit(t)

	files := map[string]string{
		".gitignore":              "*.log\n!logs/KEEP.log\nbuild/\n",
		"src/main.py":             "print('hi')\n",
		"logs/app.log":            "noise",
		"logs/KEEP.log":           "deliberately un-ignored",
		"build/out.o":             "artifact",
		"tracked-but-ignored.log": "committed anyway",
	}
	tracked := []string{".gitignore", "src/main.py", "tracked-but-ignored.log"}

	// Both path shapes, because the git-derived patterns are anchored to
	// the transfer root and nest mode shifts every path under the source
	// basename. An unanchored pattern would silently no-op in nest mode.
	for _, tc := range []struct {
		name     string
		contents bool
	}{{"nest", false}, {"contents", true}} {
		t.Run(tc.name, func(t *testing.T) {
			src := setupGitRepo(t, files, tracked)
			dst := t.TempDir()
			opts := &options{contents: tc.contents, excludeVCS: true}

			gf := resolveGitignoreFilter(src, opts)
			if !gf.fromGit {
				t.Fatal("expected the git-backed ignore path for a real repo")
			}
			if err := runRsync(src, dst+"/", nil, buildExcludeList(nil, nil, opts), gf, opts); err != nil {
				t.Fatalf("runRsync: %v", err)
			}

			root := dst
			if !tc.contents {
				root = filepath.Join(dst, "testproj")
			}
			assertPresent(t, root, "git-backed", []string{
				"src/main.py",
				".gitignore",
				"logs/KEEP.log",           // negated re-include
				"tracked-but-ignored.log", // tracked, so never ignored
			})
			assertAbsent(t, root, "git-backed", []string{
				"logs/app.log",
				"build",
			})
		})
	}
}

// TestIntegrationGitignoreSubdirOfRepo covers syncing one package out of a
// larger repo. git reports repo-root-relative paths, which have to be
// rebased onto the source before they mean anything to rsync.
func TestIntegrationGitignoreSubdirOfRepo(t *testing.T) {
	requireRsync(t)
	requireGit(t)
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	repo := setupFakeProject(t, map[string]string{
		".gitignore":              "dist/\n",
		"packages/api/server.go":  "package main\n",
		"packages/api/dist/bin":   "artifact",
		"packages/web/index.html": "<html>\n",
	})
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("add", "-f", "--", ".gitignore", "packages/api/server.go", "packages/web/index.html")
	run("-c", "user.email=test@example.invalid", "-c", "user.name=test", "commit", "-qm", "init")

	src := filepath.Join(repo, "packages", "api")
	dst := t.TempDir()
	opts := &options{excludeVCS: true}
	gf := resolveGitignoreFilter(src, opts)
	if !gf.fromGit {
		t.Fatal("expected the git-backed ignore path")
	}
	if err := runRsync(src, dst+"/", nil, buildExcludeList(nil, nil, opts), gf, opts); err != nil {
		t.Fatalf("runRsync: %v", err)
	}

	nest := filepath.Join(dst, "api")
	assertPresent(t, nest, "subdir-of-repo", []string{"server.go"})
	assertAbsent(t, nest, "subdir-of-repo", []string{"dist"})
}

// TestIntegrationDeleteProtectsDestOnlyIgnored pins the receiver-side
// protection. The git-derived ignore set only knows paths that exist in
// the source right now, so ignored content living only at the destination
// (synced earlier, since deleted locally) has no pattern covering it and
// --delete would wipe it. That contradicts the deliberate choice not to
// pass --delete-excluded.
func TestIntegrationDeleteProtectsDestOnlyIgnored(t *testing.T) {
	requireRsync(t)
	requireGit(t)

	src := setupGitRepo(t, map[string]string{
		".gitignore": "output/\n",
		"keep.py":    "print('hi')\n",
	}, []string{".gitignore", "keep.py"}) // note: no output/ in the source

	dst := t.TempDir()
	// The destination looks like a previous sync: it has the project's
	// .gitignore and ignored content that is now gone locally.
	mustWrite(t, filepath.Join(dst, ".gitignore"), "output/\n")
	mustWrite(t, filepath.Join(dst, "output", "run1.png"), "generated earlier")

	opts := &options{contents: true, deleteExtras: true, excludeVCS: true}
	gf := resolveGitignoreFilter(src, opts)
	if err := runRsync(src, dst+"/", nil, buildExcludeList(nil, nil, opts), gf, opts); err != nil {
		t.Fatalf("runRsync: %v", err)
	}

	assertPresent(t, dst, "delete-protection", []string{"keep.py", "output/run1.png"})
}

// TestGitIgnoreSetFallbackReasons verifies the banner distinguishes a
// routine non-repo source from a genuine git failure. Reporting "no git
// repo" for a directory that plainly is inside one tells the user
// something false about their own project.
func TestGitIgnoreSetFallbackReasons(t *testing.T) {
	requireGit(t)
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	// A source nested inside an ignored directory makes git ls-files abort
	// with "directory entry not superset of prefix".
	repo := setupFakeProject(t, map[string]string{
		".gitignore":          "vendor/\n",
		"a.py":                "x\n",
		"vendor/pkg/lib.go":   "package pkg\n",
		"vendor/pkg/notes.md": "x\n",
	})
	for _, args := range [][]string{
		{"init", "-q"},
		{"add", "-f", "--", ".gitignore", "a.py"},
		{"-c", "user.email=test@example.invalid", "-c", "user.name=test", "commit", "-qm", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	set := loadGitIgnoreSet(filepath.Join(repo, "vendor", "pkg"))
	if set.ok {
		t.Fatal("expected git to fail for a source inside an ignored directory")
	}
	if strings.Contains(set.why, "not a git repo") {
		t.Errorf("must not claim %q for a path that is inside a repo; got %q", "not a git repo", set.why)
	}
	if set.why == "" {
		t.Error("fallback must carry a reason")
	}

	// And the routine case still reads as such.
	plain := loadGitIgnoreSet(setupFakeProject(t, map[string]string{"a.py": "x\n"}))
	if plain.ok || plain.why != "not a git repo" {
		t.Errorf("non-repo source: got ok=%v why=%q", plain.ok, plain.why)
	}
}

// TestGitIgnoreSetSkipsUnrepresentable verifies a newline-bearing ignored
// path is dropped rather than written into the line-based pattern file,
// where its second fragment would become an unanchored rule free to
// exclude unrelated files anywhere in the tree.
func TestGitIgnoreSetSkipsUnrepresentable(t *testing.T) {
	requireGit(t)
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	src := setupFakeProject(t, map[string]string{".gitignore": "*.log\n", "keep.py": "x\n"})
	if err := os.WriteFile(filepath.Join(src, "we\nird.log"), []byte("x"), 0o644); err != nil {
		t.Skipf("filesystem rejects newline in filename: %v", err)
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"add", "-f", "--", ".gitignore", "keep.py"},
		{"-c", "user.email=test@example.invalid", "-c", "user.name=test", "commit", "-qm", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = src
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	set := loadGitIgnoreSet(src)
	if !set.ok {
		t.Fatalf("expected git to answer, got %+v", set)
	}
	if len(set.unrepresentable) != 1 {
		t.Errorf("expected the newline path to be quarantined, got %+v", set)
	}
	for _, p := range set.paths {
		if strings.ContainsAny(p, "\n\r") {
			t.Errorf("newline path leaked into the pattern set: %q", p)
		}
	}
}

// TestGitIgnoreSetFallback verifies a non-repo source degrades to the
// approximate filter instead of erroring or silently dropping the filter.
func TestGitIgnoreSetFallback(t *testing.T) {
	src := setupFakeProject(t, map[string]string{".gitignore": "*.log\n"})
	if set := loadGitIgnoreSet(src); set.ok {
		t.Errorf("expected ok=false for a non-repo source, got %+v", set)
	}
	gf := resolveGitignoreFilter(src, &options{})
	if !gf.enabled || gf.fromGit {
		t.Errorf("expected the approximate filter to stay enabled, got %+v", gf)
	}
	got := gitignoreMode(gf)
	if !strings.Contains(got, "approximate") || !strings.Contains(got, "not a git repo") {
		t.Errorf("banner should disclose the fallback and its reason, got %q", got)
	}
}

func assertPresent(t *testing.T, root, label string, rels []string) {
	t.Helper()
	for _, rel := range rels {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Errorf("%s: expected %s at destination: %v", label, rel, err)
		}
	}
}

func assertAbsent(t *testing.T, root, label string, rels []string) {
	t.Helper()
	for _, rel := range rels {
		if _, err := os.Stat(filepath.Join(root, rel)); err == nil {
			t.Errorf("%s: expected %s to be excluded, but it exists", label, rel)
		} else if !os.IsNotExist(err) {
			t.Errorf("%s: unexpected error checking %s: %v", label, rel, err)
		}
	}
}

// TestRepoConfigRoundTrip verifies save→load produces equivalent config.
func TestRepoConfigRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	src := filepath.Join(t.TempDir(), "myproj")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := saveRepoConfig(src, &repoConfig{}, "nas", []string{"internal/", "models/weights.bin"}); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadRepoConfig(src)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.dest != "nas" {
		t.Errorf("dest=%q, want nas", cfg.dest)
	}
	// internal/ auto-expands to internal/ + internal/***
	// models/weights.bin stays as-is (no trailing slash)
	// -> 3 expanded includes total
	includes := expandIncludePatterns(cfg.rawIncludes)
	if len(includes) != 3 {
		t.Errorf("includes=%v, want 3 entries", includes)
	}
}
