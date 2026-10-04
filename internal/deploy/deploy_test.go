package deploy

import (
	"context"
	"encoding/json"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
)

const sampleVariables = `WT_PRIMARY_DB_HOST = "127.0.0.1"
WT_PRIMARY_DB_PORT = 3306
WT_PRIMARY_DB_NAME = "wt_media_online"
WT_PRIMARY_DB_USERNAME = "wt_media_cloud"
WT_PRIMARY_DB_PASSWORD = "db-pass"
WT_AGENT_AUTH_TOKEN = ""
WT_DOUYIN_API_KEY = "douyin-key"
WT_DOUYIN_COOKIE = "douyin-cookie"
WT_OBJECT_STORAGE_PREFIX = "wt_media/online/"
WT_OBJECT_STORAGE_ACCESS_KEY = "access-key"
WT_OBJECT_STORAGE_SECRET_KEY = "secret-key"
`

func TestSchemaMatchesRepositoryTemplates(t *testing.T) {
	root := filepath.Join("..", "..")
	schema, err := LoadVariableSchema(filepath.Join(root, "deploy", "config-variable-schema.toml"))
	if err != nil {
		t.Fatal(err)
	}
	names, err := TemplateVariableNames(filepath.Join(root, "config_online"))
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifySchemaMatchesTemplates(schema, names); err != nil {
		t.Fatal(err)
	}
}

func TestCheckVariablesRejectsMissingAndPlaceholder(t *testing.T) {
	schema, err := LoadVariableSchema(filepath.Join("..", "..", "deploy", "config-variable-schema.toml"))
	if err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"missing":     strings.Replace(sampleVariables, "WT_PRIMARY_DB_PASSWORD = \"db-pass\"\n", "", 1),
		"placeholder": strings.Replace(sampleVariables, "db-pass", "<REPLACE_ME>", 1),
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "online.toml")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, _, err := CheckVariables(path, schema); err == nil {
				t.Fatal("CheckVariables() error = nil, want refusal")
			}
		})
	}
}

func TestRenderConfig(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	if err := copyTree(filepath.Join("..", "..", "config_online"), configDir); err != nil {
		t.Fatal(err)
	}
	variables := filepath.Join(root, "online.toml")
	if err := os.WriteFile(variables, []byte(sampleVariables), 0o600); err != nil {
		t.Fatal(err)
	}
	schema, err := LoadVariableSchema(filepath.Join("..", "..", "deploy", "config-variable-schema.toml"))
	if err != nil {
		t.Fatal(err)
	}
	digest, err := RenderConfig(configDir, variables, "online", schema)
	if err != nil {
		t.Fatal(err)
	}
	if len(digest) != 64 {
		t.Fatalf("digest = %q", digest)
	}
	if _, err := os.Stat(filepath.Join(configDir, "database", "primary.toml")); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(configDir, ".render-info.json"))
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(raw, &record); err != nil {
		t.Fatal(err)
	}
	if record["environment"] != "online" {
		t.Fatalf("render record = %#v", record)
	}
}

func TestPullVariablesWithHTTPS(t *testing.T) {
	schema, err := LoadVariableSchema(filepath.Join("..", "..", "deploy", "config-variable-schema.toml"))
	if err != nil {
		t.Fatal(err)
	}
	server := newTLSVariablesServer(t, sampleVariables)
	defer server.Close()
	target := filepath.Join(t.TempDir(), "online.toml")
	digest, count, err := pullVariablesWithClient(context.Background(), server.URL, target, schema, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	if len(digest) != 64 || count != 11 {
		t.Fatalf("digest=%q count=%d", digest, count)
	}
	mode, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if mode.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o", mode.Mode().Perm())
	}
}

func TestInstallAndActivateRelease(t *testing.T) {
	source := makePackageFixture(t)
	current, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	installRoot := t.TempDir()
	profile := Profile{SchemaVersion: 1, Deploy: ProfileDeploy{
		InstallRoot:    installRoot,
		OutputDir:      filepath.Join(installRoot, "output"),
		PackageRoot:    source,
		Release:        "v0.1.0-rc.1",
		Environment:    "online",
		ServiceUser:    current.Username,
		ProcessManager: "baota",
		VariablesFile:  filepath.Join(installRoot, "output", "online.toml"),
	}}
	if err := InstallRelease(profile, source); err != nil {
		t.Fatal(err)
	}
	releaseRoot := profile.ReleaseRoot()
	if err := os.MkdirAll(filepath.Join(releaseRoot, "config", "database"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(releaseRoot, "config", "database", "primary.toml"), []byte("name='primary'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(releaseRoot, "config", ".render-info.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ActivateRelease(profile, profile.Deploy.Release); err != nil {
		t.Fatal(err)
	}
	target, err := os.Readlink(filepath.Join(installRoot, "current"))
	if err != nil {
		t.Fatal(err)
	}
	if target != filepath.Join("releases", profile.Deploy.Release) {
		t.Fatalf("current -> %s", target)
	}
}

func makePackageFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(relative, content string, mode os.FileMode) {
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), mode); err != nil {
			t.Fatal(err)
		}
	}
	for _, binary := range []string{"server", "discovery-scheduler", "discovery-worker", "migrate", "config-check", "wtmctl", "ffmpeg", "ffprobe"} {
		write("bin/"+binary, "#!/bin/sh\n", 0o755)
	}
	write("web/index.cloud.html", "<html></html>\n", 0o644)
	for _, fixed := range []string{"config/app.toml", "config/clients/http/agent.toml", "config/clients/http/douyin.toml", "deploy/DEPLOYMENT.md", "deploy/config-variable-schema.toml", "deploy/prepare-database.sql.example", "deploy/examples/online.toml.example", "deploy/examples/online-deploy.toml.example"} {
		write(fixed, "# test\n", 0o644)
	}
	write("config/database/primary.toml.tpl", "password = {{WT_PRIMARY_DB_PASSWORD}}\n", 0o644)
	write("config/credentials/agent.toml.tpl", "auth_token = {{WT_AGENT_AUTH_TOKEN}}\n", 0o644)
	write("config/credentials/douyin.toml.tpl", "api_key = {{WT_DOUYIN_API_KEY}}\n", 0o644)
	write("config/credentials/object_storage.toml.tpl", "access_key = {{WT_OBJECT_STORAGE_ACCESS_KEY}}\n", 0o644)
	write("config/storage/object_storage.toml.tpl", "prefix = {{WT_OBJECT_STORAGE_PREFIX}}\n", 0o644)
	write("migrations/001_identity.sql", "CREATE TABLE users (id INT);\n", 0o644)
	write("deploy/config-variable-schema.toml", `schema_version = 1
[[variables]]
name = "WT_PRIMARY_DB_PASSWORD"
type = "string"
required = true
secret = true
[[variables]]
name = "WT_AGENT_AUTH_TOKEN"
type = "string"
required = false
secret = true
allow_empty = true
[[variables]]
name = "WT_DOUYIN_API_KEY"
type = "string"
required = true
secret = true
[[variables]]
name = "WT_OBJECT_STORAGE_ACCESS_KEY"
type = "string"
required = true
secret = true
[[variables]]
name = "WT_OBJECT_STORAGE_PREFIX"
type = "string"
required = true
secret = false
`, 0o644)
	write("release-info.json", `{"schema_version":1,"product_tag":"v0.1.0-rc.1","source_commit":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","configuration":"template-state config/ rendered by wtmctl"}`+"\n", 0o644)
	return root
}
