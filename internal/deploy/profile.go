// Package deploy implements the wtmctl deployment control surface.
package deploy

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

type Profile struct {
	SchemaVersion int           `toml:"schema_version"`
	Deploy        ProfileDeploy `toml:"deploy"`
}

type ProfileDeploy struct {
	InstallRoot      string `toml:"install_root"`
	OutputDir        string `toml:"output_dir"`
	PackageRoot      string `toml:"package_root"`
	ArtifactFile     string `toml:"artifact_file"`
	Release          string `toml:"release"`
	Environment      string `toml:"environment"`
	ServiceUser      string `toml:"service_user"`
	VariablesURL     string `toml:"variables_url"`
	VariablesURLFile string `toml:"variables_url_file"`
	VariablesFile    string `toml:"variables_file"`
	HealthURL        string `toml:"health_url"`
	ProcessManager   string `toml:"process_manager"`
}

func LoadProfile(path string) (Profile, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Profile{}, fmt.Errorf("read profile: %w", err)
	}
	var profile Profile
	if err := toml.Unmarshal(raw, &profile); err != nil {
		return Profile{}, fmt.Errorf("parse profile: %w", err)
	}
	if err := profile.Validate(); err != nil {
		return Profile{}, err
	}
	return profile, nil
}

func (p *Profile) Validate() error {
	if p.SchemaVersion != 1 {
		return errors.New("profile schema_version must be 1")
	}
	if !filepath.IsAbs(p.Deploy.InstallRoot) {
		return errors.New("deploy.install_root must be an absolute path")
	}
	if p.Deploy.OutputDir == "" {
		p.Deploy.OutputDir = filepath.Join(p.Deploy.InstallRoot, "output")
	}
	if !filepath.IsAbs(p.Deploy.OutputDir) {
		return errors.New("deploy.output_dir must be an absolute path")
	}
	if strings.TrimSpace(p.Deploy.Release) == "" {
		return errors.New("deploy.release is required")
	}
	if p.Deploy.Environment != "pre" && p.Deploy.Environment != "online" {
		return errors.New("deploy.environment must be pre or online")
	}
	if strings.TrimSpace(p.Deploy.ServiceUser) == "" {
		return errors.New("deploy.service_user is required")
	}
	if p.Deploy.ProcessManager == "" {
		p.Deploy.ProcessManager = "baota"
	}
	if p.Deploy.ProcessManager != "baota" {
		return errors.New("only BaoTa process management is supported")
	}
	if p.Deploy.VariablesFile == "" {
		p.Deploy.VariablesFile = filepath.Join(p.Deploy.OutputDir, p.Deploy.Environment+".toml")
	}
	if !filepath.IsAbs(p.Deploy.VariablesFile) {
		return errors.New("deploy.variables_file must be an absolute path")
	}
	if p.Deploy.VariablesURL != "" && !strings.HasPrefix(p.Deploy.VariablesURL, "https://") {
		return errors.New("deploy.variables_url must use https://")
	}
	return nil
}

func (p Profile) ReleaseRoot() string {
	return filepath.Join(p.Deploy.InstallRoot, "releases", p.Deploy.Release)
}

func ResolvePackageRoot(profile Profile, explicit string) (string, error) {
	candidates := []string{explicit, profile.Deploy.PackageRoot}
	if executable, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Dir(filepath.Dir(executable)))
	}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		absolute, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(absolute, "release-info.json")); err == nil {
			return absolute, nil
		}
	}
	return "", errors.New("package root is not configured and wtmctl is not running from an extracted package")
}
