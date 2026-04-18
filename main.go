package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const version = "0.1.0"

type options struct {
	dryRun       bool
	verbose      bool
	deleteExtras bool
	noGitignore  bool
	excludeVCS   bool
	showExcludes bool
	listDests    bool
	showVersion  bool
	keepName     bool
	destName     string
	extraExcl    stringSlice
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
	excludes := buildExcludes(types, opts)
	userExcludes, err := loadUserExcludes()
	if err != nil {
		fail(err)
	}
	excludes = append(excludes, userExcludes...)
	excludes = append(excludes, opts.extraExcl...)
	excludes = dedupe(excludes)

	if opts.showExcludes {
		fmt.Printf("Source:       %s\n", absSource)
		if len(types) > 0 {
			fmt.Printf("Detected:     %s\n", strings.Join(types, ", "))
		} else {
			fmt.Printf("Detected:     (no known project markers)\n")
		}
		fmt.Printf("Gitignore:    %v\n", !opts.noGitignore)
		fmt.Printf("Exclude .git: %v\n", opts.excludeVCS)
		fmt.Printf("Excludes (%d):\n", len(excludes))
		for _, e := range excludes {
			fmt.Println("  " + e)
		}
		return
	}

	destination, err := resolveDestArg(opts, args)
	if err != nil {
		fail(err)
	}

	if err := runRsync(absSource, destination, excludes, opts); err != nil {
		fail(err)
	}
}

func parseFlags() *options {
	opts := &options{}

	flag.BoolVar(&opts.dryRun, "dry-run", false, "show what would be transferred without copying")
	flag.BoolVar(&opts.dryRun, "n", false, "alias for --dry-run")
	flag.BoolVar(&opts.verbose, "verbose", false, "verbose rsync output; also prints the invoked command")
	flag.BoolVar(&opts.verbose, "v", false, "alias for --verbose")
	flag.BoolVar(&opts.deleteExtras, "delete", false, "delete files on destination not present on source")
	flag.BoolVar(&opts.noGitignore, "no-gitignore", false, "don't use .gitignore as an rsync filter")
	flag.BoolVar(&opts.excludeVCS, "no-vcs", false, "exclude .git/.hg/.svn metadata")
	flag.BoolVar(&opts.keepName, "keep-name", false, "don't add trailing slash to source; nest source dir under destination")
	flag.BoolVar(&opts.showExcludes, "show-excludes", false, "print exclude list and exit")
	flag.BoolVar(&opts.listDests, "list-dests", false, "list named destinations and exit")
	flag.BoolVar(&opts.showVersion, "version", false, "print version and exit")
	flag.StringVar(&opts.destName, "dest", "", "named destination from ~/.config/rsync2project/destinations")
	flag.StringVar(&opts.destName, "d", "", "alias for --dest")
	flag.Var(&opts.extraExcl, "extra", "additional exclude pattern (repeatable)")

	flag.Usage = usage
	flag.Parse()
	return opts
}

func resolveDestArg(opts *options, args []string) (string, error) {
	switch {
	case opts.destName != "":
		if len(args) > 1 {
			return "", fmt.Errorf("cannot supply both --dest and a positional destination")
		}
		return resolveDestination(opts.destName)
	case len(args) >= 2:
		return args[1], nil
	default:
		return "", fmt.Errorf("destination required (positional argument or --dest NAME)")
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

Source handling: trailing slash is added automatically so contents are
copied into the destination rather than nested under a new directory.

Options:
  -n, --dry-run         Preview without transferring
  -v, --verbose         Verbose rsync output (also prints invoked command)
      --delete          Delete files on destination not present on source
      --no-gitignore    Don't use .gitignore as an rsync filter
      --no-vcs          Exclude .git/.hg/.svn
      --keep-name       Don't auto-append trailing slash; nest source under dest
      --show-excludes   Print exclude list and exit
      --extra PATTERN   Additional exclude pattern (repeatable)
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
