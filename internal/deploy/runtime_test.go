package deploy

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	// build-release-linux.sh builds every name in a single `for name in a b c; do`.
	releaseBuildLoop = regexp.MustCompile(`for name in ([a-z0-9 -]+); do`)
	// package_release_linux.py declares what is shipped and what must be running.
	releaseBinaries = regexp.MustCompile(`BINARIES = \(([^)]*)\)`)
	releaseProcess  = regexp.MustCompile(`"processes": \(([^)]*)\)`)
)

// namedFields splits a captured quoted list, tolerating both shell and Python
// quoting.
func namedFields(t *testing.T, match []string) []string {
	t.Helper()
	if match == nil {
		t.Fatalf("release manifest pattern not found; the release scripts changed shape")
	}
	var names []string
	for _, field := range strings.Fields(match[1]) {
		names = append(names, strings.Trim(field, `",`))
	}
	return names
}

// TestServiceProcessCheckCoversReleaseManifest keeps the release manifest and the
// process check in step. A binary that is packaged and expected to run but never
// verified lets `wtmctl deploy verify` stay green while that work never executes;
// a binary the check waits for but the release does not ship can never come up.
func TestServiceProcessCheckCoversReleaseManifest(t *testing.T) {
	root := filepath.Join("..", "..")
	script, err := os.ReadFile(filepath.Join(root, "scripts", "dev", "build-release-linux.sh"))
	if err != nil {
		t.Fatalf("read build script: %v", err)
	}
	packager, err := os.ReadFile(filepath.Join(root, "scripts", "dev", "package_release_linux.py"))
	if err != nil {
		t.Fatalf("read packager: %v", err)
	}

	built := namedFields(t, releaseBuildLoop.FindStringSubmatch(string(script)))
	shipped := namedFields(t, releaseBinaries.FindStringSubmatch(string(packager)))
	expected := namedFields(t, releaseProcess.FindStringSubmatch(string(packager)))
	// The HTTP entrypoint is cmd/server, released under the product name.
	built = append(built, "wt-media-cloud")

	builtSet := make(map[string]bool, len(built))
	for _, name := range built {
		builtSet[name] = true
	}
	shippedSet := make(map[string]bool, len(shipped))
	for _, name := range shipped {
		shippedSet[name] = true
	}
	expectedSet := make(map[string]bool, len(expected))
	for _, name := range expected {
		expectedSet[name] = true
	}

	wanted := ServiceProcessBinaries(Profile{Deploy: ProfileDeploy{InstallRoot: "/home/www/wt-media-cloud"}})
	covered := make(map[string]bool, len(wanted))
	for _, binary := range wanted {
		covered[filepath.Base(binary)] = true
	}

	for _, name := range expected {
		if !shippedSet[name] {
			t.Errorf("release declares process %q but does not package it", name)
		}
		if !builtSet[name] {
			t.Errorf("release declares process %q but does not build it", name)
		}
		if !covered[name] {
			t.Errorf("release declares process %q but CheckServiceProcesses never verifies it", name)
		}
	}
	for name := range covered {
		if !shippedSet[name] {
			t.Errorf("CheckServiceProcesses verifies %q but the release does not package it", name)
		}
		if !expectedSet[name] {
			t.Errorf("CheckServiceProcesses verifies %q but the release manifest does not declare it a process", name)
		}
	}
}
