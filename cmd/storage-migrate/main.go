package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"

	"github.com/wt-media/wt-media-cloud/internal/bootstrap"
	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/infra/storage"
	storagerepository "github.com/wt-media/wt-media-cloud/internal/infra/storage/repository"
	transfers "github.com/wt-media/wt-media-cloud/internal/modules/filetransfer/repository"
	production "github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
)

type reportItem struct {
	Kind   string `json:"kind"`
	ID     string `json:"id"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
	storage.MigrationResult
}

func main() {
	inventory := flag.Bool("inventory", false, "list stored references without reading objects")
	apply := flag.Bool("apply", false, "copy verified objects and switch owner references")
	report := flag.String("report", "", "write a JSON report to this local file")
	flag.Parse()
	if err := run(*apply, *report, *inventory); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(apply bool, reportPath string, inventory bool) error {
	if apply && inventory {
		return errors.New("inventory and apply are mutually exclusive")
	}
	config.MustInitializeRuntimePaths()
	close, err := bootstrap.InitializeMigration()
	if err != nil {
		return err
	}
	defer close()
	cfg := config.Get()
	if err := storage.Initialize(cfg.Storage.ObjectStorage, cfg.Credentials.ObjectStorage, storagerepository.ContentClaims{DB: database.DB()}); err != nil {
		return err
	}
	defer storage.Close()
	store, ok := storage.Get().(storage.ImmutableStore)
	if !ok {
		return storage.ErrNotConfigured
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	assets, err := production.StoredAssets()
	if err != nil {
		return err
	}
	refs, err := transfers.StoredReferences()
	if err != nil {
		return err
	}
	items := make([]reportItem, 0, len(assets)+len(refs))
	failures := 0
	cache := map[string]storage.MigrationResult{}
	move := func(kind, id, source, key string, size int64, digest string, switchRef func(string) (bool, error)) {
		item := reportItem{Kind: kind, ID: id, Status: "verified"}
		if inventory {
			item.Status = "listed"
			item.MigrationResult = storage.MigrationResult{Location: source, Size: size, SHA256: digest}
			items = append(items, item)
			return
		}
		cacheKey := source + "\n" + key + "\n" + digest
		result, found := cache[cacheKey]
		var err error
		if !found {
			result, err = storage.MigrateVerified(ctx, store, source, key, size, digest, apply)
			if err == nil {
				cache[cacheKey] = result
			}
		}
		if err == nil && size > 0 && size != result.Size {
			err = errors.New("reference size conflicts with verified object")
		}
		if err == nil && apply && source != result.Location {
			var changed bool
			changed, err = switchRef(result.Location)
			if err == nil && !changed {
				err = errors.New("reference changed concurrently; rerun migration")
			}
			item.Status = "migrated"
		}
		item.MigrationResult = result
		if err != nil {
			item.Status = "failed"
			item.Error = err.Error()
			failures++
		}
		items = append(items, item)
		fmt.Fprintf(os.Stderr, "%s %s %s\n", item.Status, kind, id)
	}
	keyFor := func(kind string, id int64, source string) string {
		if kind == "composite_output" {
			return fmt.Sprintf("media-ku/finalVideo/%d.mp4", id)
		}
		if kind == "material" {
			extension := strings.ToLower(path.Ext(source))
			if extension == "" {
				extension = ".mp4"
			}
			return fmt.Sprintf("media-ku/material/%d%s", id, extension)
		}
		return ""
	}
	for _, asset := range assets {
		item := asset
		key := keyFor(item.Kind, item.ID, item.Location)
		move(item.Kind, fmt.Sprint(item.ID), item.Location, key, item.Size, item.SHA256, func(destination string) (bool, error) { return production.SwitchStoredAsset(item, destination) })
	}
	for _, ref := range refs {
		item := ref
		key := keyFor(item.AssetType, item.AssetID, item.Location)
		move("transfer", item.ID, item.Location, key, item.Size, item.SHA256, func(destination string) (bool, error) { return transfers.SwitchStoredReference(item, destination) })
	}
	payload, err := json.MarshalIndent(struct {
		Apply  bool         `json:"apply"`
		Failed int          `json:"failed"`
		Items  []reportItem `json:"items"`
	}{apply, failures, items}, "", "  ")
	if err != nil {
		return err
	}
	if reportPath != "" {
		if err := os.WriteFile(reportPath, payload, 0600); err != nil {
			return err
		}
	} else {
		fmt.Println(string(payload))
	}
	if failures > 0 {
		return fmt.Errorf("storage migration: %d of %d references failed; no failed reference was switched", failures, len(items))
	}
	return nil
}
