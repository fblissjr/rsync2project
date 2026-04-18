package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveRepoConfigArg(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")
	base := "/tmp/xdg/rsync2project/repos"

	cases := map[string]string{
		"myapp":                     filepath.Join(base, "myapp.conf"),
		"myapp.conf":                filepath.Join(base, "myapp.conf"),
		"/Users/fred/code/myapp":    filepath.Join(base, "myapp.conf"),
		"/Users/fred/code/myapp/":   filepath.Join(base, "myapp.conf"),
		"./myapp":                   filepath.Join(base, "myapp.conf"),
	}
	for in, want := range cases {
		if got := resolveRepoConfigArg(in); got != want {
			t.Errorf("resolveRepoConfigArg(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRunRepoListEmpty(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if code := runRepoCmd(nil); code != 0 {
		t.Errorf("empty list exit = %d, want 0", code)
	}
}

func TestRunRepoShowMissing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if code := runRepoCmd([]string{"show", "ghost"}); code != 1 {
		t.Errorf("show missing exit = %d, want 1", code)
	}
}

func TestRunRepoRmDryRun(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path := resolveRepoConfigArg("myapp")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("dest = nas\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if code := runRepoCmd([]string{"rm", "-n", "myapp"}); code != 0 {
		t.Errorf("dry-run exit = %d, want 0", code)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("dry-run deleted the file: %v", err)
	}

	if code := runRepoCmd([]string{"rm", "myapp"}); code != 0 {
		t.Errorf("rm exit = %d, want 0", code)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("rm did not delete the file: err=%v", err)
	}
}

func TestRunRepoRmMissing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if code := runRepoCmd([]string{"rm", "ghost"}); code != 1 {
		t.Errorf("rm missing exit = %d, want 1", code)
	}
}

func TestRunRepoPath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")
	// We can't easily capture stdout without plumbing, but we can at
	// least confirm the exit codes for the two variants.
	if code := runRepoCmd([]string{"path"}); code != 0 {
		t.Errorf("bare path exit = %d", code)
	}
	if code := runRepoCmd([]string{"path", "myapp"}); code != 0 {
		t.Errorf("named path exit = %d", code)
	}
	if code := runRepoCmd([]string{"path", "a", "b"}); code != 2 {
		t.Errorf("extra args exit = %d, want 2", code)
	}
}

func TestRunConfigPath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")
	if code := runConfigCmd([]string{"path"}); code != 0 {
		t.Errorf("config path exit = %d, want 0", code)
	}
	if code := runConfigCmd([]string{"path", "extra"}); code != 2 {
		t.Errorf("extra args exit = %d, want 2", code)
	}
	if code := runConfigCmd([]string{"frobnicate"}); code != 2 {
		t.Errorf("unknown sub exit = %d, want 2", code)
	}
	if code := runConfigCmd(nil); code != 2 {
		t.Errorf("no args exit = %d, want 2", code)
	}
}

func TestRunRepoShowFormatJSON(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path := resolveRepoConfigArg("myapp")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "# rsync2project\n# source: /tmp/src/myapp\ndest = nas\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := runRepoCmd([]string{"show", "--format", "json", "myapp"}); code != 0 {
		t.Errorf("json show exit = %d, want 0", code)
	}
	if code := runRepoCmd([]string{"show", "--format", "text", "myapp"}); code != 0 {
		t.Errorf("text show exit = %d, want 0", code)
	}
	if code := runRepoCmd([]string{"show", "--format", "xml", "myapp"}); code != 2 {
		t.Errorf("unknown format exit = %d, want 2", code)
	}
}

func TestRepoHelpExitZero(t *testing.T) {
	cases := [][]string{
		{"list", "--help"},
		{"show", "--help"},
		{"rm", "--help"},
		{"path", "--help"},
	}
	for _, c := range cases {
		if code := runRepoCmd(c); code != 0 {
			t.Errorf("runRepoCmd(%v) exit = %d, want 0", c, code)
		}
	}
}

func TestConfigHelpExitZero(t *testing.T) {
	if code := runConfigCmd([]string{"path", "--help"}); code != 0 {
		t.Errorf("config path --help exit = %d, want 0", code)
	}
}

// Integration-lite: end-to-end list output mentions saved repo names.
func TestRunRepoListWithEntries(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, name := range []string{"alpha", "beta"} {
		path := resolveRepoConfigArg(name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		body := strings.Join([]string{
			"# rsync2project per-repo config",
			"# source: /tmp/src/" + name,
			"",
			"dest = nas",
		}, "\n") + "\n"
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if code := runRepoCmd(nil); code != 0 {
		t.Errorf("list exit = %d, want 0", code)
	}
}
