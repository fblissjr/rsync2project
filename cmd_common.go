package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
)

// boolFlag matches the standard library flag package's internal interface
// for flags that do not consume a following value argument.
type boolFlag interface {
	flag.Value
	IsBoolFlag() bool
}

// failMsg prints a standard "rsync2project: <err>" line to stderr and
// returns exit code 1. Used inside subcommand handlers (which return an
// int) in place of main's fail() (which calls os.Exit and skips tests).
func failMsg(err error) int {
	fmt.Fprintln(os.Stderr, "rsync2project:", err)
	return 1
}

// parseInterspersed parses flags from args into fs while allowing flags and
// positional arguments to appear in any order. Go's standard flag.FlagSet.Parse
// stops at the first non-flag argument, which silently turns trailing flags
// (e.g. `rsync2project src dst --all`) into extra positional arguments.
// A bare "--" stops flag parsing and treats all remaining arguments as
// positional. On success, fs.Args() holds the positional arguments in order.
func parseInterspersed(fs *flag.FlagSet, args []string) error {
	var positionals []string
	rem := args
	for len(rem) > 0 {
		arg := rem[0]
		if arg == "--" {
			positionals = append(positionals, rem[1:]...)
			break
		}
		if len(arg) < 2 || arg[0] != '-' {
			positionals = append(positionals, arg)
			rem = rem[1:]
			continue
		}

		// arg is a flag token ("-x" or "--xxx"). Determine whether it
		// consumes the next element of rem as its value argument.
		n := 1
		name := arg[1:]
		if strings.HasPrefix(name, "-") {
			name = name[1:]
		}
		if !strings.Contains(name, "=") {
			if f := fs.Lookup(name); f != nil {
				if bf, ok := f.Value.(boolFlag); !ok || !bf.IsBoolFlag() {
					if len(rem) >= 2 {
						n = 2
					}
				}
			}
		}
		if err := fs.Parse(rem[:n]); err != nil {
			return err
		}
		rem = rem[n:]
	}
	return fs.Parse(append([]string{"--"}, positionals...))
}

// parseSubFlags wraps parseInterspersed so -h/--help exits 0 rather than 2.
// Returns (stop, exitCode): when stop is true the caller should return
// exitCode; when false, Parse succeeded and the caller continues.
func parseSubFlags(fs *flag.FlagSet, args []string) (stop bool, exitCode int) {
	err := parseInterspersed(fs, args)
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
