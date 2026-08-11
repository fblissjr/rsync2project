package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const version = "0.7.0"

type options struct {
	dryRun        bool
	verbose       bool
	quiet         bool
	deleteExtras  bool
	noGitignore   bool
	noExcludes    bool
	all           bool
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
	// Subcommand dispatch happens before flag parsing so the subcommand
	// owns its own flags (e.g., `dest add -n NAME VALUE` should dry-run
	// the write, not be rejected as an unknown top-level flag).
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "dest":
			os.Exit(runDestCmd(os.Args[2:]))
		case "repo":
			os.Exit(runRepoCmd(os.Args[2:]))
		case "config":
			os.Exit(runConfigCmd(os.Args[2:]))
		}
	}

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
	userExcludes, err := loadUserExcludes()
	if err != nil {
		fail(err)
	}
	excludes := buildExcludeList(types, userExcludes, opts)

	gf := resolveGitignoreFilter(absSource, opts)

	repoCfg, err := loadRepoConfig(absSource)
	if err != nil {
		fail(err)
	}
	raw := append([]string{}, repoCfg.rawIncludes...)
	raw = append(raw, opts.extraIncludes...)
	includes := dedupe(expandIncludePatterns(raw))

	if opts.saveConfig {
		// Validate the destination name before persisting it, so a typo
		// doesn't silently break every future run against this repo.
		if opts.destName != "" {
			if _, err := resolveDestination(opts.destName); err != nil {
				fail(fmt.Errorf("refusing to save config: %w", err))
			}
		}
		// Honor the universal "-n means zero side effects" convention:
		// under dry-run, report what would be saved but don't write.
		if opts.dryRun {
			fmt.Fprintf(os.Stderr, "dry-run: would save to %s\n", repoConfigPath(absSource))
		} else {
			if err := saveRepoConfig(absSource, repoCfg, opts.destName, opts.extraIncludes); err != nil {
				fail(err)
			}
			fmt.Fprintf(os.Stderr, "saved: %s\n", repoConfigPath(absSource))
		}
	}

	if opts.showExcludes {
		fmt.Printf("Source:       %s\n", absSource)
		if len(types) > 0 {
			fmt.Printf("Detected:     %s\n", joinTypes(types))
		} else {
			fmt.Printf("Detected:     (no known project markers)\n")
		}
		fmt.Printf("Gitignore:    %s\n", gitignoreMode(gf))
		fmt.Printf("Builtin excl: %s\n", onOff(!opts.noExcludes))
		fmt.Printf("Exclude .git: %s\n", onOff(opts.excludeVCS))
		// The git-derived set is the answer to "why didn't this file
		// copy?", so print it rather than making the user re-derive it.
		if gf.enabled && gf.fromGit {
			fmt.Printf("Gitignored (%d):\n", len(gf.excludes))
			for _, p := range gf.excludes {
				fmt.Println("  " + p)
			}
		}
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

	if !opts.quiet {
		printRunBanner(absSource, destination, includes, excludes, gf, opts)
	}
	warnNestCollision(absSource, destination, opts)

	if err := runRsync(absSource, destination, includes, excludes, gf, opts); err != nil {
		fail(err)
	}
}

// resolveGitignoreFilter decides how .gitignore gets honored for this run
// and reports the fallback, since "git couldn't answer" changes which files
// transfer and the user has no other way to notice.
func resolveGitignoreFilter(source string, opts *options) gitignoreFilter {
	if opts.noGitignore {
		return gitignoreFilter{}
	}
	set := loadGitIgnoreSet(source)
	if !set.ok {
		return gitignoreFilter{enabled: true, fallbackReason: set.why}
	}
	if len(set.unrepresentable) > 0 {
		fmt.Fprintf(os.Stderr,
			"rsync2project: note: %d ignored path(s) contain a newline and cannot be\n"+
				"  expressed as an rsync pattern; they will be copied instead of skipped.\n"+
				"  First: %q\n",
			len(set.unrepresentable), set.unrepresentable[0])
	}
	if set.selfIgnored {
		fmt.Fprintf(os.Stderr,
			"rsync2project: note: an enclosing git repo ignores %s itself, so git\n"+
				"  reported nothing about its contents. Copying everything under it.\n"+
				"  Use --extra PATTERN to trim, or --all to silence .gitignore entirely.\n",
			filepath.Base(source))
	}
	return gitignoreFilter{
		enabled:  true,
		fromGit:  true,
		excludes: set.rsyncExcludes(source, opts.contents),
	}
}

// gitignoreMode renders the filter state for the banner. The git/approx
// distinction is worth surfacing: the approximate path silently drops
// negated and tracked-but-matched files, so knowing which one ran explains
// an otherwise baffling missing file.
func gitignoreMode(gf gitignoreFilter) string {
	switch {
	case !gf.enabled:
		return "off"
	case gf.fromGit:
		return "on (via git)"
	case gf.fallbackReason != "":
		return "on (approximate rsync filter: " + gf.fallbackReason + ")"
	default:
		return "on (approximate rsync filter)"
	}
}

// printRunBanner states where the tree is going and which filter layers are
// live, in one line on stderr. Both facts were previously invisible: rsync
// reports neither the nested landing path nor the reason a directory never
// showed up at the far end.
func printRunBanner(source, destination string, includes, excludes []string, gf gitignoreFilter, opts *options) {
	prefix := "rsync2project:"
	if opts.dryRun {
		prefix = "dry-run:"
	}
	fmt.Fprintf(os.Stderr, "%s %s -> %s\n", prefix, source, effectiveDest(source, destination, opts.contents))
	fmt.Fprintf(os.Stderr, "%s .gitignore %s | builtin excludes %s | %d include, %d exclude patterns\n",
		prefix, gitignoreMode(gf), onOff(!opts.noExcludes), len(includes), len(excludes))
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

// warnNestCollision flags a destination whose last segment already repeats
// the source name, which nests the project inside itself. See nestCollides.
func warnNestCollision(source, destination string, opts *options) {
	if !nestCollides(source, destination, opts.contents) {
		return
	}
	base := filepath.Base(source)
	fmt.Fprintf(os.Stderr,
		"rsync2project: note: destination already ends in %q and --contents was not given.\n"+
			"  Files will land in %s/, not %s/.\n"+
			"  Pass --contents to sync into the destination directly.\n",
		base, effectiveDest(source, destination, false), strings.TrimRight(destination, "/"))
}

func parseFlags() *options {
	opts := &options{}

	flag.BoolVar(&opts.dryRun, "dry-run", false, "show what would be transferred without copying")
	flag.BoolVar(&opts.dryRun, "n", false, "")
	flag.BoolVar(&opts.verbose, "verbose", false, "verbose rsync output; also prints the invoked command")
	flag.BoolVar(&opts.verbose, "v", false, "")
	flag.BoolVar(&opts.quiet, "quiet", false, "progress bar only, no per-file list (the pre-0.6 default)")
	flag.BoolVar(&opts.quiet, "q", false, "")
	flag.BoolVar(&opts.deleteExtras, "delete", false, "delete files on destination not present on source")
	flag.BoolVar(&opts.noGitignore, "no-gitignore", false, "don't use .gitignore as an rsync filter")
	flag.BoolVar(&opts.noExcludes, "no-excludes", false, "don't apply the builtin/project-type/user exclude lists")
	flag.BoolVar(&opts.all, "all", false, "copy everything: implies --no-gitignore and --no-excludes")
	flag.BoolVar(&opts.excludeVCS, "no-vcs", false, "exclude .git/.hg/.svn metadata")
	flag.BoolVar(&opts.contents, "contents", false, "copy source contents directly into destination instead of nesting under source name")
	flag.BoolVar(&opts.saveConfig, "save-config", false, "write current --dest and --include flags to ~/.config/rsync2project/repos/<basename>.conf for reuse")
	flag.BoolVar(&opts.showExcludes, "show-excludes", false, "print exclude list and exit")
	flag.BoolVar(&opts.listDests, "list-dests", false, "list named destinations and exit")
	flag.BoolVar(&opts.showVersion, "version", false, "print version and exit")
	flag.StringVar(&opts.destName, "dest", "", "named destination from ~/.config/rsync2project/destinations")
	flag.StringVar(&opts.destName, "d", "", "")
	flag.Var(&opts.extraExcludes, "extra", "additional exclude pattern (repeatable)")
	flag.Var(&opts.extraIncludes, "include", "include pattern that overrides .gitignore/excludes (repeatable)")

	flag.Usage = usage
	flag.Parse()

	// --all is pure sugar over the two precise flags, resolved here so
	// every downstream consumer (including --show-excludes) sees a single
	// consistent filter state instead of testing three booleans.
	if opts.all {
		opts.noGitignore = true
		opts.noExcludes = true
	}
	return opts
}

// resolveDest picks a destination using priority: explicit --dest, explicit
// positional, then the repo config's dest= directive as a silent default.
// A positional argument overrides a repo config default without error — the
// repo default exists precisely to be overridable. Only --dest + positional
// is a real conflict.
func resolveDest(opts *options, args []string, repoDefault string) (string, error) {
	if opts.destName != "" && len(args) > 1 {
		return "", fmt.Errorf("cannot supply both --dest and a positional destination")
	}
	if opts.destName != "" {
		return resolveDestination(opts.destName)
	}
	if len(args) >= 2 {
		return args[1], nil
	}
	if repoDefault != "" {
		return resolveDestination(repoDefault)
	}
	return "", fmt.Errorf("destination required (positional, --dest NAME, or 'dest = NAME' in repo config)")
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

Filtering: two independent layers drop files. The project's .gitignore, and
a builtin list of regenerable junk (node_modules/, __pycache__/, .venv/,
target/, plus per-project-type additions). --no-gitignore turns off only the
first; use --all to turn off both and copy the tree verbatim.

Options:
  -n, --dry-run         Preview without transferring (always lists files)
  -v, --verbose         Verbose rsync output (also prints invoked command)
  -q, --quiet           Progress bar only, no per-file list
      --delete          Delete files on destination not present on source
      --no-gitignore    Don't use .gitignore as an rsync filter
      --no-excludes     Don't apply the builtin/project-type/user exclude lists
      --all             Copy everything (--no-gitignore + --no-excludes)
      --no-vcs          Exclude .git/.hg/.svn
      --contents        Copy source contents into dest (don't nest by source name)
      --show-excludes   Print exclude list and exit
      --extra PATTERN   Additional exclude pattern (repeatable)
      --include PATTERN Re-include a path that .gitignore/excludes would drop (repeatable)
      --save-config     Persist current --dest and --include flags to per-repo config
  -d, --dest NAME       Named destination from config file
      --list-dests      List configured destinations and exit
      --version         Print version
  -h, --help            Show this help

Subcommands:
  dest                  Manage named destinations (see 'rsync2project dest --help')
  repo                  Inspect/remove per-repo configs (see 'rsync2project repo --help')
  config path           Print the rsync2project config directory

Examples:
  rsync2project --contents --all . user@host:/path/myapp
  rsync2project ~/code/myapp user@host:/path/
  rsync2project --dest name ~/code/myapp
  rsync2project -n --show-excludes ~/code/myapp
  rsync2project dest add mac fred@mac.local:/Users/fred/backup/
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
