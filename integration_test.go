package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func requireRsync(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync not available on PATH; skipping integration test")
	}
}

// setupFakeProject creates a project directory named "testproj" inside a
// fresh temp parent and populates it with the given relative paths. Returns
// the absolute source path.
func setupFakeProject(t *testing.T, files map[string]string) string {
	t.Helper()
	parent := t.TempDir()
	src := filepath.Join(parent, "testproj")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	for rel, content := range files {
		mustWrite(t, filepath.Join(src, rel), content)
	}
	return src
}

// TestIntegrationSyncDefault shells out to rsync and verifies both the
// curated exclude list and the default "nest under source name" behavior.
func TestIntegrationSyncDefault(t *testing.T) {
	requireRsync(t)

	src := setupFakeProject(t, map[string]string{
		"pyproject.toml":                  "[project]\nname=\"x\"\n",
		"main.py":                         "print('hello')\n",
		"package.json":                    "{}\n",
		"src/app.ts":                      "export {};\n",
		".venv/bin/python":                "binary",
		"node_modules/left-pad/index.js":  "module.exports = x => x;\n",
		"src/__pycache__/app.cpython.pyc": "compiled",
		"pkg.egg-info/PKG-INFO":           "Metadata-Version: 2.1\n",
		".DS_Store":                       "mac-metadata",
		"build/lib/module.py":             "build artifact",
		"dist/wheel.whl":                  "wheel artifact",
	})
	dst := t.TempDir()

	types := detectProjectTypes(src)
	opts := &options{}
	excludes := buildExcludes(types, opts.excludeVCS)

	if err := runRsync(src, dst+"/", excludes, opts); err != nil {
		t.Fatalf("runRsync: %v", err)
	}

	// With the default (nest) behavior, everything lands under dst/testproj/.
	nest := filepath.Join(dst, "testproj")

	mustExist := []string{"main.py", "package.json", "src/app.ts"}
	mustMiss := []string{
		".venv",
		"node_modules",
		"src/__pycache__",
		"pkg.egg-info",
		".DS_Store",
		"build",
		"dist",
	}
	for _, rel := range mustExist {
		if _, err := os.Stat(filepath.Join(nest, rel)); err != nil {
			t.Errorf("expected %s to be copied under nest, stat error: %v", rel, err)
		}
	}
	for _, rel := range mustMiss {
		if _, err := os.Stat(filepath.Join(nest, rel)); err == nil {
			t.Errorf("expected %s to be excluded, but it exists at destination", rel)
		} else if !os.IsNotExist(err) {
			t.Errorf("unexpected error checking %s: %v", rel, err)
		}
	}
}

// TestIntegrationContents verifies --contents spills the source's files
// directly into the destination without the intermediate directory.
func TestIntegrationContents(t *testing.T) {
	requireRsync(t)

	src := setupFakeProject(t, map[string]string{"file.txt": "hi\n"})
	dst := t.TempDir()

	opts := &options{contents: true}
	if err := runRsync(src, dst+"/", nil, opts); err != nil {
		t.Fatalf("runRsync: %v", err)
	}

	direct := filepath.Join(dst, "file.txt")
	if _, err := os.Stat(direct); err != nil {
		t.Errorf("expected file directly in dest at %s, got: %v", direct, err)
	}
	nested := filepath.Join(dst, "testproj")
	if _, err := os.Stat(nested); err == nil {
		t.Errorf("did not expect nested dir %s with --contents, but it exists", nested)
	}
}
