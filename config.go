package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func configDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "rsync2project")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "rsync2project")
}

func destinationsPath() string { return filepath.Join(configDir(), "destinations") }
func excludesPath() string     { return filepath.Join(configDir(), "excludes") }

// forEachConfigLine invokes fn for each non-blank, non-comment line of path
// after trimming surrounding whitespace. Missing files are treated as empty
// so both config files are optional for callers.
func forEachConfigLine(path string, fn func(lineNo int, line string) error) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if err := fn(lineNo, line); err != nil {
			return err
		}
	}
	return sc.Err()
}

func parseKVFile(path string) (map[string]string, error) {
	out := make(map[string]string)
	err := forEachConfigLine(path, func(lineNo int, line string) error {
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("%s:%d: expected name=value, got %q", path, lineNo, line)
		}
		out[strings.TrimSpace(k)] = strings.TrimSpace(v)
		return nil
	})
	return out, err
}

func parseLineFile(path string) ([]string, error) {
	var out []string
	err := forEachConfigLine(path, func(_ int, line string) error {
		out = append(out, line)
		return nil
	})
	return out, err
}

func resolveDestination(name string) (string, error) {
	dests, err := parseKVFile(destinationsPath())
	if err != nil {
		return "", err
	}
	v, ok := dests[name]
	if !ok {
		return "", fmt.Errorf("unknown destination %q (configure in %s)", name, destinationsPath())
	}
	return v, nil
}

func printDestinations() error {
	path := destinationsPath()
	dests, err := parseKVFile(path)
	if err != nil {
		return err
	}
	if len(dests) == 0 {
		fmt.Printf("No destinations configured. Create %s with lines like:\n", path)
		fmt.Println("  name=user@host:/path/")
		return nil
	}
	names := make([]string, 0, len(dests))
	for k := range dests {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Printf("  %-16s %s\n", n, dests[n])
	}
	return nil
}

func loadUserExcludes() ([]string, error) {
	return parseLineFile(excludesPath())
}

// reposDir is where per-repo configs live. Each repo config is a single
// file named <basename>.conf. Using a central directory instead of a file
// inside the source tree keeps the repo itself clean (no gitignore edit, no
// accidental commits of personal backup settings) and keeps all
// rsync2project state under ~/.config/rsync2project.
func reposDir() string { return filepath.Join(configDir(), "repos") }

// repoConfigPath returns the central-config path for the given source.
// Collision: two repos with the same basename share one config. That matches
// "configure once per repo name" intuition; users with genuine duplicates
// can edit the file manually to disambiguate.
func repoConfigPath(source string) string {
	return filepath.Join(reposDir(), filepath.Base(source)+".conf")
}

// repoConfig holds the per-source configuration. Fields are zero-valued
// when absent; a missing file is not an error. rawIncludes holds the
// user-entered include patterns; consumers call expandIncludePatterns to
// get the rsync-ready form.
type repoConfig struct {
	dest        string
	rawIncludes []string
}

// loadRepoConfig reads <source>/.rsync2project. Format:
//
//	# comments and blank lines are ignored
//	dest = name              (optional directive, same semantics as --dest)
//	internal/                (include pattern, same semantics as --include)
//	models/
//	notes/drafts/
//
// Directive lines match "KEY = VALUE" (spaces optional). All other non-empty,
// non-comment lines are treated as rsync include patterns.
func loadRepoConfig(source string) (*repoConfig, error) {
	cfg := &repoConfig{}
	path := repoConfigPath(source)
	err := forEachConfigLine(path, func(lineNo int, line string) error {
		if k, v, ok := strings.Cut(line, "="); ok {
			k = strings.TrimSpace(k)
			v = strings.TrimSpace(v)
			switch k {
			case "dest":
				cfg.dest = v
			default:
				return fmt.Errorf("%s:%d: unknown directive %q (supported: dest)", path, lineNo, k)
			}
			return nil
		}
		cfg.rawIncludes = append(cfg.rawIncludes, line)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// saveRepoConfig writes the repo's config file. Merges existing patterns
// with newly supplied ones (raw, pre-expansion) and uses newDest if
// non-empty, else preserves existing.dest. Creates the repos directory if
// needed. The file's header records the absolute source path so future
// humans can disambiguate same-basename collisions.
func saveRepoConfig(source string, existing *repoConfig, newDest string, newIncludes []string) error {
	dest := existing.dest
	if newDest != "" {
		dest = newDest
	}

	merged := append([]string{}, existing.rawIncludes...)
	for _, p := range newIncludes {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		merged = append(merged, p)
	}
	merged = dedupe(merged)

	path := repoConfigPath(source)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintln(f, "# rsync2project per-repo config")
	fmt.Fprintf(f, "# source: %s\n", source)
	fmt.Fprintf(f, "# saved:  %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintln(f, "#")
	fmt.Fprintln(f, "# 'dest = NAME' pins a default destination (matches --dest).")
	fmt.Fprintln(f, "# Other non-comment lines are rsync include patterns that")
	fmt.Fprintln(f, "# override .gitignore; trailing '/' auto-expands to include contents.")
	fmt.Fprintln(f)
	if dest != "" {
		fmt.Fprintf(f, "dest = %s\n", dest)
		fmt.Fprintln(f)
	}
	for _, p := range merged {
		fmt.Fprintln(f, p)
	}
	return nil
}

// expandIncludePatterns strips an optional leading '+ ' (rsync's native
// include marker) and auto-expands directory-style patterns (trailing '/')
// into both "X/" and "X/***" so the directory AND its contents survive any
// later exclude or gitignore filter. Trims whitespace; blank lines drop out.
func expandIncludePatterns(patterns []string) []string {
	out := make([]string, 0, len(patterns)*2)
	for _, p := range patterns {
		if rest, ok := strings.CutPrefix(p, "+ "); ok {
			p = rest
		}
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.HasSuffix(p, "/") {
			out = append(out, p, p+"***")
		} else {
			out = append(out, p)
		}
	}
	return out
}
