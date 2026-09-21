package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wt-media/wt-media-cloud/pkg/config"
)

type decodeTarget struct {
	Name    string `toml:"name" yaml:"name" json:"name"`
	Count   int    `toml:"count" yaml:"count" json:"count"`
	Enabled bool   `toml:"enabled" yaml:"enabled" json:"enabled"`
}

func TestLoadFileSupportsTOMLJSONAndYAML(t *testing.T) {
	files := map[string]string{
		"item.toml": `name = "toml"` + "\n" + `count = 3` + "\n" + `enabled = true` + "\n",
		"item.json": `{"name":"json","count":4,"enabled":true}`,
		"item.yaml": "name: yaml\ncount: 5\nenabled: true\n",
	}
	for filename, content := range files {
		t.Run(filename, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), filename)
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}

			document, err := config.LoadFile(path)
			if err != nil {
				t.Fatalf("LoadFile() error = %v", err)
			}
			if document.Name != "item" || document.Path != path {
				t.Fatalf("document=%+v", document)
			}

			var target decodeTarget
			if err := document.Decode(&target); err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if !target.Enabled || target.Count == 0 || target.Name == "" {
				t.Fatalf("target=%+v", target)
			}
		})
	}
}

func TestLoadDirIsSortedAndNonRecursive(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "beta.toml", `name = "beta"`)
	writeConfig(t, root, "alpha.json", `{"name":"alpha"}`)
	writeConfig(t, root, filepath.Join("nested", "gamma.toml"), `name = "gamma"`)

	documents, err := config.LoadDir(root)
	if err != nil {
		t.Fatalf("LoadDir() error = %v", err)
	}
	if len(documents) != 2 || documents[0].Name != "alpha" || documents[1].Name != "beta" {
		t.Fatalf("documents=%+v", documents)
	}
}

func TestLoadDirRecursiveUsesRelativeNames(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "root.toml", `name = "root"`)
	writeConfig(t, root, filepath.Join("group", "child.toml"), `name = "child"`)

	documents, err := config.LoadDirRecursive(root)
	if err != nil {
		t.Fatalf("LoadDirRecursive() error = %v", err)
	}
	if len(documents) != 2 || documents[0].Name != "group/child" || documents[1].Name != "root" {
		t.Fatalf("documents=%+v", documents)
	}
}

func TestLoadDirRejectsDuplicateNamesAndUnsupportedFiles(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, "item.toml", `name = "toml"`)
	writeConfig(t, root, "item.json", `{"name":"json"}`)

	if _, err := config.LoadDir(root); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("LoadDir() error = %v", err)
	}

	root = t.TempDir()
	writeConfig(t, root, "README.md", "# ignored\n")
	if _, err := config.LoadDir(root); err == nil || !strings.Contains(err.Error(), "unsupported config format") {
		t.Fatalf("LoadDir() error = %v", err)
	}
}

func TestDecodeRejectsUnknownFields(t *testing.T) {
	tests := map[string]string{
		"item.toml": "name = \"x\"\nunknown = \"y\"\n",
		"item.json": `{"name":"x","unknown":"y"}`,
		"item.yaml": "name: x\nunknown: y\n",
	}
	for filename, content := range tests {
		t.Run(filename, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), filename)
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			document, err := config.LoadFile(path)
			if err != nil {
				t.Fatalf("LoadFile() error = %v", err)
			}
			var target decodeTarget
			if err := document.Decode(&target); err == nil {
				t.Fatal("Decode() error = nil")
			}
		})
	}
}

func TestLoadFileRejectsMissingFiles(t *testing.T) {
	_, err := config.LoadFile(filepath.Join(t.TempDir(), "missing.toml"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("LoadFile() error = %v", err)
	}
}

func writeConfig(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
