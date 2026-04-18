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

// parseKVFile reads a simple name=value config file. Lines starting with '#'
// are comments; blank lines are skipped. Returns an empty map if the file
// does not exist.
func parseKVFile(path string) (map[string]string, error) {
	out := make(map[string]string)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
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
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("%s:%d: expected name=value, got %q", path, lineNo, line)
		}
		out[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// parseLineFile reads a file with one non-comment, non-blank entry per line.
func parseLineFile(path string) ([]string, error) {
	var out []string
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out, sc.Err()
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
