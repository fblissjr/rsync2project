package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpsertDestinationAddsNew(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	added, err := upsertDestination("mac", "fred@mac.local:/Users/fred/backup/")
	if err != nil {
		t.Fatal(err)
	}
	if !added {
		t.Error("expected added=true for new entry")
	}

	dests, err := parseKVFile(destinationsPath())
	if err != nil {
		t.Fatal(err)
	}
	if got, want := dests["mac"], "fred@mac.local:/Users/fred/backup/"; got != want {
		t.Errorf("mac=%q want %q", got, want)
	}
}

func TestUpsertDestinationUpdatesExistingAndPreservesComments(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path := destinationsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	initial := "# my named destinations\n\nnas=old@host:/old/\nmac=old@mac:/old/\n# trailing comment\n"
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	added, err := upsertDestination("mac", "fred@mac.local:/new/")
	if err != nil {
		t.Fatal(err)
	}
	if added {
		t.Error("expected added=false when updating existing entry")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(got)
	if !strings.Contains(content, "# my named destinations") {
		t.Errorf("leading comment dropped: %q", content)
	}
	if !strings.Contains(content, "# trailing comment") {
		t.Errorf("trailing comment dropped: %q", content)
	}
	if !strings.Contains(content, "mac=fred@mac.local:/new/") {
		t.Errorf("mac not updated in place: %q", content)
	}
	if strings.Contains(content, "old@mac:/old/") {
		t.Errorf("old value still present: %q", content)
	}
	// nas entry should be untouched.
	if !strings.Contains(content, "nas=old@host:/old/") {
		t.Errorf("unrelated entry lost: %q", content)
	}
}

func TestRemoveDestination(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path := destinationsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	initial := "# top\nnas=user@host:/path/\nmac=fred@mac:/path/\n# bottom\n"
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := removeDestination("mac"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(got)
	if strings.Contains(content, "mac=") {
		t.Errorf("mac entry not removed: %q", content)
	}
	if !strings.Contains(content, "nas=user@host:/path/") {
		t.Errorf("unrelated entry lost: %q", content)
	}
	if !strings.Contains(content, "# top") || !strings.Contains(content, "# bottom") {
		t.Errorf("comments lost: %q", content)
	}
}

func TestRemoveDestinationMissing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	err := removeDestination("ghost")
	if err == nil || !strings.Contains(err.Error(), "no destination") {
		t.Errorf("expected not-found error, got %v", err)
	}
}

func TestValidateDestName(t *testing.T) {
	cases := []struct {
		in      string
		wantErr bool
	}{
		{"mac", false},
		{"my-laptop", false},
		{"", true},
		{"with space", true},
		{"has=sign", true},
	}
	for _, c := range cases {
		err := validateDestName(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("validateDestName(%q) err=%v wantErr=%v", c.in, err, c.wantErr)
		}
	}
}

func TestRunDestAddDryRun(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	code := runDestCmd([]string{"add", "-n", "mac", "fred@mac:/path/"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if _, err := os.Stat(destinationsPath()); !os.IsNotExist(err) {
		t.Errorf("dry-run wrote file: err=%v", err)
	}
}

func TestRunDestAddBadArgs(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if code := runDestCmd([]string{"add", "only-one-arg"}); code != 2 {
		t.Errorf("expected exit 2 for missing value, got %d", code)
	}
}

func TestRunDestUnknownSubcommand(t *testing.T) {
	if code := runDestCmd([]string{"frobnicate"}); code != 2 {
		t.Errorf("expected exit 2 for unknown subcommand, got %d", code)
	}
}

func TestRunDestShow(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if _, err := upsertDestination("mac", "fred@mac:/path/"); err != nil {
		t.Fatal(err)
	}
	if code := runDestCmd([]string{"show", "mac"}); code != 0 {
		t.Errorf("show existing exit = %d, want 0", code)
	}
	if code := runDestCmd([]string{"show", "ghost"}); code != 1 {
		t.Errorf("show missing exit = %d, want 1", code)
	}
	if code := runDestCmd([]string{"show"}); code != 2 {
		t.Errorf("show no-arg exit = %d, want 2", code)
	}
}

func TestHelpExitZero(t *testing.T) {
	// --help through any subcommand flagset should exit 0, not 2.
	cases := [][]string{
		{"add", "--help"},
		{"rm", "--help"},
		{"list", "--help"},
		{"show", "--help"},
	}
	for _, c := range cases {
		if code := runDestCmd(c); code != 0 {
			t.Errorf("runDestCmd(%v) exit = %d, want 0", c, code)
		}
	}
}
