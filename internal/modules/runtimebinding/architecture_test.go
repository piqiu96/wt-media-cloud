package runtimebinding

import (
	"os/exec"
	"strings"
	"testing"
)

func TestProfileRuntimeModulesRejectCrossModuleRepositoryImports(t *testing.T) {
	scans := []struct {
		dir     string
		pattern string
	}{
		{dir: "../profilebinding", pattern: `internal/modules/(runtimebinding|profileguard)/repository`},
		{dir: "../profileguard", pattern: `internal/modules/(runtimebinding|profilebinding)/repository`},
	}
	for _, scan := range scans {
		cmd := exec.Command("rg", "-n", scan.pattern, scan.dir, "--glob", "*.go")
		output, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("cross-module repository imports found in %s:\n%s", scan.dir, output)
		}
		if !strings.Contains(string(output), "no matches") && len(output) != 0 {
			t.Fatalf("rg scan failed for %s: %s", scan.dir, output)
		}
	}
}
