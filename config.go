package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
