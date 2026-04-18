package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// runRsync builds the rsync argv and execs it, streaming stdio to the user.
func runRsync(source, destination string, excludes []string, opts *options) error {
	if _, err := exec.LookPath("rsync"); err != nil {
		return fmt.Errorf("rsync not found on PATH; please install rsync")
	}

	args := []string{
		"-a",        // archive: recursive, preserve perms/times/symlinks/etc.
		"-h",        // human-readable sizes
		"--partial", // keep partially transferred files to allow resume
	}
	if opts.verbose {
		args = append(args, "-v", "--stats")
	} else {
		args = append(args, "--info=progress2,stats1")
	}
	if opts.dryRun {
		args = append(args, "--dry-run")
	}
	if opts.deleteExtras {
		// Deliberately not --delete-excluded: we don't want to wipe an
		// existing .venv on the destination just because our exclude list
		// grew since the last sync.
		args = append(args, "--delete")
	}
	if !opts.noGitignore {
		args = append(args, "--filter=:- .gitignore")
	}
	if looksRemote(destination) {
		args = append(args, "-z")
	}
	for _, e := range excludes {
		args = append(args, "--exclude="+e)
	}

	// Default: treat source as "copy its contents" by appending a trailing
	// slash. --keep-name passes the source through as-is, which nests the
	// source dir under the destination (standard rsync "no trailing slash"
	// semantics).
	src := source
	if !opts.keepName && !strings.HasSuffix(src, "/") {
		src += "/"
	}
	args = append(args, src, destination)

	if opts.verbose {
		fmt.Fprintln(os.Stderr, "+ rsync", strings.Join(args, " "))
	}

	cmd := exec.Command("rsync", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rsync failed: %w", err)
	}
	return nil
}

// looksRemote heuristically decides whether dest is a remote rsync target, so
// we can enable compression (-z) only where it actually pays off.
func looksRemote(dest string) bool {
	if strings.Contains(dest, "://") {
		return true
	}
	colon := strings.IndexByte(dest, ':')
	slash := strings.IndexByte(dest, '/')
	if colon < 0 {
		return false
	}
	if slash < 0 {
		return true
	}
	return colon < slash
}
