package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestDetectProjectTypes(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "pyproject.toml"), "[project]\nname='x'\n")
	mustWrite(t, filepath.Join(dir, "package.json"), "{}\n")
	// These should be skipped and not cause the walk to fail.
	mustMkdir(t, filepath.Join(dir, "node_modules", "left-pad"))
	mustWrite(t, filepath.Join(dir, "node_modules", "left-pad", "package.json"), "{}")
	// Nested project marker at depth 1 should still register.
	mustMkdir(t, filepath.Join(dir, "services", "api"))
	mustWrite(t, filepath.Join(dir, "services", "api", "go.mod"), "module x\n")

	types := detectProjectTypes(dir)
	for _, want := range []projectType{ptPython, ptNode, ptGo} {
		if !slices.Contains(types, want) {
			t.Errorf("expected %q in %v", want, types)
		}
	}
}

func TestBuildExcludesVCS(t *testing.T) {
	if ex := buildExcludes(nil, true); !slices.Contains(ex, ".git/") {
		t.Error("expected .git/ when excludeVCS is true")
	}
	if ex := buildExcludes(nil, false); slices.Contains(ex, ".git/") {
		t.Error("did not expect .git/ when excludeVCS is false")
	}
}

func TestBuildExcludesDotnet(t *testing.T) {
	ex := buildExcludes([]projectType{ptDotnet}, false)
	if !slices.Contains(ex, "bin/") || !slices.Contains(ex, "obj/") {
		t.Errorf("expected bin/ and obj/ for dotnet; got %v", ex)
	}
}

func TestLooksRemote(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"/local/path", false},
		{"./relative", false},
		{"relative", false},
		{"user@host:/path", true},
		{"host:/path", true},
		{"host:relpath", true},
		{"rsync://host/mod", true},
		{"/path/with:colon", false},
	}
	for _, c := range cases {
		if got := looksRemote(c.in); got != c.want {
			t.Errorf("looksRemote(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestDestBase(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"/srv/backup/myapp", "myapp"},
		{"/srv/backup/myapp/", "myapp"},
		{"/srv/backup/myapp///", "myapp"},
		{"user@host:/srv/backup/myapp", "myapp"},
		{"host:myapp", "myapp"},
		{"host:", ""},
		{"rsync://host/mod/myapp", "myapp"},
		{"/", ""},
		{"myapp", "myapp"},
	}
	for _, c := range cases {
		if got := destBase(c.in); got != c.want {
			t.Errorf("destBase(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestEffectiveDest(t *testing.T) {
	cases := []struct {
		source, dest string
		contents     bool
		want         string
	}{
		// Default nests the source dir under the destination.
		{"/src/myapp", "/srv/backup", false, "/srv/backup/myapp"},
		{"/src/myapp", "/srv/backup/", false, "/srv/backup/myapp"},
		{"/src/myapp", "user@host:/srv/backup/", false, "user@host:/srv/backup/myapp"},
		// The doubling case: dest already names the project.
		{"/src/myapp", "user@host:/srv/myapp", false, "user@host:/srv/myapp/myapp"},
		// --contents spills into the destination as given.
		{"/src/myapp", "/srv/backup/", true, "/srv/backup/"},
		// A bare "host:" means the remote home; appending a slash would
		// wrongly make the target absolute.
		{"/src/myapp", "host:", false, "host:myapp"},
	}
	for _, c := range cases {
		if got := effectiveDest(c.source, c.dest, c.contents); got != c.want {
			t.Errorf("effectiveDest(%q, %q, %v) = %q, want %q",
				c.source, c.dest, c.contents, got, c.want)
		}
	}
}

func TestNestCollides(t *testing.T) {
	cases := []struct {
		source, dest string
		contents     bool
		want         bool
	}{
		{"/src/myapp", "user@host:/srv/myapp", false, true},
		{"/src/myapp", "user@host:/srv/myapp/", false, true},
		// --contents is the fix, so it must never warn.
		{"/src/myapp", "user@host:/srv/myapp", true, false},
		// Ordinary "back up into a parent dir" must stay quiet.
		{"/src/myapp", "user@host:/srv/backup/", false, false},
		{"/src/myapp", "/srv/backup", false, false},
		// Near-miss names are not collisions.
		{"/src/myapp", "/srv/myapp2", false, false},
	}
	for _, c := range cases {
		if got := nestCollides(c.source, c.dest, c.contents); got != c.want {
			t.Errorf("nestCollides(%q, %q, %v) = %v, want %v",
				c.source, c.dest, c.contents, got, c.want)
		}
	}
}

func TestBuildExcludeListNoExcludes(t *testing.T) {
	user := []string{"secrets/"}

	full := buildExcludeList([]projectType{ptPython}, user, &options{extraExcludes: stringSlice{"scratch/"}})
	for _, want := range []string{"node_modules/", "__pycache__/", "build/", "secrets/", "scratch/"} {
		if !slices.Contains(full, want) {
			t.Errorf("default run: expected %q in %v", want, full)
		}
	}

	// --no-excludes drops the curated tables and the user's global file...
	bare := buildExcludeList([]projectType{ptPython}, user, &options{
		noExcludes:    true,
		extraExcludes: stringSlice{"scratch/"},
	})
	for _, unwanted := range []string{"node_modules/", "__pycache__/", "build/", "secrets/"} {
		if slices.Contains(bare, unwanted) {
			t.Errorf("--no-excludes: did not expect %q in %v", unwanted, bare)
		}
	}
	// ...but must not undo --extra, which is an explicit same-run request.
	if !slices.Contains(bare, "scratch/") {
		t.Errorf("--no-excludes: expected --extra pattern to survive; got %v", bare)
	}

	// Same for --no-vcs: asked for on this command line, so it stands.
	vcs := buildExcludeList(nil, nil, &options{noExcludes: true, excludeVCS: true})
	if !slices.Contains(vcs, ".git/") {
		t.Errorf("--no-excludes --no-vcs: expected .git/ to survive; got %v", vcs)
	}
	if noVCS := buildExcludeList(nil, nil, &options{noExcludes: true}); len(noVCS) != 0 {
		t.Errorf("--no-excludes alone should yield an empty list; got %v", noVCS)
	}
}

func TestEscapeRsyncPattern(t *testing.T) {
	cases := []struct{ in, want string }{
		{"models/weights.bin", "models/weights.bin"},
		// Without escaping, "[1]" is a character class: the pattern would
		// miss this file and exclude "w1.txt" instead. Escaping the opening
		// bracket is sufficient — with no class open, "]" is already
		// literal (verified against rsync 3.4.4).
		{"weird[1].txt", `weird\[1].txt`},
		{"star*.log", `star\*.log`},
		{"what?.txt", `what\?.txt`},
		// No wildcard in the pattern means rsync compares literally and
		// never honors escapes, so escaping the backslash here would make
		// the pattern stop matching its own file.
		{`back\slash`, `back\slash`},
		// With a wildcard present the pattern IS parsed, so now every
		// backslash has to be escaped too.
		{`back\slash*.txt`, `back\\slash\*.txt`},
	}
	for _, c := range cases {
		if got := escapeRsyncPattern(c.in); got != c.want {
			t.Errorf("escapeRsyncPattern(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestGitIgnoreSetRsyncExcludes(t *testing.T) {
	set := gitIgnoreSet{ok: true, paths: []string{"build/", "logs/app.log"}}

	// Nest mode: the transfer root is one level above the source, so every
	// pattern needs the source basename or it silently matches nothing.
	nest := set.rsyncExcludes("/src/myapp", false)
	want := []string{"/myapp/build/", "/myapp/logs/app.log"}
	if !slices.Equal(nest, want) {
		t.Errorf("nest excludes = %v, want %v", nest, want)
	}

	// --contents: the source's contents are the root.
	contents := set.rsyncExcludes("/src/myapp", true)
	want = []string{"/build/", "/logs/app.log"}
	if !slices.Equal(contents, want) {
		t.Errorf("contents excludes = %v, want %v", contents, want)
	}

	// A source basename containing a metacharacter must not become a glob.
	odd := gitIgnoreSet{ok: true, paths: []string{"x"}}.rsyncExcludes("/src/app[1]", false)
	if !slices.Equal(odd, []string{`/app\[1]/x`}) {
		t.Errorf("metachar basename not escaped: %v", odd)
	}
}

func TestDedupe(t *testing.T) {
	got := dedupe([]string{"a", "b", "a", "c", "b"})
	want := []string{"a", "b", "c"}
	if !slices.Equal(got, want) {
		t.Errorf("dedupe = %v, want %v", got, want)
	}
}

func TestParseKVFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "destinations")
	mustWrite(t, path, `# comment
nas=user@host:/path/
ubuntu = me@ubuntu:~/code/

php=nope
`)
	m, err := parseKVFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if m["nas"] != "user@host:/path/" {
		t.Errorf("nas=%q", m["nas"])
	}
	if m["ubuntu"] != "me@ubuntu:~/code/" {
		t.Errorf("ubuntu=%q", m["ubuntu"])
	}
	if m["php"] != "nope" {
		t.Errorf("php=%q", m["php"])
	}
}

func TestParseKVFileMissing(t *testing.T) {
	m, err := parseKVFile(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 0 {
		t.Errorf("expected empty map for missing file, got %v", m)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}
