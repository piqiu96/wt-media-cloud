package deploy

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	cloudconfig "github.com/wt-media/wt-media-cloud/internal/config"
)

var placeholderPattern = regexp.MustCompile(`\{\{([A-Z][A-Z0-9_]*)\}\}`)

func TemplateVariableNames(configDir string) ([]string, error) {
	names := map[string]bool{}
	err := filepath.WalkDir(configDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("config templates may not contain symlinks: %s", path)
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".toml.tpl") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range placeholderPattern.FindAllSubmatch(raw, -1) {
			names[string(match[1])] = true
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	sort.Strings(result)
	return result, nil
}

func RenderConfig(configDir, variablesFile, environment string, schema VariableSchema) (string, error) {
	if environment != "pre" && environment != "online" {
		return "", errors.New("environment must be pre or online")
	}
	values, _, digest, err := LoadVariables(variablesFile)
	if err != nil {
		return "", err
	}
	if err := validateVariableValues(values, schema); err != nil {
		return "", err
	}
	parent := filepath.Dir(configDir)
	temp, err := os.MkdirTemp(parent, "."+filepath.Base(configDir)+".render-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(temp)
	if err := renderTree(configDir, temp, values); err != nil {
		return "", err
	}
	if _, err := cloudconfig.LoadFromDir(temp); err != nil {
		return "", fmt.Errorf("rendered configuration invalid: %w", err)
	}
	info := map[string]any{
		"schema_version":   1,
		"environment":      environment,
		"variables_sha256": digest,
		"rendered_at":      time.Now().UTC().Format(time.RFC3339),
	}
	infoRaw, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(temp, ".render-info.json"), append(infoRaw, '\n'), 0o600); err != nil {
		return "", err
	}

	backup := configDir + fmt.Sprintf(".templates-%d", os.Getpid())
	if err := os.Rename(configDir, backup); err != nil {
		return "", err
	}
	if err := os.Rename(temp, configDir); err != nil {
		_ = os.Rename(backup, configDir)
		return "", err
	}
	if err := os.RemoveAll(backup); err != nil {
		return "", fmt.Errorf("remove template backup: %w", err)
	}
	if err := chmodTree(configDir, 0o700, 0o600); err != nil {
		return "", err
	}
	return digest, nil
}

func renderTree(source, destination string, values map[string]any) error {
	used := map[string]bool{}
	err := filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("config templates may not contain symlinks: %s", relative)
		}
		if entry.IsDir() {
			return os.MkdirAll(filepath.Join(destination, relative), 0o700)
		}
		target := filepath.Join(destination, relative)
		if strings.HasSuffix(entry.Name(), ".toml.tpl") {
			target = strings.TrimSuffix(target, ".tpl")
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			text := string(raw)
			for _, name := range placeholderPattern.FindAllStringSubmatch(text, -1) {
				if _, ok := values[name[1]]; !ok {
					return fmt.Errorf("missing variables: %s", name[1])
				}
				used[name[1]] = true
			}
			rendered, err := replacePlaceholders(text, values)
			if err != nil {
				return fmt.Errorf("render %s: %w", relative, err)
			}
			if strings.Contains(rendered, "{{") || strings.Contains(rendered, "}}") {
				return fmt.Errorf("unresolved placeholder in %s", relative)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return err
			}
			return os.WriteFile(target, []byte(rendered), 0o600)
		}
		if strings.HasSuffix(entry.Name(), ".toml") {
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return err
			}
			return copyFile(path, target, 0o600)
		}
		return nil
	})
	if err != nil {
		return err
	}
	var unknown []string
	for name := range values {
		if !used[name] {
			unknown = append(unknown, name)
		}
	}
	sort.Strings(unknown)
	if len(unknown) > 0 {
		return fmt.Errorf("unknown variables: %s", strings.Join(unknown, ", "))
	}
	return nil
}

func replacePlaceholders(text string, values map[string]any) (string, error) {
	var renderErr error
	rendered := placeholderPattern.ReplaceAllStringFunc(text, func(token string) string {
		if renderErr != nil {
			return token
		}
		match := placeholderPattern.FindStringSubmatch(token)
		literal, err := tomlLiteral(values[match[1]])
		if err != nil {
			renderErr = err
			return token
		}
		return literal
	})
	return rendered, renderErr
}

func tomlLiteral(value any) (string, error) {
	switch typed := value.(type) {
	case string:
		raw, err := json.Marshal(typed)
		if err != nil {
			return "", err
		}
		return string(raw), nil
	case bool:
		return strconv.FormatBool(typed), nil
	case int64:
		return strconv.FormatInt(typed, 10), nil
	case int:
		return strconv.Itoa(typed), nil
	case int32:
		return strconv.FormatInt(int64(typed), 10), nil
	case uint:
		return strconv.FormatUint(uint64(typed), 10), nil
	case uint64:
		return strconv.FormatUint(typed, 10), nil
	case float64:
		return strconv.FormatFloat(typed, 'g', -1, 64), nil
	default:
		return "", fmt.Errorf("unsupported variable value type %T", value)
	}
}

func copyFile(source, target string, mode os.FileMode) error {
	raw, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if err := os.WriteFile(target, raw, mode); err != nil {
		return err
	}
	return os.Chmod(target, mode)
}

func chmodTree(root string, directoryMode, fileMode os.FileMode) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return os.Chmod(path, directoryMode)
		}
		return os.Chmod(path, fileMode)
	})
}
