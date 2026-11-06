package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/wt-media/wt-media-cloud/internal/config"
	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	storagerepository "github.com/wt-media/wt-media-cloud/internal/infra/storage/repository"
)

func TestLivePrimaryStoreImmutableRoundTrip(t *testing.T) {
	if os.Getenv("WT_MEDIA_STORAGE_LIVE") != "1" {
		t.Skip("set WT_MEDIA_STORAGE_LIVE=1 to verify the configured primary bucket")
	}
	// Resolve the real Cloud configuration without logging credential contents.
	cfg, err := config.LoadFromDir("../../../config")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Initialize(cfg.Databases); err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	store, err := newMinioStore(cfg.Storage.ObjectStorage, cfg.Credentials.ObjectStorage)
	if err != nil {
		t.Fatal(err)
	}
	store.claims = storagerepository.ContentClaims{DB: database.DB()}
	var token [12]byte
	if _, err := rand.Read(token[:]); err != nil {
		t.Fatal(err)
	}
	key, err := store.Reference("file-ku/files/0_storage-check-" + hex.EncodeToString(token[:]) + ".bin")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	defer func() {
		if err := store.Remove(ctx, key); err != nil {
			t.Error("verification object cleanup failed", err)
		}
	}()
	for range 2 {
		if err := store.PutImmutable(ctx, key, strings.NewReader("video-A"), 7, "application/octet-stream", DigestOf([]byte("video-A"))); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.PutImmutable(ctx, key, strings.NewReader("video-B"), 7, "application/octet-stream", DigestOf([]byte("video-B"))); !errors.Is(err, ErrObjectConflict) {
		t.Fatalf("conflict error %v", err)
	}
	body, err := store.Open(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	data, err := io.ReadAll(body)
	if err != nil || string(data) != "video-A" {
		t.Fatalf("readback=%q err=%v", data, err)
	}
	// Independent clients share only the database claim. Exercise concurrent
	// different bytes on a fresh key; exactly one payload can ever be admitted.
	raceKey := key + "-race"
	defer store.Remove(ctx, raceKey)
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, payload := range []string{"video-A", "video-B"} {
		other, err := newMinioStore(cfg.Storage.ObjectStorage, cfg.Credentials.ObjectStorage)
		if err != nil {
			t.Fatal(err)
		}
		other.claims = storagerepository.ContentClaims{DB: database.DB()}
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results <- other.PutImmutable(ctx, raceKey, strings.NewReader(payload), 7, "application/octet-stream", DigestOf([]byte(payload)))
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	succeeded, conflicted := 0, 0
	for err := range results {
		if err == nil {
			succeeded++
		} else if errors.Is(err, ErrObjectConflict) {
			conflicted++
		} else {
			t.Fatal(err)
		}
	}
	if succeeded != 1 || conflicted != 1 {
		t.Fatalf("succeeded=%d conflicted=%d", succeeded, conflicted)
	}
	traced, err := store.Open(ctx, raceKey)
	if err != nil {
		t.Fatal(err)
	}
	defer traced.Close()
	got, err := io.ReadAll(traced)
	if err != nil || (string(got) != "video-A" && string(got) != "video-B") {
		t.Fatalf("concurrent readback=%q err=%v", got, err)
	}
}
