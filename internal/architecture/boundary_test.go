package architecture

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

type boundaryCheck struct {
	name    string
	root    string
	pattern *regexp.Regexp
	message string
}

func TestProductionArchitectureBoundaries(t *testing.T) {
	checks := []boundaryCheck{
		{name: "module_http_client", root: "../modules", pattern: regexp.MustCompile(`http\.Client\s*\{`), message: "modules must use internal/infra/client"},
		{name: "module_generic_infrastructure_import", root: "../modules", pattern: regexp.MustCompile(`github\.com/wt-media/wt-media-cloud/pkg/(config|logger|clients/http)`), message: "modules must use internal semantic infrastructure boundaries"},
		{name: "module_ticker", root: "../modules", pattern: regexp.MustCompile(`time\.NewTicker\(`), message: "modules must not own scheduler timing"},
		{name: "module_environment", root: "../modules", pattern: regexp.MustCompile(`os\.(Getenv|LookupEnv)\(`), message: "modules must not read process configuration"},
		{name: "module_database_ownership", root: "../modules", pattern: regexp.MustCompile(`sql\.Open\s*\(|\*sql\.DB\b`), message: "modules must not create or own database connections"},
		{name: "module_identity_root_import", root: "../modules", pattern: regexp.MustCompile(`"github\.com/wt-media/wt-media-cloud/internal/modules/identity"`), message: "business modules must use middleware or identity Service APIs"},
	}

	for _, check := range checks {
		check := check
		t.Run(check.name, func(t *testing.T) {
			matches, err := findMatches(check.root, check.pattern)
			if err != nil {
				t.Fatal(err)
			}
			if len(matches) != 0 {
				t.Fatalf("%s\nactual files:\n%s", check.message, strings.Join(matches, "\n"))
			}
		})
	}
}

func TestRepositoriesDoNotImportForeignModules(t *testing.T) {
	matches, err := findForeignModuleImports("../modules", "repository")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("repositories must depend only on their own module types:\n%s", strings.Join(matches, "\n"))
	}
}

func TestModelsAndDTOsDoNotImportForeignModels(t *testing.T) {
	for _, layer := range []string{"model", "dto"} {
		matches, err := findForeignLayerImports("../modules", layer, "model")
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) != 0 {
			t.Fatalf("%s packages must not import foreign models:\n%s", layer, strings.Join(matches, "\n"))
		}
	}
}

func findMatches(root string, pattern *regexp.Regexp) ([]string, error) {
	var matches []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if pattern.Match(contents) {
			matches = append(matches, filepath.ToSlash(path))
		}
		return nil
	})
	sort.Strings(matches)
	return matches, err
}

func findForeignModuleImports(root, layer string) ([]string, error) {
	return findForeignLayerImports(root, layer, "service", "model", "dto", "repository", "handler")
}

func findForeignLayerImports(root, layer string, importedLayers ...string) ([]string, error) {
	allowedLayers := map[string]bool{}
	for _, importedLayer := range importedLayers {
		allowedLayers[importedLayer] = true
	}
	importPattern := regexp.MustCompile(`"github\.com/wt-media/wt-media-cloud/internal/modules/([^/"]+)/([^/"]+)"`)
	var matches []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		relative := filepath.ToSlash(path)
		parts := strings.Split(relative, "/")
		moduleIndex := -1
		for index, part := range parts {
			if part == layer && index > 0 && parts[index-1] != "modules" {
				moduleIndex = index - 1
				break
			}
			if part == layer && index >= 2 && parts[index-2] == "modules" {
				moduleIndex = index - 1
				break
			}
		}
		if moduleIndex < 0 {
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range importPattern.FindAllStringSubmatch(string(contents), -1) {
			if !allowedLayers[match[2]] {
				continue
			}
			if match[1] != parts[moduleIndex] {
				matches = append(matches, relative+" -> "+match[0])
			}
		}
		return nil
	})
	sort.Strings(matches)
	return matches, err
}
