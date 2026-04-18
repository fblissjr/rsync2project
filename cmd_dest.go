package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

// runDestCmd returns an exit code instead of calling os.Exit so callers
// (main, tests) decide how the process terminates.
func runDestCmd(args []string) int {
	if len(args) == 0 {
		if err := printDestinations(); err != nil {
			return failMsg(err)
		}
		return 0
	}

	sub, rest := args[0], args[1:]
	switch sub {
	case "list", "ls":
		return runDestList(rest)
	case "show", "cat":
		return runDestShow(rest)
	case "add", "set":
		return runDestAdd(rest)
	case "rm", "remove", "delete":
		return runDestRm(rest)
	case "-h", "--help", "help":
		destUsage(os.Stdout)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "rsync2project dest: unknown subcommand %q\n\n", sub)
		destUsage(os.Stderr)
		return 2
	}
}

func runDestList(args []string) int {
	fs := flag.NewFlagSet("dest list", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if stop, code := parseSubFlags(fs, args); stop {
		return code
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "rsync2project dest list: takes no arguments")
		return 2
	}
	if err := printDestinations(); err != nil {
		return failMsg(err)
	}
	return 0
}

func runDestShow(args []string) int {
	fs := flag.NewFlagSet("dest show", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: rsync2project dest show NAME")
	}
	if stop, code := parseSubFlags(fs, args); stop {
		return code
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return 2
	}
	v, err := resolveDestination(fs.Arg(0))
	if err != nil {
		return failMsg(err)
	}
	fmt.Println(v)
	return 0
}

func runDestAdd(args []string) int {
	fs := flag.NewFlagSet("dest add", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var dryRun bool
	addDryRunFlag(fs, &dryRun)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: rsync2project dest add [-n] NAME VALUE")
	}
	if stop, code := parseSubFlags(fs, args); stop {
		return code
	}
	if fs.NArg() != 2 {
		fs.Usage()
		return 2
	}
	name, value := fs.Arg(0), fs.Arg(1)

	if dryRun {
		fmt.Fprintf(os.Stderr, "dry-run: would set %s=%s in %s\n", name, value, destinationsPath())
		return 0
	}

	added, err := upsertDestination(name, value)
	if err != nil {
		return failMsg(err)
	}
	verb := "updated"
	if added {
		verb = "added"
	}
	fmt.Fprintf(os.Stderr, "%s: %s=%s\n", verb, name, value)
	return 0
}

func runDestRm(args []string) int {
	fs := flag.NewFlagSet("dest rm", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var dryRun bool
	addDryRunFlag(fs, &dryRun)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: rsync2project dest rm [-n] NAME")
	}
	if stop, code := parseSubFlags(fs, args); stop {
		return code
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return 2
	}
	name := fs.Arg(0)

	if dryRun {
		// Verify existence so dry-run can't falsely report a no-op removal.
		dests, err := parseKVFile(destinationsPath())
		if err != nil {
			return failMsg(err)
		}
		if _, ok := dests[name]; !ok {
			fmt.Fprintf(os.Stderr, "dry-run: no destination named %q\n", name)
			return 1
		}
		fmt.Fprintf(os.Stderr, "dry-run: would remove %q from %s\n", name, destinationsPath())
		return 0
	}

	if err := removeDestination(name); err != nil {
		return failMsg(err)
	}
	fmt.Fprintf(os.Stderr, "removed: %s\n", name)
	return 0
}

func destUsage(w io.Writer) {
	fmt.Fprint(w, `Usage: rsync2project dest <subcommand>

Manage named destinations in ~/.config/rsync2project/destinations.

Subcommands:
  list                  List configured destinations (default if no subcommand)
  show NAME             Print a single destination's value
  add NAME VALUE        Add or update a destination
  rm NAME               Remove a destination

Each mutating subcommand accepts -n / --dry-run to preview without writing.

Examples:
  rsync2project dest add mac fred@mac.local:/Users/fred/backup/
  rsync2project dest show mac
  rsync2project dest rm mac
`)
}
