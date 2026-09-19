// Package config provides generic configuration file and directory loading.
// It owns no application schema and publishes no mutable global state.
package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

// Format identifies a supported configuration encoding.
type Format string

const (
	FormatYAML Format = "yaml"
	FormatJSON Format = "json"
	FormatTOML Format = "toml"
)

// Document is one decoded configuration file and its source metadata.
type Document struct {
	Name   string
	Path   string
	Format Format
	Raw    []byte
	Data   map[string]any
}

// LoadFile reads one regular YAML, JSON, or TOML file.
func LoadFile(path string) (Document, error) {
	format, err := formatForPath(path)
	if err != nil {
		return Document{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return Document{}, fmt.Errorf("stat config %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return Document{}, fmt.Errorf("config %s is not a regular file", path)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return Document{}, fmt.Errorf("read config %s: %w", path, err)
	}
	document := Document{
		Name:   documentName(filepath.Base(path)),
		Path:   path,
		Format: format,
		Raw:    raw,
	}
	if err := document.decodeData(); err != nil {
		return Document{}, fmt.Errorf("decode config %s: %w", path, err)
	}
	return document, nil
}

// LoadDir reads supported files from one directory without descending into child directories.
func LoadDir(path string) ([]Document, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("read config dir %s: %w", path, err)
	}
	documents := make([]Document, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		document, err := loadDirEntry(path, entry.Name())
		if err != nil {
			return nil, err
		}
		if err := appendDocument(&documents, seen, document); err != nil {
			return nil, err
		}
	}
	sortDocuments(documents)
	return documents, nil
}

// LoadDirRecursive reads supported files below path using deterministic slash-separated relative names.
func LoadDirRecursive(path string) ([]Document, error) {
	documents := make([]Document, 0)
	seen := make(map[string]struct{})
	err := filepath.WalkDir(path, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relativePath, relErr := filepath.Rel(path, current)
		if relErr != nil {
			return relErr
		}
		document, err := loadDirEntry(path, filepath.ToSlash(relativePath))
		if err != nil {
			return err
		}
		if err := appendDocument(&documents, seen, document); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan config dir %s: %w", path, err)
	}
	sortDocuments(documents)
	return documents, nil
}

// Decode strictly decodes Raw into target using the document's original format.
func (d Document) Decode(target any) error {
	decoder := strictDecoder(d.Format, d.Raw)
	if decoder == nil {
		return fmt.Errorf("unsupported config format %q", d.Format)
	}
	return decoder(target)
}

func loadDirEntry(root, relativePath string) (Document, error) {
	path := filepath.Join(root, filepath.FromSlash(relativePath))
	format, err := formatForPath(path)
	if err != nil {
		return Document{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return Document{}, fmt.Errorf("inspect config %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return Document{}, fmt.Errorf("config %s is not a regular file", path)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return Document{}, fmt.Errorf("read config %s: %w", path, err)
	}
	document := Document{
		Name:   documentName(relativePath),
		Path:   path,
		Format: format,
		Raw:    raw,
	}
	if err := document.decodeData(); err != nil {
		return Document{}, fmt.Errorf("decode config %s: %w", path, err)
	}
	return document, nil
}

func (d *Document) decodeData() error {
	var data map[string]any
	if err := d.Decode(&data); err != nil {
		return err
	}
	d.Data = data
	return nil
}

func strictDecoder(format Format, raw []byte) func(any) error {
	switch format {
	case FormatYAML:
		return func(target any) error {
			decoder := yaml.NewDecoder(bytes.NewReader(raw))
			decoder.KnownFields(true)
			if err := decoder.Decode(target); err != nil {
				return err
			}
			var extra any
			if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
				if err == nil {
					return errors.New("unexpected trailing YAML content")
				}
				return err
			}
			return nil
		}
	case FormatJSON:
		return func(target any) error {
			decoder := json.NewDecoder(bytes.NewReader(raw))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(target); err != nil {
				return err
			}
			if decoder.More() {
				return errors.New("unexpected trailing JSON content")
			}
			return nil
		}
	case FormatTOML:
		return func(target any) error {
			decoder := toml.NewDecoder(bytes.NewReader(raw))
			return decoder.DisallowUnknownFields().Decode(target)
		}
	default:
		return nil
	}
}

func formatForPath(path string) (Format, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		return FormatYAML, nil
	case ".json":
		return FormatJSON, nil
	case ".toml":
		return FormatTOML, nil
	default:
		return "", fmt.Errorf("unsupported config format for %s", path)
	}
}

func documentName(filename string) string {
	return strings.TrimSuffix(filename, filepath.Ext(filename))
}

func appendDocument(documents *[]Document, seen map[string]struct{}, document Document) error {
	if _, exists := seen[document.Name]; exists {
		return fmt.Errorf("duplicate config name %q", document.Name)
	}
	seen[document.Name] = struct{}{}
	*documents = append(*documents, document)
	return nil
}

func sortDocuments(documents []Document) {
	sort.Slice(documents, func(left, right int) bool {
		return documents[left].Name < documents[right].Name
	})
}
