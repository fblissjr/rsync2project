package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestIntegrationSync actually shells out to rsync and verifies that the
// curated exclude list filters real files. Skipped when rsync is not on PATH
// so the unit suite still runs in minimal CI environments.
func TestIntegrationSync(t *testing.T) {
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync not available on PATH; skipping integration test")
	}

	src := t.TempDir()
	dst := t.TempDir()

	// Build a fake project with real source plus dirs/files that must be
	// excluded: a Python venv, a node_modules tree, a __pycache__, and an
	// .egg-info.
	files := map[string]string{
		"main.py":                          "print('hello')\n",
		"package.json":                     "{}\n",
		"src/app.ts":                       "export {};\n",
		".venv/bin/python":                 "binary",
		"node_modules/left-pad/index.js":   "module.exports = x => x;\n",
		"src/__pycache__/app.cpython.pyc":  "compiled",
		"pkg.egg-info/PKG-INFO":            "Metadata-Version: 2.1\n",
		".DS_Store":                        "mac-metadata",
	}
	for rel, content := range files {
		full := filepath.Join(src, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	absSrc, err := filepath.Abs(src)
	if err != nil {
		t.Fatal(err)
	}
	types := detectProjectTypes(absSrc)
	opts := &options{}
	excludes := buildExcludes(types, opts)

	if err := runRsync(absSrc, dst+"/", excludes, opts); err != nil {
		t.Fatalf("runRsync: %v", err)
	}

	mustExist := []string{"main.py", "package.json", "src/app.ts"}
	mustMiss := []string{
		".venv",
		"node_modules",
		"src/__pycache__",
		"pkg.egg-info",
		".DS_Store",
	}
	for _, rel := range mustExist {
		if _, err := os.Stat(filepath.Join(dst, rel)); err != nil {
			t.Errorf("expected %s to be copied, stat error: %v", rel, err)
		}
	}
	for _, rel := range mustMiss {
		if _, err := os.Stat(filepath.Join(dst, rel)); err == nil {
			t.Errorf("expected %s to be excluded, but it exists at destination", rel)
		} else if !os.IsNotExist(err) {
			t.Errorf("unexpected error checking %s: %v", rel, err)
		}
	}
}

func TestIntegrationKeepName(t *testing.T) {
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync not available on PATH; skipping integration test")
	}

	parent := t.TempDir()
	src := filepath.Join(parent, "myproject")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "file.txt"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := t.TempDir()

	opts := &options{keepName: true}
	if err := runRsync(src, dst+"/", nil, opts); err != nil {
		t.Fatalf("runRsync: %v", err)
	}

	// With --keep-name, rsync should nest the source directory under the dest.
	nested := filepath.Join(dst, "myproject", "file.txt")
	if _, err := os.Stat(nested); err != nil {
		t.Errorf("expected nested file at %s, got: %v", nested, err)
	}
}
