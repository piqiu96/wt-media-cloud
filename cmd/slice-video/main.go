package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"syscall"

	"github.com/wt-media/wt-media-cloud/internal/bootstrap"
	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/client/platforms/douyin"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"github.com/wt-media/wt-media-cloud/internal/infra/storage"
	"github.com/wt-media/wt-media-cloud/internal/modules/slicelibrary/model"
	"github.com/wt-media/wt-media-cloud/internal/modules/slicelibrary/repository"
	"github.com/wt-media/wt-media-cloud/internal/modules/slicelibrary/service"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: slice-video prepare|publish|verify [flags]")
	}
	op := args[0]
	flags := flag.NewFlagSet(op, flag.ContinueOnError)
	root := flags.String("dir", "", "local run directory (prepare)")
	keyword := flags.String("keyword", "", "Douyin search keyword")
	source := flags.String("source", "", "Douyin video URL or ID")
	offset := flags.Int("offset", 0, "search offset")
	count := flags.Int("count", 1, "candidate count within configured budget")
	maxClips := flags.Int("max-clips", 0, "per-source clip limit, at most the configured maximum")
	candidate := flags.String("candidate", "", "local candidate.json path")
	decision := flags.String("decision", "", "model suitability decision JSON")
	report := flags.String("report", "", "local result JSON path")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if op != "prepare" && op != "publish" && op != "verify" {
		return errors.New("unknown slice-video operation")
	}
	if *report == "" {
		return errors.New("report path is required")
	}
	close, err := bootstrap.InitializeSliceTool()
	if err != nil {
		return err
	}
	defer close()
	cfg := config.Get()
	store, ok := storage.Get().(storage.ImmutableStore)
	if !ok {
		return storage.ErrNotConfigured
	}
	tool := service.Service{Config: cfg.Media.SliceLibrary, Tools: cfg.Media.FFmpeg, Store: store, Repository: repository.Repository{DB: database.DB()}}
	if *maxClips != 0 {
		if *maxClips < 1 || *maxClips > tool.Config.MaxClipsPerSource {
			return errors.New("max-clips exceeds configured source budget")
		}
		tool.Config.MaxClipsPerSource = *maxClips
	}
	if err = tool.Validate(); err != nil {
		return err
	}
	ctx, cancelSignal := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelSignal()
	ctx, cancel := context.WithTimeout(ctx, cfg.Media.SliceLibrary.Timeout.Duration)
	defer cancel()
	write := func(value any) error {
		data, e := json.MarshalIndent(value, "", "  ")
		if e != nil {
			return e
		}
		return os.WriteFile(*report, data, 0600)
	}
	switch op {
	case "prepare":
		if *root == "" || (*keyword == "") == (*source == "") || *offset < 0 || *count < 1 || *count > cfg.Media.SliceLibrary.MaxCandidates {
			return errors.New("prepare requires dir and exactly one of keyword/source, with bounded count/offset")
		}
		client := douyin.Get()
		if !client.Configured() {
			return errors.New("Douyin API credentials are not configured")
		}
		var items []map[string]any
		if *source != "" {
			payload, e := client.FetchByURL(ctx, douyin.FetchByURLRequest{URL: *source})
			if e != nil {
				return errors.New("Douyin detail request failed")
			}
			item, e := douyin.DetailItem(payload)
			if e != nil {
				return e
			}
			items = []map[string]any{item}
		} else {
			payload, e := client.Search(ctx, douyin.SearchRequest{Keyword: *keyword, Count: *count, Offset: *offset})
			if e != nil {
				return errors.New("Douyin search request failed")
			}
			items = douyin.SearchItems(payload)
		}
		type result struct {
			SourceID  string `json:"source_id"`
			Candidate string `json:"candidate,omitempty"`
			Error     string `json:"error,omitempty"`
		}
		results := []result{}
		seen := map[string]bool{}
		for _, item := range items {
			if len(results) >= *count {
				break
			}
			id, _ := item["aweme_id"].(string)
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			row := result{SourceID: id}
			// Resolve immediately before download rather than persisting search URLs.
			payload, e := client.FetchByURL(ctx, douyin.FetchByURLRequest{URL: id})
			if e != nil {
				row.Error = "detail lookup failed"
				results = append(results, row)
				continue
			}
			item, e = douyin.DetailItem(payload)
			if e != nil {
				row.Error = "missing detail"
				results = append(results, row)
				continue
			}
			address, e := douyin.MediaAddress(item)
			if e != nil {
				row.Error = "no usable media address"
				results = append(results, row)
				continue
			}
			title, _ := item["desc"].(string)
			directory, e := tool.Prepare(ctx, *root, id, title, address)
			if e != nil {
				row.Error = e.Error()
			} else {
				row.Candidate = filepath.Join(directory, "candidate.json")
			}
			results = append(results, row)
		}
		if err = write(results); err != nil {
			return err
		}
		fmt.Printf("prepared %d candidate results; report saved\n", len(results))
		if len(results) == 0 {
			return errors.New("no candidate videos returned")
		}
		return nil
	case "publish":
		if *candidate == "" || *decision == "" {
			return errors.New("candidate and decision paths are required")
		}
		data, e := os.ReadFile(*decision)
		if e != nil {
			return e
		}
		var review service.Decision
		if e = json.Unmarshal(data, &review); e != nil {
			return e
		}
		index, e := tool.Publish(ctx, filepath.Dir(*candidate), review)
		if e != nil {
			return e
		}
		if e = write(map[string]any{"suitable": review.Suitable, "source_id": review.SourceID, "library_count": len(index.Clips)}); e != nil {
			return e
		}
		fmt.Printf("processed source %s; suitable=%t; library clips=%d\n", review.SourceID, review.Suitable, len(index.Clips))
		return nil
	case "verify":
		ref, e := store.Reference(path.Join(cfg.Media.SliceLibrary.Prefix, "index.json"))
		if e != nil {
			return e
		}
		body, e := store.Open(ctx, ref)
		if e != nil {
			return e
		}
		defer body.Close()
		data, e := io.ReadAll(io.LimitReader(body, cfg.Media.SliceLibrary.MaxSourceBytes))
		if e != nil {
			return e
		}
		var index model.Index
		if e = json.Unmarshal(data, &index); e != nil {
			return e
		}
		if index.Version != 1 || len(index.Clips) == 0 {
			return errors.New("slice index is empty or unsupported")
		}
		for _, clip := range index.Clips {
			if e = service.VerifyClip(ctx, store, clip); e != nil {
				return e
			}
		}
		if e = write(index); e != nil {
			return e
		}
		fmt.Printf("verified %d library clips\n", len(index.Clips))
		return nil
	}
	return nil
}
