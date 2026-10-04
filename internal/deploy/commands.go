package deploy

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Runner struct {
	Stdout io.Writer
	Stderr io.Writer
}

func (r Runner) Run(ctx context.Context, args []string) int {
	if len(args) == 0 {
		r.usage()
		return 2
	}
	var err error
	switch args[0] {
	case "version":
		_, err = fmt.Fprintln(r.Stdout, "wtmctl dev")
	case "doctor":
		err = r.doctor(ctx, args[1:])
	case "vars":
		err = r.vars(ctx, args[1:])
	case "artifact":
		err = r.artifact(args[1:])
	case "config":
		err = r.config(args[1:])
	case "db":
		err = r.db(ctx, args[1:])
	case "deploy":
		err = r.deploy(ctx, args[1:])
	case "release":
		err = r.release(args[1:])
	case "service":
		err = r.service(args[1:])
	case "status":
		err = r.status(ctx, args[1:])
	case "help", "-h", "--help":
		r.usage()
	default:
		err = fmt.Errorf("unknown command: %s", args[0])
	}
	if err != nil {
		fmt.Fprintf(r.Stderr, "wtmctl: %v\n", err)
		return 1
	}
	return 0
}

func (r Runner) usage() {
	fmt.Fprintln(r.Stdout, `wtmctl commands:
  version
  doctor --profile <file>
  vars pull --profile <file> [--url <url>|--url-file <file>] [--output <file>] [--check]
  vars check --file <file> --schema <file>
  artifact verify --profile <file> [--package-root <dir>]
  config render --profile <file> [--variables-file <file>] [--root <release-root>]
  db migrate --profile <file>
  db verify --profile <file> [--require-admin=false]
  deploy plan --profile <file> [--package-root <dir>]
  deploy apply --profile <file> [--package-root <dir>]
  deploy verify --profile <file>
  release rollback --profile <file> --to <tag> --services-stopped
  service check --profile <file>
  status --profile <file>

wtmctl never starts, stops, or restarts Cloud processes. BaoTa manages those.`)
}

type commonFlags struct {
	profile     string
	packageRoot string
}

func addCommon(fs *flag.FlagSet, common *commonFlags) {
	fs.StringVar(&common.profile, "profile", "", "deployment profile TOML")
	fs.StringVar(&common.packageRoot, "package-root", "", "extracted package root")
}

func loadProfile(common commonFlags) (Profile, error) {
	if strings.TrimSpace(common.profile) == "" {
		return Profile{}, errors.New("--profile is required")
	}
	return LoadProfile(common.profile)
}

// resolvePackageRelease fills deploy.release from the extracted package's
// release-info.json when the profile omits it, so operators never edit a
// version-specific value. A configured release must still match the package.
func resolvePackageRelease(profile Profile, packageRoot string) (Profile, error) {
	info, err := ReadReleaseInfo(packageRoot)
	if err != nil {
		return profile, err
	}
	if strings.TrimSpace(profile.Deploy.Release) == "" {
		profile.Deploy.Release = info.ProductTag
		return profile, nil
	}
	if profile.Deploy.Release != info.ProductTag {
		return profile, errors.New("release-info product tag differs from deployment profile")
	}
	return profile, nil
}

// resolveInstalledRelease fills deploy.release from the `current` symlink when
// the profile omits it, for commands that operate on an installed release.
func resolveInstalledRelease(profile Profile) (Profile, error) {
	if strings.TrimSpace(profile.Deploy.Release) != "" {
		return profile, nil
	}
	current, err := CurrentRelease(profile)
	if err != nil {
		return profile, errors.New("deploy.release is required when current is unavailable")
	}
	profile.Deploy.Release = current
	return profile, nil
}

func resolveSchemaPath(profile Profile, packageRoot, explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if packageRoot == "" {
		var err error
		packageRoot, err = ResolvePackageRoot(profile, "")
		if err != nil {
			return "", err
		}
	}
	return filepath.Join(packageRoot, "deploy", "config-variable-schema.toml"), nil
}

func resolveVariablesSource(profile Profile, urlValue, urlFile string) (string, bool, error) {
	if urlValue != "" {
		return urlValue, true, nil
	}
	if urlFile != "" {
		raw, err := os.ReadFile(urlFile)
		if err != nil {
			return "", false, err
		}
		return strings.TrimSpace(string(raw)), true, nil
	}
	if profile.Deploy.VariablesURL != "" {
		return profile.Deploy.VariablesURL, true, nil
	}
	if profile.Deploy.VariablesURLFile != "" {
		raw, err := os.ReadFile(profile.Deploy.VariablesURLFile)
		if err != nil {
			return "", false, err
		}
		return strings.TrimSpace(string(raw)), true, nil
	}
	return "", false, nil
}

func (r Runner) vars(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("vars subcommand is required")
	}
	switch args[0] {
	case "pull":
		fs := flag.NewFlagSet("vars pull", flag.ContinueOnError)
		fs.SetOutput(r.Stderr)
		var common commonFlags
		addCommon(fs, &common)
		urlValue := fs.String("url", "", "remote TOML URL")
		urlFile := fs.String("url-file", "", "file containing remote TOML URL")
		output := fs.String("output", "", "output TOML path")
		schemaPath := fs.String("schema", "", "variable schema")
		check := fs.Bool("check", false, "check after download")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		profile, err := loadProfile(common)
		if err != nil {
			return err
		}
		source, ok, err := resolveVariablesSource(profile, *urlValue, *urlFile)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("variables URL source is required")
		}
		if *output == "" {
			*output = profile.Deploy.VariablesFile
		}
		schemaFile, err := resolveSchemaPath(profile, common.packageRoot, *schemaPath)
		if err != nil {
			return err
		}
		schema, err := LoadVariableSchema(schemaFile)
		if err != nil {
			return err
		}
		digest, count, err := PullVariables(ctx, source, *output, schema)
		if err != nil {
			return err
		}
		fmt.Fprintf(r.Stdout, "environment=%s\nvariables_file=%s\nvariables_sha256=%s\nkeys=%d\n", profile.Deploy.Environment, *output, digest, count)
		if *check {
			if _, _, err := CheckVariables(*output, schema); err != nil {
				return err
			}
		}
		return nil
	case "check":
		fs := flag.NewFlagSet("vars check", flag.ContinueOnError)
		fs.SetOutput(r.Stderr)
		file := fs.String("file", "", "TOML variables file")
		schemaPath := fs.String("schema", "", "variable schema")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *file == "" || *schemaPath == "" {
			return errors.New("--file and --schema are required")
		}
		schema, err := LoadVariableSchema(*schemaPath)
		if err != nil {
			return err
		}
		digest, count, err := CheckVariables(*file, schema)
		if err != nil {
			return err
		}
		fmt.Fprintf(r.Stdout, "variables_file=%s\nvariables_sha256=%s\nkeys=%d\n", *file, digest, count)
		return nil
	default:
		return fmt.Errorf("unknown vars subcommand: %s", args[0])
	}
}

func (r Runner) artifact(args []string) error {
	if len(args) == 0 || args[0] != "verify" {
		return errors.New("artifact verify is required")
	}
	fs := flag.NewFlagSet("artifact verify", flag.ContinueOnError)
	fs.SetOutput(r.Stderr)
	var common commonFlags
	addCommon(fs, &common)
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	profile, err := loadProfile(common)
	if err != nil {
		return err
	}
	root, err := ResolvePackageRoot(profile, common.packageRoot)
	if err != nil {
		return err
	}
	profile, err = resolvePackageRelease(profile, root)
	if err != nil {
		return err
	}
	info, err := VerifyPackage(root, profile.Deploy.Release)
	if err != nil {
		return err
	}
	fmt.Fprintf(r.Stdout, "package_ok=%s\nproduct_tag=%s\nsource_commit=%s\n", root, info.ProductTag, info.SourceCommit)
	return nil
}

func (r Runner) config(args []string) error {
	if len(args) == 0 {
		return errors.New("config subcommand is required")
	}
	fs := flag.NewFlagSet("config "+args[0], flag.ContinueOnError)
	fs.SetOutput(r.Stderr)
	var common commonFlags
	addCommon(fs, &common)
	variablesFile := fs.String("variables-file", "", "variables TOML")
	root := fs.String("root", "", "release root")
	schemaPath := fs.String("schema", "", "variable schema")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	profile, err := loadProfile(common)
	if err != nil {
		return err
	}
	profile, err = resolveInstalledRelease(profile)
	if err != nil {
		return err
	}
	if *root == "" {
		*root = profile.ReleaseRoot()
	}
	packageRoot, _ := ResolvePackageRoot(profile, common.packageRoot)
	schemaFile, err := resolveSchemaPath(profile, packageRoot, *schemaPath)
	if err != nil {
		return err
	}
	schema, err := LoadVariableSchema(schemaFile)
	if err != nil {
		return err
	}
	if *variablesFile == "" {
		*variablesFile = profile.Deploy.VariablesFile
	}
	switch args[0] {
	case "render":
		digest, err := RenderConfig(filepath.Join(*root, "config"), *variablesFile, profile.Deploy.Environment, schema)
		if err != nil {
			return err
		}
		fmt.Fprintf(r.Stdout, "configuration_rendered=%s\nvariables_sha256=%s\n", filepath.Join(*root, "config"), digest)
		return nil
	case "check":
		cfgDir := filepath.Join(*root, "config")
		if _, err := os.Stat(filepath.Join(cfgDir, "database", "primary.toml")); err != nil {
			return errors.New("rendered config is missing")
		}
		fmt.Fprintf(r.Stdout, "configuration_checked=%s\n", cfgDir)
		return nil
	default:
		return fmt.Errorf("unknown config subcommand: %s", args[0])
	}
}

func (r Runner) db(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("db subcommand is required")
	}
	fs := flag.NewFlagSet("db "+args[0], flag.ContinueOnError)
	fs.SetOutput(r.Stderr)
	var common commonFlags
	addCommon(fs, &common)
	requireAdmin := fs.Bool("require-admin", true, "require enabled admin")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	profile, err := loadProfile(common)
	if err != nil {
		return err
	}
	profile, err = resolveInstalledRelease(profile)
	if err != nil {
		return err
	}
	root := profile.ReleaseRoot()
	switch args[0] {
	case "migrate":
		result, err := MigrateRelease(ctx, root)
		if err != nil {
			return err
		}
		fmt.Fprintf(r.Stdout, "migration_ok_applied=%d\ntotal=%d\n", result.Applied, result.Total)
		return nil
	case "verify":
		result, err := VerifyDatabase(ctx, root, *requireAdmin)
		if err != nil {
			return err
		}
		fmt.Fprintf(r.Stdout, "migrations=%d\ntotal=%d\n", result.Applied, result.Total)
		if result.Admin != "" {
			fmt.Fprintf(r.Stdout, "initial_admin=%s/enabled\n", result.Admin)
		}
		return nil
	default:
		return fmt.Errorf("unknown db subcommand: %s", args[0])
	}
}

func (r Runner) deploy(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("deploy subcommand is required")
	}
	fs := flag.NewFlagSet("deploy "+args[0], flag.ContinueOnError)
	fs.SetOutput(r.Stderr)
	var common commonFlags
	addCommon(fs, &common)
	variablesFile := fs.String("variables-file", "", "offline variables TOML")
	variablesURL := fs.String("variables-url", "", "remote variables URL")
	variablesURLFile := fs.String("url-file", "", "file containing variables URL")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	profile, err := loadProfile(common)
	if err != nil {
		return err
	}
	switch args[0] {
	case "plan", "apply":
		return r.deployPlanOrApply(ctx, profile, common, *variablesFile, *variablesURL, *variablesURLFile, args[0] == "apply")
	case "verify":
		profile, err = resolveInstalledRelease(profile)
		if err != nil {
			return err
		}
		result, err := VerifyDatabase(ctx, profile.ReleaseRoot(), true)
		if err != nil {
			return err
		}
		if err := VerifyRuntime(ctx, profile, profile.ReleaseRoot()); err != nil {
			return err
		}
		processes, err := CheckServiceProcesses(profile)
		if err != nil {
			return err
		}
		for name, running := range processes {
			if !running {
				return fmt.Errorf("%s process is not running", name)
			}
		}
		fmt.Fprintf(r.Stdout, "current=%s\nmigrations=%d\ninitial_admin=%s/enabled\nruntime=ok\nprocesses=server,worker,scheduler\n", profile.Deploy.Release, result.Applied, result.Admin)
		return nil
	default:
		return fmt.Errorf("unknown deploy subcommand: %s", args[0])
	}
}

func (r Runner) deployPlanOrApply(ctx context.Context, profile Profile, common commonFlags, variablesFile, variablesURL, variablesURLFile string, apply bool) error {
	packageRoot, err := ResolvePackageRoot(profile, common.packageRoot)
	if err != nil {
		return err
	}
	profile, err = resolvePackageRelease(profile, packageRoot)
	if err != nil {
		return err
	}
	if _, err := VerifyPackage(packageRoot, profile.Deploy.Release); err != nil {
		return err
	}
	schema, err := LoadVariableSchema(filepath.Join(packageRoot, "deploy", "config-variable-schema.toml"))
	if err != nil {
		return err
	}
	targetVariables := variablesFile
	if targetVariables == "" {
		targetVariables = profile.Deploy.VariablesFile
	}
	source, hasURL, err := resolveVariablesSource(profile, variablesURL, variablesURLFile)
	if err != nil {
		return err
	}
	variablesForPlan := targetVariables
	cleanup := func() {}
	if apply {
		if hasURL {
			if _, _, err := PullVariables(ctx, source, targetVariables, schema); err != nil {
				return err
			}
		}
		if _, _, err := CheckVariables(targetVariables, schema); err != nil {
			return err
		}
		if err := InstallRelease(profile, packageRoot); err != nil {
			return err
		}
		if _, err := RenderConfig(filepath.Join(profile.ReleaseRoot(), "config"), targetVariables, profile.Deploy.Environment, schema); err != nil {
			return err
		}
		result, err := MigrateRelease(ctx, profile.ReleaseRoot())
		if err != nil {
			return err
		}
		if _, err := VerifyDatabase(ctx, profile.ReleaseRoot(), false); err != nil {
			return err
		}
		if err := ActivateRelease(profile, profile.Deploy.Release); err != nil {
			return err
		}
		fmt.Fprintf(r.Stdout, "deployed=%s\ncurrent=%s\nmigrations_applied=%d\ntotal=%d\nservices_restart_required=server,worker,scheduler\n", profile.Deploy.Release, profile.Deploy.Release, result.Applied, result.Total)
		return nil
	}
	tempDir, err := os.MkdirTemp(profile.Deploy.OutputDir, ".wtmctl-plan-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)
	cleanup = func() { _ = os.RemoveAll(tempDir) }
	defer cleanup()
	if hasURL {
		variablesForPlan = filepath.Join(tempDir, profile.Deploy.Environment+".toml")
		if _, _, err := PullVariables(ctx, source, variablesForPlan, schema); err != nil {
			return err
		}
	}
	digest, count, err := CheckVariables(variablesForPlan, schema)
	if err != nil {
		return err
	}
	templateCopy := filepath.Join(tempDir, "config")
	if err := copyTree(filepath.Join(packageRoot, "config"), templateCopy); err != nil {
		return err
	}
	if _, err := RenderConfig(templateCopy, variablesForPlan, profile.Deploy.Environment, schema); err != nil {
		return err
	}
	fmt.Fprintf(r.Stdout, "release=%s\nenvironment=%s\ninstall_root=%s\noutput_dir=%s\nvariables_sha256=%s\nvariables_keys=%d\npackage_root=%s\nservices_restart_required=server,worker,scheduler\n", profile.Deploy.Release, profile.Deploy.Environment, profile.Deploy.InstallRoot, profile.Deploy.OutputDir, digest, count, packageRoot)
	return nil
}

func (r Runner) release(args []string) error {
	if len(args) == 0 || args[0] != "rollback" {
		return errors.New("release rollback is required")
	}
	fs := flag.NewFlagSet("release rollback", flag.ContinueOnError)
	fs.SetOutput(r.Stderr)
	var common commonFlags
	addCommon(fs, &common)
	to := fs.String("to", "", "release tag")
	stopped := fs.Bool("services-stopped", false, "confirm BaoTa processes are stopped")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if !*stopped {
		return errors.New("--services-stopped is required; wtmctl does not stop BaoTa processes")
	}
	profile, err := loadProfile(common)
	if err != nil {
		return err
	}
	if *to == "" {
		return errors.New("--to is required")
	}
	if err := ActivateRelease(profile, *to); err != nil {
		return err
	}
	fmt.Fprintf(r.Stdout, "current=%s\nservices_start_required=true\n", *to)
	return nil
}

func (r Runner) service(args []string) error {
	if len(args) == 0 || args[0] != "check" {
		return errors.New("service check is required")
	}
	fs := flag.NewFlagSet("service check", flag.ContinueOnError)
	fs.SetOutput(r.Stderr)
	var common commonFlags
	addCommon(fs, &common)
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	profile, err := loadProfile(common)
	if err != nil {
		return err
	}
	processes, err := CheckServiceProcesses(profile)
	if err != nil {
		return err
	}
	raw, _ := json.Marshal(processes)
	fmt.Fprintln(r.Stdout, string(raw))
	for _, running := range processes {
		if !running {
			return errors.New("one or more BaoTa-managed processes are not running")
		}
	}
	return nil
}

func (r Runner) status(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(r.Stderr)
	var common commonFlags
	addCommon(fs, &common)
	if err := fs.Parse(args); err != nil {
		return err
	}
	profile, err := loadProfile(common)
	if err != nil {
		return err
	}
	if resolved, resolveErr := resolveInstalledRelease(profile); resolveErr == nil {
		profile = resolved
	}
	current, currentErr := CurrentRelease(profile)
	fmt.Fprintf(r.Stdout, "profile_release=%s\nenvironment=%s\ninstall_root=%s\noutput_dir=%s\n", profile.Deploy.Release, profile.Deploy.Environment, profile.Deploy.InstallRoot, profile.Deploy.OutputDir)
	if currentErr != nil {
		fmt.Fprintf(r.Stdout, "current=missing\n")
	} else {
		fmt.Fprintf(r.Stdout, "current=%s\n", current)
	}
	processes, err := CheckServiceProcesses(profile)
	if err == nil {
		raw, _ := json.Marshal(processes)
		fmt.Fprintf(r.Stdout, "processes=%s\n", raw)
	}
	if _, err := os.Stat(filepath.Join(profile.ReleaseRoot(), "config", ".render-info.json")); err == nil {
		result, err := VerifyDatabase(ctx, profile.ReleaseRoot(), false)
		if err == nil {
			fmt.Fprintf(r.Stdout, "migrations=%d/%d\n", result.Applied, result.Total)
		}
	}
	return nil
}

func (r Runner) doctor(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(r.Stderr)
	var common commonFlags
	addCommon(fs, &common)
	if err := fs.Parse(args); err != nil {
		return err
	}
	profile, err := loadProfile(common)
	if err != nil {
		return err
	}
	if err := EnsureServiceUser(profile.Deploy.ServiceUser); err != nil {
		return err
	}
	if err := ensureWritableDirectory(profile.Deploy.InstallerOutputDir()); err != nil {
		return err
	}
	if err := os.MkdirAll(profile.Deploy.InstallRoot, 0o755); err != nil {
		return err
	}
	root, err := ResolvePackageRoot(profile, common.packageRoot)
	if err != nil {
		return err
	}
	profile, err = resolvePackageRelease(profile, root)
	if err != nil {
		return err
	}
	if _, err := VerifyPackage(root, profile.Deploy.Release); err != nil {
		return err
	}
	fmt.Fprintf(r.Stdout, "doctor=ok\nos=%s\ninstall_root=%s\noutput_dir=%s\nrelease=%s\nprocess_manager=baota\n", "linux/amd64", profile.Deploy.InstallRoot, profile.Deploy.OutputDir, profile.Deploy.Release)
	return nil
}

func (d ProfileDeploy) InstallerOutputDir() string {
	if d.OutputDir != "" {
		return d.OutputDir
	}
	return filepath.Join(d.InstallRoot, "output")
}
