package deploy

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var commitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

type ReleaseInfo struct {
	SchemaVersion int    `json:"schema_version"`
	ProductTag    string `json:"product_tag"`
	SourceCommit  string `json:"source_commit"`
	Configuration string `json:"configuration"`
}

// ReadReleaseInfo reads the package identity from an extracted Cloud package.
func ReadReleaseInfo(root string) (ReleaseInfo, error) {
	raw, err := os.ReadFile(filepath.Join(root, "release-info.json"))
	if err != nil {
		return ReleaseInfo{}, fmt.Errorf("read release-info.json: %w", err)
	}
	var info ReleaseInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		return ReleaseInfo{}, fmt.Errorf("parse release-info.json: %w", err)
	}
	if info.SchemaVersion != 1 {
		return ReleaseInfo{}, errors.New("release-info schema_version must be 1")
	}
	if strings.TrimSpace(info.ProductTag) == "" {
		return ReleaseInfo{}, errors.New("release-info product_tag is required")
	}
	return info, nil
}

func VerifyPackage(root, release string) (ReleaseInfo, error) {
	info, err := ReadReleaseInfo(root)
	if err != nil {
		return ReleaseInfo{}, err
	}
	if release != "" && info.ProductTag != release {
		return ReleaseInfo{}, errors.New("release-info product tag differs from deployment profile")
	}
	if !commitPattern.MatchString(info.SourceCommit) {
		return ReleaseInfo{}, errors.New("release-info source_commit must be a full Git SHA-1")
	}
	if info.Configuration != "template-state config/ rendered by wtmctl" {
		return ReleaseInfo{}, errors.New("release-info configuration marker differs from wtmctl")
	}
	required := []string{
		"bin/wt-media-cloud", "bin/discovery-scheduler", "bin/discovery-worker", "bin/migrate", "bin/config-check", "bin/wtmctl", "bin/ffmpeg", "bin/ffprobe",
		"web/index.cloud.html",
		"config/app.toml",
		"config/clients/http/agent.toml",
		"config/clients/http/douyin.toml",
		"config/database/primary.toml.tpl",
		"config/credentials/agent.toml.tpl",
		"config/credentials/douyin.toml.tpl",
		"config/credentials/object_storage.toml.tpl",
		"config/storage/object_storage.toml.tpl",
		"deploy/DEPLOYMENT.md",
		"deploy/config-variable-schema.toml",
		"deploy/prepare-database.sql.example",
		"deploy/examples/online.toml.example",
		"deploy/examples/online-deploy.toml.example",
	}
	for _, relative := range required {
		path := filepath.Join(root, filepath.FromSlash(relative))
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return ReleaseInfo{}, fmt.Errorf("package omits %s", relative)
		}
		if strings.HasPrefix(relative, "bin/") && info.Mode()&0o111 == 0 {
			return ReleaseInfo{}, fmt.Errorf("package binary is not executable: %s", relative)
		}
	}
	migrations, err := filepath.Glob(filepath.Join(root, "migrations", "*.sql"))
	if err != nil || len(migrations) == 0 {
		return ReleaseInfo{}, errors.New("package contains no SQL migrations")
	}
	forbidden := []string{
		"config_online", "config_test", "config/.render-info.json", "deploy/render-config.py",
		"config/database/primary.toml", "config/credentials/agent.toml", "config/credentials/douyin.toml",
		"config/credentials/object_storage.toml", "config/storage/object_storage.toml",
	}
	for _, relative := range forbidden {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(relative))); err == nil {
			return ReleaseInfo{}, fmt.Errorf("package contains forbidden path: %s", relative)
		}
	}
	schema, err := LoadVariableSchema(filepath.Join(root, "deploy", "config-variable-schema.toml"))
	if err != nil {
		return ReleaseInfo{}, err
	}
	templates, err := TemplateVariableNames(filepath.Join(root, "config"))
	if err != nil {
		return ReleaseInfo{}, err
	}
	if err := VerifySchemaMatchesTemplates(schema, templates); err != nil {
		return ReleaseInfo{}, err
	}
	return info, nil
}
