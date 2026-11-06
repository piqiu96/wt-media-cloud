// resource-cutover performs the explicitly authorized one-time old-data cleanup.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/wt-media/wt-media-cloud/internal/bootstrap"
	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/modules/production/repository"
	"os"
	"path/filepath"
)

func main() {
	apply := flag.Bool("apply", false, "delete inventoried old references and retire the three old templates")
	backup := flag.String("backup", "", "required new private local backup file (never overwritten)")
	flag.Parse()
	if *backup == "" {
		fmt.Fprintln(os.Stderr, "-backup is required")
		os.Exit(2)
	}
	config.MustInitializeRuntimePaths()
	close, e := bootstrap.InitializeMigration()
	if e != nil {
		panic(e)
	}
	defer close()
	counts, e := repository.RetireLegacyResources(context.Background(), *apply, func(b repository.CutoverBackup) error {
		f, e := os.OpenFile(*backup, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return e
		}
		defer f.Close()
		if e = json.NewEncoder(f).Encode(b); e != nil {
			return e
		}
		if e = f.Sync(); e != nil {
			return e
		}
		if e = f.Close(); e != nil {
			return e
		}
		dir, e := os.Open(filepath.Dir(*backup))
		if e != nil {
			return e
		}
		defer dir.Close()
		return dir.Sync()
	})
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
	json.NewEncoder(os.Stdout).Encode(struct {
		Apply bool           `json:"apply"`
		Rows  map[string]int `json:"rows"`
	}{*apply, counts})
}
