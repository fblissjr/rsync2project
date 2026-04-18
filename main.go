package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const version = "0.4.0"

type options struct {
	dryRun        bool
	verbose       bool
	deleteExtras  bool
	noGitignore   bool
	excludeVCS    bool
	showExcludes  bool
	listDests     bool
	showVersion   bool
	contents      bool
	saveConfig    bool
	destName      string
	extraExcludes stringSlice
	extraIncludes stringSlice
}

type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ",") }
func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func main() {
	opts := parseFlags()

	if opts.showVersion {
		fmt.Println("rsync2project", version)
		return
	}
	if opts.listDests {
		if err := printDestinations(); err != nil {
			fail(err)
		}
		return
	}

	args := flag.Args()
	if len(args) < 1 {
		flag.Usage()
		os.Exit(2)
	}
	source := args[0]

	absSource, err := filepath.Abs(source)
	if err != nil {
		fail(err)
	}
	info, err := os.Stat(absSource)
	if err != nil {
		fail(fmt.Errorf("source: %w", err))
	}
	if !info.IsDir() {
		fail(fmt.Errorf("source must be a directory: %s", absSource))
	}

	types := detectProjectTypes(absSource)
	excludes := buildExcludes(types, opts.excludeVCS)
	userExcludes, err := loadUserExcludes()
	if err != nil {
		fail(err)
	}
	excludes = append(excludes, userExcludes...)
	excludes = append(excludes, opts.extraExcludes...)
	excludes = dedupe(excludes)

	repoCfg, err := loadRepoConfig(absSource)
	if err != nil {
		fail(err)
	}
	includes := append([]string{}, repoCfg.includes...)
	includes = append(includes, expandIncludePatterns(opts.extraIncludes)...)
	includes = dedupe(includes)

	if opts.saveConfig {
		if err := saveRepoConfig(absSource, repoCfg, opts.destName, opts.extraIncludes); err != nil {
			fail(err)
		}
		fmt.Fprintf(os.Stderr, "saved: %s\n", repoConfigPath(absSource))
	}

	if opts.showExcludes {
		fmt.Printf("Source:       %s\n", absSource)
		if len(types) > 0 {
			fmt.Printf("Detected:     %s\n", joinTypes(types))
		} else {
			fmt.Printf("Detected:     (no known project markers)\n")
		}
		fmt.Printf("Gitignore:    %v\n", !opts.noGitignore)
		fmt.Printf("Exclude .git: %v\n", opts.excludeVCS)
		if len(includes) > 0 {
			fmt.Printf("Includes (%d):\n", len(includes))
			for _, i := range includes {
				fmt.Println("  " + i)
			}
		}
		fmt.Printf("Excludes (%d):\n", len(excludes))
		for _, e := range excludes {
			fmt.Println("  " + e)
		}
		return
	}

	destination, err := resolveDest(opts, args, repoCfg.dest)
	if err != nil {
		fail(err)
	}

	if err := runRsync(absSource, destination, includes, excludes, opts); err != nil {
		fail(err)
	}
}

func parseFlags() *options {
	opts := &options{}

	flag.BoolVar(&opts.dryRun, "dry-run", false, "show what would be transferred without copying")
	flag.BoolVar(&opts.dryRun, "n", false, "")
	flag.BoolVar(&opts.verbose, "verbose", false, "verbose rsync output; also prints the invoked command")
	flag.BoolVar(&opts.verbose, "v", false, "")
	flag.BoolVar(&opts.deleteExtras, "delete", false, "delete files on destination not present on source")
	flag.BoolVar(&opts.noGitignore, "no-gitignore", false, "don't use .gitignore as an rsync filter")
	flag.BoolVar(&opts.excludeVCS, "no-vcs", false, "exclude .git/.hg/.svn metadata")
	flag.BoolVar(&opts.contents, "contents", false, "copy source contents directly into destination instead of nesting under source name")
	flag.BoolVar(&opts.saveConfig, "save-config", false, "write current --dest and --include flags to <source>/.rsync2project for reuse")
	flag.BoolVar(&opts.showExcludes, "show-excludes", false, "print exclude list and exit")
	flag.BoolVar(&opts.listDests, "list-dests", false, "list named destinations and exit")
	flag.BoolVar(&opts.showVersion, "version", false, "print version and exit")
	flag.StringVar(&opts.destName, "dest", "", "named destination from ~/.config/rsync2project/destinations")
	flag.StringVar(&opts.destName, "d", "", "")
	flag.Var(&opts.extraExcludes, "extra", "additional exclude pattern (repeatable)")
	flag.Var(&opts.extraIncludes, "include", "include pattern that overrides .gitignore/excludes (repeatable)")

	flag.Usage = usage
	flag.Parse()
	return opts
}

// resolveDest picks a destination using priority: explicit --dest, explicit
// positional, then the default from the repo config (if any). If --dest is
// empty and there's no positional, a repo-configured dest acts as the
// "remembered" target so repeat syncs become a one-arg command.
func resolveDest(opts *options, args []string, repoDefault string) (string, error) {
	name := opts.destName
	if name == "" && len(args) < 2 && repoDefault != "" {
		name = repoDefault
	}
	switch {
	case name != "":
		if len(args) > 1 {
			return "", fmt.Errorf("cannot supply both --dest/repo-config dest and a positional destination")
		}
		return resolveDestination(name)
	case len(args) >= 2:
		return args[1], nil
	default:
		return "", fmt.Errorf("destination required (positional, --dest NAME, or 'dest = NAME' in repo config)")
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `rsync2project - smart project sync over rsync

Usage:
  rsync2project [options] <source> <destination>
  rsync2project [options] --dest NAME <source>

Copies <source> to <destination>, excluding common build artifacts and
dependency directories (node_modules, __pycache__, .venv, target, .gradle,
etc.) and honoring each project's .gitignore.

Source handling: by default the source directory is preserved at the
destination (rsync's native behavior). For example,
  rsync2project ~/code/myapp /backup/
creates /backup/myapp/. Pass --contents to spill the source's files
directly into the destination without the intermediate directory.

Options:
  -n, --dry-run         Preview without transferring
  -v, --verbose         Verbose rsync output (also prints invoked command)
      --delete          Delete files on destination not present on source
      --no-gitignore    Don't use .gitignore as an rsync filter
      --no-vcs          Exclude .git/.hg/.svn
      --contents        Copy source contents into dest (don't nest by source name)
      --show-excludes   Print exclude list and exit
      --extra PATTERN   Additional exclude pattern (repeatable)
      --include PATTERN Re-include a path that .gitignore/excludes would drop (repeatable)
      --save-config     Persist current --dest and --include flags to <source>/.rsync2project
  -d, --dest NAME       Named destination from config file
      --list-dests      List configured destinations and exit
      --version         Print version
  -h, --help            Show this help

Examples:
  rsync2project ~/code/myapp user@host:/path/
  rsync2project --dest name ~/code/myapp
  rsync2project -n --show-excludes ~/code/myapp
`)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "rsync2project:", err)
	os.Exit(1)
}

func joinTypes(types []projectType) string {
	s := make([]string, len(types))
	for i, t := range types {
		s[i] = string(t)
	}
	return strings.Join(s, ", ")
}

func dedupe(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
