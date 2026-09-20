package architecture

import (
	"os"
	"path/filepath"
	"reflect"
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
	legacy := map[string][]string{
		"module_database_sql": {
			"../modules/cloudagent/repository/mysql_registry.go",
			"../modules/cloudagent/repository/mysql_task_store.go",
			"../modules/contentpool/repository/discovery_store_mysql.go",
			"../modules/contentpool/repository/store_mysql.go",
			"../modules/identity/repository/store_mysql.go",
			"../modules/mediaaccount/repository/store_mysql.go",
			"../modules/profilebinding/repository/store_mysql.go",
			"../modules/profileguard/repository/store_mysql.go",
			"../modules/proxy/repository/store_mysql.go",
			"../modules/runtimebinding/repository/store_mysql.go",
		},
	}
	checks := []boundaryCheck{
		{name: "module_http_client", root: "../modules", pattern: regexp.MustCompile(`http\.Client\s*\{`), message: "modules must use internal/infra/client"},
		{name: "module_generic_infrastructure_import", root: "../modules", pattern: regexp.MustCompile(`github\.com/wt-media/wt-media-cloud/pkg/(config|logger|clients/http)`), message: "modules must use internal semantic infrastructure boundaries"},
		{name: "module_ticker", root: "../modules", pattern: regexp.MustCompile(`time\.NewTicker\(`), message: "modules must not own scheduler timing"},
		{name: "module_environment", root: "../modules", pattern: regexp.MustCompile(`os\.(Getenv|LookupEnv)\(`), message: "modules must not read process configuration"},
		{name: "module_database_sql", root: "../modules", pattern: regexp.MustCompile(`"database/sql"`), message: "module repositories must use the GORM database boundary"},
	}

	for _, check := range checks {
		check := check
		t.Run(check.name, func(t *testing.T) {
			matches, err := findMatches(check.root, check.pattern)
			if err != nil {
				t.Fatal(err)
			}
			allowed := legacy[check.name]
			if !reflect.DeepEqual(matches, allowed) {
				t.Fatalf("%s\nallowed legacy files:\n%s\nactual files:\n%s", check.message, strings.Join(allowed, "\n"), strings.Join(matches, "\n"))
			}
		})
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
