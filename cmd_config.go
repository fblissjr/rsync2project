package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

func runConfigCmd(args []string) int {
	if len(args) == 0 {
		configUsage(os.Stderr)
		return 2
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "path":
		return runConfigPath(rest)
	case "-h", "--help", "help":
		configUsage(os.Stdout)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "rsync2project config: unknown subcommand %q\n\n", sub)
		configUsage(os.Stderr)
		return 2
	}
}

func runConfigPath(args []string) int {
	fs := flag.NewFlagSet("config path", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: rsync2project config path")
	}
	if stop, code := parseSubFlags(fs, args); stop {
		return code
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return 2
	}
	fmt.Println(configDir())
	return 0
}

func configUsage(w io.Writer) {
	fmt.Fprint(w, `Usage: rsync2project config <subcommand>

Subcommands:
  path                  Print the rsync2project config directory
`)
}
