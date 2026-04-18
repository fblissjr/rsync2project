package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

// failMsg prints a standard "rsync2project: <err>" line to stderr and
// returns exit code 1. Used inside subcommand handlers (which return an
// int) in place of main's fail() (which calls os.Exit and skips tests).
func failMsg(err error) int {
	fmt.Fprintln(os.Stderr, "rsync2project:", err)
	return 1
}

// parseSubFlags wraps fs.Parse so -h/--help exits 0 rather than 2.
// Returns (stop, exitCode): when stop is true the caller should return
// exitCode; when false, Parse succeeded and the caller continues.
func parseSubFlags(fs *flag.FlagSet, args []string) (stop bool, exitCode int) {
	err := fs.Parse(args)
	if err == nil {
		return false, 0
	}
	if errors.Is(err, flag.ErrHelp) {
		return true, 0
	}
	return true, 2
}

// addDryRunFlag binds -n and --dry-run to the same bool so every
// mutating subcommand shares one alias contract.
func addDryRunFlag(fs *flag.FlagSet, dst *bool) {
	fs.BoolVar(dst, "n", false, "preview without writing")
	fs.BoolVar(dst, "dry-run", false, "preview without writing")
}
