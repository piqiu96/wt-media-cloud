package deploy

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	cloudconfig "github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/infra/database/migration"
)

type DatabaseResult struct {
	Applied int
	Total   int
	Admin   string
}

func MigrateRelease(ctx context.Context, releaseRoot string) (DatabaseResult, error) {
	cfg, err := cloudconfig.LoadFromDir(filepath.Join(releaseRoot, "config"))
	if err != nil {
		return DatabaseResult{}, err
	}
	if err := database.Initialize(cfg.Databases); err != nil {
		return DatabaseResult{}, err
	}
	defer database.Close()
	migrations, err := migration.LoadDir(filepath.Join(releaseRoot, "migrations"))
	if err != nil {
		return DatabaseResult{}, err
	}
	applied, err := migration.Apply(ctx, database.DB(), migrations)
	if err != nil {
		return DatabaseResult{}, err
	}
	return DatabaseResult{Applied: len(applied), Total: len(migrations)}, nil
}

func VerifyDatabase(ctx context.Context, releaseRoot string, requireAdmin bool) (DatabaseResult, error) {
	cfg, err := cloudconfig.LoadFromDir(filepath.Join(releaseRoot, "config"))
	if err != nil {
		return DatabaseResult{}, err
	}
	if err := database.Initialize(cfg.Databases); err != nil {
		return DatabaseResult{}, err
	}
	defer database.Close()
	files, err := filepath.Glob(filepath.Join(releaseRoot, "migrations", "*.sql"))
	if err != nil {
		return DatabaseResult{}, err
	}
	var count int64
	if err := database.DB().WithContext(ctx).Raw("SELECT COUNT(*) FROM schema_migrations").Scan(&count).Error; err != nil {
		return DatabaseResult{}, fmt.Errorf("count schema_migrations: %w", err)
	}
	result := DatabaseResult{Total: len(files), Applied: int(count)}
	if int(count) != len(files) {
		return result, fmt.Errorf("schema_migrations count is %d, expected %d", count, len(files))
	}
	if requireAdmin {
		var username, role, status string
		err := database.DB().WithContext(ctx).Raw(
			"SELECT username, role, status FROM users WHERE username = ?",
			cfg.App.InitialAdmin.Username,
		).Row().Scan(&username, &role, &status)
		if err != nil {
			return result, fmt.Errorf("load initial admin: %w", err)
		}
		if role != "admin" || status != "enabled" {
			return result, fmt.Errorf("initial admin is %s/%s, want admin/enabled", role, status)
		}
		result.Admin = username
	}
	return result, nil
}

func DoctorDatabase(ctx context.Context, releaseRoot string) error {
	cfg, err := cloudconfig.LoadFromDir(filepath.Join(releaseRoot, "config"))
	if err != nil {
		return err
	}
	if err := database.Initialize(cfg.Databases); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	defer database.Close()
	var one int
	if err := database.DB().WithContext(ctx).Raw("SELECT 1").Scan(&one).Error; err != nil {
		return err
	}
	if one != 1 {
		return errors.New("database connectivity check returned an unexpected value")
	}
	return nil
}
