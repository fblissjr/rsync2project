package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// gitIgnoreSet is the answer to "which paths under this source does git
// ignore?", plus enough context for the caller to explain itself.
type gitIgnoreSet struct {
	// ok reports that git answered authoritatively. When false the caller
	// must fall back to rsync's approximate dir-merge filter.
	ok bool
	// paths are source-relative slash paths. Directories carry a trailing
	// slash, which git emits when it collapses a wholly-ignored directory.
	paths []string
	// selfIgnored reports that an enclosing repo ignores the source
	// directory itself, so git had nothing to say about its contents.
	selfIgnored bool
	// unrepresentable holds ignored paths that cannot be written to a
	// line-based pattern file (see loadGitIgnoreSet).
	unrepresentable []string
	// why explains, when ok is false, what stopped git from answering.
	// Reported verbatim in the banner: "not a git repo" is routine, but
	// "git failed" is worth noticing, and conflating them tells the user
	// something false about their own project.
	why string
}

// loadGitIgnoreSet asks git which paths under source it ignores.
//
// Deriving the ignore set from git is the only faithful way to honor
// .gitignore. Replaying it through rsync's own filter engine
// (--filter=':- .gitignore') is an approximation that diverges in two ways,
// and both lose files rather than copy extra ones:
//
//   - Negation. Git's "!pattern" re-includes a path. rsync's exclude-only
//     dir-merge has no negation, so the line is read as an exclude of a
//     file literally named "!pattern" and whatever the project deliberately
//     un-ignored is silently dropped.
//   - Tracked files. Git never ignores a file it is tracking, even when a
//     pattern matches it. rsync has no notion of the index, so a committed
//     file matching an ignore rule vanishes from the copy — the worst case
//     for a tool whose job is backups.
//
// Asking git also picks up .git/info/exclude, core.excludesFile, and nested
// .gitignore precedence for free, none of which the dir-merge approximation
// handles the same way.
//
// A non-repo source, a missing git binary, or any git failure yields
// ok=false rather than an error: honoring .gitignore approximately is
// better than refusing to sync.
func loadGitIgnoreSet(source string) gitIgnoreSet {
	if _, err := exec.LookPath("git"); err != nil {
		return gitIgnoreSet{why: "git not installed"}
	}
	top, err := gitOutput(source, "rev-parse", "--show-toplevel")
	if err != nil || top == "" {
		return gitIgnoreSet{why: "not a git repo"}
	}

	// --full-name reports paths relative to the repo root, which stays
	// correct when source is a package inside a larger repo. Strip that
	// prefix back off to get source-relative paths, since rsync anchors
	// against the transfer root, not the repo root.
	//
	// git reports a fully symlink-resolved toplevel, so resolve the source
	// the same way before relating the two. Skipping this breaks any source
	// reached through a symlink (/tmp -> /private/tmp on macOS, and plenty
	// of home-directory setups): the relative path comes out as "../../..",
	// every prefix check then fails, and the ignore set arrives empty —
	// which reads as "nothing is ignored" and silently disables the filter.
	resolved := source
	if r, err := filepath.EvalSymlinks(source); err == nil {
		resolved = r
	}
	prefix := ""
	if rel, err := filepath.Rel(top, resolved); err != nil {
		return gitIgnoreSet{why: "source path could not be related to the repo root"}
	} else if rel != "." {
		// A source outside its own repo toplevel means the two paths could
		// not be related. Falling back to the approximate filter is the
		// safe failure: an empty set here would be indistinguishable from
		// "this repo ignores nothing" and would quietly copy everything.
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return gitIgnoreSet{why: "source path resolves outside the repo root"}
		}
		prefix = filepath.ToSlash(rel) + "/"
	}

	out, err := gitOutput(source, "ls-files", "-z", "--others", "--ignored",
		"--exclude-standard", "--directory", "--full-name")
	if err != nil {
		// git aborts here when the source sits inside an ignored directory
		// ("directory entry not superset of prefix"). Reporting that as
		// "not a git repo" would be a lie about the user's own project.
		return gitIgnoreSet{why: "git could not list ignored paths (source may be inside an ignored directory)"}
	}

	set := gitIgnoreSet{ok: true}
	for _, entry := range strings.Split(out, "\x00") {
		if entry == "" {
			continue
		}
		if prefix != "" {
			if !strings.HasPrefix(entry, prefix) {
				continue
			}
			entry = entry[len(prefix):]
		}
		// An empty or "./" entry means git collapsed the source directory
		// itself: an enclosing repo ignores the whole thing, so git never
		// looked inside and has no per-path detail to give. Turning that
		// into a pattern would exclude the entire transfer. Record it for
		// the caller to report and copy everything instead — syncing too
		// much is recoverable, silently syncing nothing is not.
		if entry == "" || entry == "./" {
			set.selfIgnored = true
			continue
		}
		// --exclude-from is line-based, and rsync's --from0 cannot rescue
		// it here: that flag also switches the .gitignore dir-merge to NUL
		// parsing, which breaks the receiver-side protection below. A path
		// containing a newline would split into two patterns, the second
		// unanchored and free to exclude unrelated files anywhere in the
		// tree. Dropping it copies one file too many; keeping it would
		// silently drop others.
		if strings.ContainsAny(entry, "\n\r") {
			set.unrepresentable = append(set.unrepresentable, entry)
			continue
		}
		set.paths = append(set.paths, entry)
	}
	return set
}

// rsyncExcludes renders the ignored set as anchored rsync patterns.
//
// Anchoring is load-bearing. rsync matches patterns against paths relative
// to the transfer root, and in the default (nest) mode that root sits one
// level above the source, so every path arrives as "<basename>/...". A
// pattern of "/models/" matches nothing there and the exclude silently
// no-ops; it has to be "/<basename>/models/". Under --contents the source's
// contents are the root, so a bare leading slash is right.
func (g gitIgnoreSet) rsyncExcludes(source string, contents bool) []string {
	root := "/"
	if !contents {
		root = "/" + filepath.Base(source) + "/"
	}
	out := make([]string, 0, len(g.paths))
	for _, p := range g.paths {
		// Escape the assembled pattern rather than its parts: whether a
		// backslash needs escaping depends on the whole pattern containing
		// a wildcard, so the basename and the path cannot be judged apart.
		out = append(out, escapeRsyncPattern(root+p))
	}
	return out
}

// escapeRsyncPattern quotes the wildcard metacharacters rsync would
// otherwise interpret, so a literal path from git matches only itself.
// Without this a file named "weird[1].txt" produces a character class that
// excludes "w1.txt" and leaves the actual file behind — the exclude lands
// on the wrong file entirely.
//
// The backslash is deliberately conditional. rsync only parses a pattern as
// a wildcard (and so only honors escapes) when it contains '*', '?' or '[';
// otherwise it compares literally, where a backslash is just a backslash.
// Escaping unconditionally therefore breaks the plain case: "back\slash"
// would become "back\\slash", still a literal comparison, now against a
// name with two backslashes — and the exclude silently misses.
func escapeRsyncPattern(p string) string {
	if !strings.ContainsAny(p, "*?[") {
		return p
	}
	var b strings.Builder
	b.Grow(len(p) + 8)
	for _, r := range p {
		switch r {
		case '\\', '*', '?', '[':
			b.WriteRune('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// gitOutput runs git in dir and returns trimmed stdout. Stderr is
// discarded: every failure here is a "git can't answer" signal that the
// caller turns into a fallback, not something to surface.
func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stderr = nil
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// writeTempPatternFile spills patterns to a temp file for --exclude-from.
// A file rather than repeated --exclude flags because a repo that ignores
// many individual paths (rather than whole directories git can collapse)
// would otherwise push the argv past ARG_MAX.
func writeTempPatternFile(patterns []string) (path string, cleanup func(), err error) {
	f, err := os.CreateTemp("", "rsync2project-ignore-*")
	if err != nil {
		return "", func() {}, err
	}
	cleanup = func() { _ = os.Remove(f.Name()) }
	for _, p := range patterns {
		if _, err := f.WriteString(p + "\n"); err != nil {
			f.Close()
			cleanup()
			return "", func() {}, err
		}
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return f.Name(), cleanup, nil
}
