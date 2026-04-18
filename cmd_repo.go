package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func runRepoCmd(args []string) int {
	if len(args) == 0 {
		return runRepoList(nil)
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "list", "ls":
		return runRepoList(rest)
	case "show", "cat":
		return runRepoShow(rest)
	case "rm", "remove", "delete":
		return runRepoRm(rest)
	case "path":
		return runRepoPath(rest)
	case "-h", "--help", "help":
		repoUsage(os.Stdout)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "rsync2project repo: unknown subcommand %q\n\n", sub)
		repoUsage(os.Stderr)
		return 2
	}
}

func runRepoList(args []string) int {
	fs := flag.NewFlagSet("repo list", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if stop, code := parseSubFlags(fs, args); stop {
		return code
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "rsync2project repo list: takes no arguments")
		return 2
	}

	dir := reposDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("No repo configs in %s\n", dir)
			return 0
		}
		return failMsg(err)
	}

	type row struct{ name, source string }
	var rows []row
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".conf") {
			continue
		}
		full := filepath.Join(dir, e.Name())
		rows = append(rows, row{
			name:   strings.TrimSuffix(e.Name(), ".conf"),
			source: readSourceHeader(full),
		})
	}
	if len(rows) == 0 {
		fmt.Printf("No repo configs in %s\n", dir)
		return 0
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })
	for _, r := range rows {
		if r.source != "" {
			fmt.Printf("  %-20s %s\n", r.name, r.source)
		} else {
			fmt.Printf("  %s\n", r.name)
		}
	}
	return 0
}

func runRepoShow(args []string) int {
	fs := flag.NewFlagSet("repo show", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	format := "text"
	fs.StringVar(&format, "format", "text", "output format: text (default) or json")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: rsync2project repo show [--format text|json] NAME|PATH")
	}
	if stop, code := parseSubFlags(fs, args); stop {
		return code
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return 2
	}
	path := resolveRepoConfigArg(fs.Arg(0))
	data, err := os.ReadFile(path)
	if err != nil {
		return failMsg(err)
	}

	switch format {
	case "text":
		fmt.Printf("# %s\n%s", path, data)
		if len(data) > 0 && data[len(data)-1] != '\n' {
			fmt.Println()
		}
	case "json":
		out := struct {
			Path    string `json:"path"`
			Source  string `json:"source"`
			Content string `json:"content"`
		}{
			Path:    path,
			Source:  readSourceHeader(path),
			Content: string(data),
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			return failMsg(err)
		}
	default:
		fmt.Fprintf(os.Stderr, "rsync2project repo show: unknown --format %q (supported: text, json)\n", format)
		return 2
	}
	return 0
}

func runRepoRm(args []string) int {
	fs := flag.NewFlagSet("repo rm", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var dryRun bool
	addDryRunFlag(fs, &dryRun)
	fs.Usage = func() { fmt.Fprintln(os.Stderr, "Usage: rsync2project repo rm [-n] NAME|PATH") }
	if stop, code := parseSubFlags(fs, args); stop {
		return code
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return 2
	}
	path := resolveRepoConfigArg(fs.Arg(0))

	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "rsync2project: no repo config at %s\n", path)
			return 1
		}
		return failMsg(err)
	}
	if dryRun {
		fmt.Fprintf(os.Stderr, "dry-run: would remove %s\n", path)
		return 0
	}
	if err := os.Remove(path); err != nil {
		return failMsg(err)
	}
	fmt.Fprintf(os.Stderr, "removed: %s\n", path)
	return 0
}

func runRepoPath(args []string) int {
	fs := flag.NewFlagSet("repo path", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if stop, code := parseSubFlags(fs, args); stop {
		return code
	}
	switch fs.NArg() {
	case 0:
		fmt.Println(reposDir())
	case 1:
		fmt.Println(resolveRepoConfigArg(fs.Arg(0)))
	default:
		fmt.Fprintln(os.Stderr, "Usage: rsync2project repo path [NAME|PATH]")
		return 2
	}
	return 0
}

// resolveRepoConfigArg accepts either a source directory path or a bare
// repo name (with or without a .conf suffix) and returns the canonical
// config file path.
func resolveRepoConfigArg(arg string) string {
	name := strings.TrimSuffix(filepath.Base(arg), ".conf")
	return filepath.Join(reposDir(), name+".conf")
}

func repoUsage(w io.Writer) {
	fmt.Fprint(w, `Usage: rsync2project repo <subcommand>

Manage per-repo configs in ~/.config/rsync2project/repos/.
(Per-repo configs are written by 'rsync2project --save-config' during a
normal sync; 'repo' inspects and cleans them up.)

Subcommands:
  list                  List configured repos with their source paths
                        (default if no subcommand)
  show NAME|PATH        Print the config file for a repo
  rm [-n] NAME|PATH     Remove a repo config file
  path [NAME|PATH]      Print the config path (dir if no arg)

NAME is the basename of the source directory (e.g. 'myapp' for
~/code/myapp). PATH is the source directory itself — both resolve to
the same config file.

Examples:
  rsync2project repo
  rsync2project repo show myapp
  rsync2project repo rm ~/code/oldproject
  rsync2project repo path
`)
}
