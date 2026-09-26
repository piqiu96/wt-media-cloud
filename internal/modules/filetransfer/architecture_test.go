package filetransfer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileTransferDoesNotImportProductionDomain(t *testing.T) {
	root := "."
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(content), "/internal/modules/production/") {
			t.Errorf("file-transfer code must not import production domain: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan file-transfer module: %v", err)
	}
}
