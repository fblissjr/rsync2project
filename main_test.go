package main

import (
	"os"
	"path/filepath"
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
	for _, want := range []string{"python", "node", "go"} {
		if !contains(types, want) {
			t.Errorf("expected %q in %v", want, types)
		}
	}
}

func TestBuildExcludesVCS(t *testing.T) {
	if ex := buildExcludes(nil, &options{excludeVCS: true}); !contains(ex, ".git/") {
		t.Error("expected .git/ when excludeVCS is true")
	}
	ex := buildExcludes(nil, &options{excludeVCS: false})
	if contains(ex, ".git/") {
		t.Error("did not expect .git/ when excludeVCS is false")
	}
}

func TestBuildExcludesDotnet(t *testing.T) {
	ex := buildExcludes([]string{"dotnet"}, &options{})
	if !contains(ex, "bin/") || !contains(ex, "obj/") {
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

func TestDedupe(t *testing.T) {
	got := dedupe([]string{"a", "b", "a", "c", "b"})
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("dedupe len=%d, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("dedupe[%d]=%q, want %q", i, got[i], want[i])
		}
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

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
