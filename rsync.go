package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// gitignoreFilter tells runRsync how to honor .gitignore: with the exact
// path set git computed, or with rsync's approximate dir-merge when git
// couldn't answer. See loadGitIgnoreSet for why the two differ.
type gitignoreFilter struct {
	enabled  bool
	fromGit  bool
	excludes []string
}

// runRsync builds the rsync argv and execs it, streaming stdio to the user.
func runRsync(source, destination string, includes, excludes []string, gf gitignoreFilter, opts *options) error {
	if _, err := exec.LookPath("rsync"); err != nil {
		return fmt.Errorf("rsync not found on PATH; please install rsync")
	}

	args := []string{
		"-a",        // archive: recursive, preserve perms/times/symlinks/etc.
		"-h",        // human-readable sizes
		"--partial", // keep partially transferred files to allow resume
	}
	// Output mode. The default itemizes every changed path: a sync you can't
	// see is a sync you can't trust, and the old progress-only default
	// printed a byte counter and nothing else — a run that transferred
	// nothing looked identical to one that transferred everything.
	// --info=progress2 is deliberately not combined with -i: the progress
	// line rewrites itself with \r and shreds the itemized output.
	switch {
	case opts.verbose:
		args = append(args, "-v", "-i", "--stats")
	case opts.quiet && !opts.dryRun:
		args = append(args, "--info=progress2,stats1")
	default:
		args = append(args, "-i", "--info=stats1")
	}
	if opts.dryRun {
		// --quiet never suppresses the dry-run listing. A preview whose
		// whole purpose is "show me what would move" has nothing left if
		// the file list is dropped.
		args = append(args, "--dry-run")
	}
	if opts.deleteExtras {
		// Deliberately not --delete-excluded: we don't want to wipe an
		// existing .venv on the destination just because our exclude list
		// grew since the last sync.
		args = append(args, "--delete")
	}
	// Includes must precede the gitignore filter and the baseline excludes
	// because rsync is first-match-wins. Anything matched here survives any
	// subsequent exclude/gitignore rule.
	for _, i := range includes {
		args = append(args, "--include="+i)
	}
	// Same slot the dir-merge filter always occupied: after --include (which
	// must keep winning) and before the baseline excludes.
	switch {
	case !gf.enabled:
		// nothing
	case gf.fromGit:
		if len(gf.excludes) > 0 {
			path, cleanup, err := writeTempPatternFile(gf.excludes)
			if err != nil {
				return err
			}
			defer cleanup()
			args = append(args, "--exclude-from="+path)
		}
	default:
		args = append(args, "--filter=:- .gitignore")
	}
	if looksRemote(destination) {
		args = append(args, "-z")
	}
	for _, e := range excludes {
		args = append(args, "--exclude="+e)
	}

	// Default: pass the source through as-is, so rsync's native "no trailing
	// slash" semantics nest the source directory under the destination. This
	// matches the common "back up each project into a parent dir" intent.
	// --contents appends a trailing slash to spill the source's contents
	// directly into the destination, which is what you want when the
	// destination path already names the target project (e.g. a dev mirror).
	src := source
	if opts.contents && !strings.HasSuffix(src, "/") {
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

// destPathPart strips an rsync target's transport/host prefix and returns
// just the path portion, so path-shape checks work identically for local
// paths, user@host:path, and rsync:// URLs.
func destPathPart(dest string) string {
	if i := strings.Index(dest, "://"); i >= 0 {
		rest := dest[i+3:]
		if j := strings.IndexByte(rest, '/'); j >= 0 {
			return rest[j:]
		}
		return ""
	}
	colon := strings.IndexByte(dest, ':')
	slash := strings.IndexByte(dest, '/')
	if colon >= 0 && (slash < 0 || colon < slash) {
		return dest[colon+1:]
	}
	return dest
}

// destBase returns the final path segment of a destination, ignoring any
// trailing slashes (which rsync ignores for the destination too).
func destBase(dest string) string {
	p := strings.TrimRight(destPathPart(dest), "/")
	if p == "" {
		return ""
	}
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[i+1:]
	}
	return p
}

// effectiveDest reports where the source tree actually lands: the
// destination itself under --contents, otherwise destination/<basename>
// because rsync nests a source given without a trailing slash. Printed in
// the run banner so the landing path is visible before the transfer rather
// than hunted for afterwards.
func effectiveDest(source, dest string, contents bool) string {
	if contents {
		return dest
	}
	base := filepath.Base(source)
	// "host:" (remote home) has no path to join onto; appending a slash
	// would turn a home-relative target into an absolute one.
	if strings.HasSuffix(dest, ":") {
		return dest + base
	}
	return strings.TrimRight(dest, "/") + "/" + base
}

// nestCollides reports the common footgun of naming the project twice:
// `rsync2project . host:/path/myapp` nests, so the files land in
// .../myapp/myapp/ rather than in the directory the user pointed at.
// This is correct rsync behavior and a myapp/myapp/ layout is legal, so
// callers warn and proceed rather than refusing.
func nestCollides(source, dest string, contents bool) bool {
	if contents {
		return false
	}
	base := filepath.Base(source)
	if base == "." || base == string(filepath.Separator) || base == "" {
		return false
	}
	return destBase(dest) == base
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
