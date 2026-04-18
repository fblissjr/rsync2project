package main

import (
	"fmt"
	"io"
	"os"
)

func runConfigCmd(args []string) int {
	if len(args) == 0 {
		configUsage(os.Stderr)
		return 2
	}
	switch args[0] {
	case "path":
		if len(args) > 1 {
			fmt.Fprintln(os.Stderr, "Usage: rsync2project config path")
			return 2
		}
		fmt.Println(configDir())
		return 0
	case "-h", "--help", "help":
		configUsage(os.Stdout)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "rsync2project config: unknown subcommand %q\n\n", args[0])
		configUsage(os.Stderr)
		return 2
	}
}

func configUsage(w io.Writer) {
	fmt.Fprint(w, `Usage: rsync2project config <subcommand>

Subcommands:
  path                  Print the rsync2project config directory
`)
}
