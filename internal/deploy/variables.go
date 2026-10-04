package deploy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	toml "github.com/pelletier/go-toml/v2"
)

const maxVariablesBytes = 2 << 20

func LoadVariables(path string) (map[string]any, []byte, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, "", fmt.Errorf("read variables: %w", err)
	}
	var values map[string]any
	if err := toml.Unmarshal(raw, &values); err != nil {
		return nil, nil, "", fmt.Errorf("parse variables TOML: %w", err)
	}
	sum := sha256.Sum256(raw)
	return values, raw, hex.EncodeToString(sum[:]), nil
}

func CheckVariables(path string, schema VariableSchema) (string, int, error) {
	values, _, digest, err := LoadVariables(path)
	if err != nil {
		return "", 0, err
	}
	if err := validateVariableValues(values, schema); err != nil {
		return "", 0, err
	}
	return digest, len(values), nil
}

func validateVariableValues(values map[string]any, schema VariableSchema) error {
	expected := make(map[string]VariableSpec, len(schema.Variables))
	for _, spec := range schema.Variables {
		expected[spec.Name] = spec
	}
	var unknown []string
	for name := range values {
		if _, ok := expected[name]; !ok {
			unknown = append(unknown, name)
		}
	}
	if len(unknown) > 0 {
		return fmt.Errorf("unknown variables: %s", strings.Join(unknown, ", "))
	}
	for _, spec := range schema.Variables {
		value, ok := values[spec.Name]
		if !ok {
			if spec.Required {
				return fmt.Errorf("missing required variable: %s", spec.Name)
			}
			continue
		}
		switch spec.Type {
		case "string":
			text, ok := value.(string)
			if !ok {
				return fmt.Errorf("variable %s must be a string", spec.Name)
			}
			trimmed := strings.TrimSpace(text)
			if spec.Required && trimmed == "" {
				return fmt.Errorf("required variable %s is empty", spec.Name)
			}
			upper := strings.ToUpper(trimmed)
			if strings.HasPrefix(trimmed, "<") || strings.Contains(upper, "TODO") || strings.Contains(upper, "CHANGE_ME") || strings.Contains(upper, "REPLACE") {
				return fmt.Errorf("variable %s still contains a placeholder", spec.Name)
			}
			if spec.Required && !spec.AllowEmpty && trimmed == "" {
				return fmt.Errorf("required variable %s is empty", spec.Name)
			}
		case "integer":
			switch value.(type) {
			case int64, int, int32, uint, uint64:
			default:
				return fmt.Errorf("variable %s must be an integer", spec.Name)
			}
		case "boolean":
			if _, ok := value.(bool); !ok {
				return fmt.Errorf("variable %s must be a boolean", spec.Name)
			}
		}
	}
	return nil
}

func PullVariables(ctx context.Context, source, target string, schema VariableSchema) (string, int, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) > 3 {
				return errors.New("too many redirects")
			}
			return nil
		},
	}
	return pullVariablesWithClient(ctx, source, target, schema, client)
}

func pullVariablesWithClient(ctx context.Context, source, target string, schema VariableSchema, client *http.Client) (string, int, error) {
	if !strings.HasPrefix(source, "https://") {
		return "", 0, errors.New("variables URL must use https://")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return "", 0, err
	}
	if token := os.Getenv("WT_CONFIG_BEARER_TOKEN"); token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := client.Do(request)
	if err != nil {
		return "", 0, fmt.Errorf("variables request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("variables request failed: HTTP %d", response.StatusCode)
	}
	limited := io.LimitReader(response.Body, maxVariablesBytes+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return "", 0, fmt.Errorf("read variables response: %w", err)
	}
	if len(raw) > maxVariablesBytes {
		return "", 0, errors.New("variables response exceeds 2 MiB")
	}
	var values map[string]any
	if err := toml.Unmarshal(raw, &values); err != nil {
		return "", 0, fmt.Errorf("parse remote variables TOML: %w", err)
	}
	if err := validateVariableValues(values, schema); err != nil {
		return "", 0, err
	}
	if err := writePrivateFileAtomic(target, raw); err != nil {
		return "", 0, err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), len(values), nil
}

func writePrivateFileAtomic(path string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-")
	if err != nil {
		return err
	}
	temp := file.Name()
	defer os.Remove(temp)
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(raw); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(temp, path)
}
