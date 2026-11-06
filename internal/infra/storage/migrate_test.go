package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

type migrationMemoryStore struct{ objects map[string][]byte }

func (s *migrationMemoryStore) Reference(key string) (string, error) {
	return "s3://wt-media/" + key, nil
}
func (s *migrationMemoryStore) Open(_ context.Context, key string) (io.ReadCloser, error) {
	b, ok := s.objects[key]
	if !ok {
		return nil, ErrObjectNotFound
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}
func (s *migrationMemoryStore) Stat(_ context.Context, key string) (ObjectInfo, error) {
	b, ok := s.objects[key]
	if !ok {
		return ObjectInfo{}, ErrObjectNotFound
	}
	return ObjectInfo{Key: key, Size: int64(len(b)), ContentType: "video/mp4"}, nil
}
func (s *migrationMemoryStore) PutImmutable(_ context.Context, key string, r io.Reader, size int64, kind, digest string) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if old, ok := s.objects[key]; ok && !bytes.Equal(old, b) {
		return ErrObjectConflict
	}
	s.objects[key] = b
	return nil
}
func (s *migrationMemoryStore) PresignGet(context.Context, string, time.Duration) (Grant, error) {
	return Grant{}, errors.New("unused")
}
func (s *migrationMemoryStore) Put(context.Context, string, io.Reader, int64, string) error {
	return errors.New("unused")
}
func (s *migrationMemoryStore) Copy(context.Context, string, string) error {
	return errors.New("unused")
}
func (s *migrationMemoryStore) Remove(context.Context, string) error {
	return errors.New("must not delete")
}

func TestMigrationVerifiesBeforeSwitchingAndCanBeRepeated(t *testing.T) {
	ctx := context.Background()
	body := []byte("real video bytes")
	store := &migrationMemoryStore{map[string][]byte{"old/key.mp4": body}}
	key := "media-ku/material/42.mp4"
	destination := "s3://wt-media/" + key
	result, err := MigrateVerified(ctx, store, "old/key.mp4", key, int64(len(body)), DigestOf(body), false)
	if err != nil || result.Location != destination {
		t.Fatalf("%+v %v", result, err)
	}
	if _, ok := store.objects[destination]; ok {
		t.Fatal("dry run wrote a new object")
	}
	for range 2 {
		if _, err := MigrateVerified(ctx, store, "old/key.mp4", key, int64(len(body)), DigestOf(body), true); err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Equal(store.objects[destination], body) || !bytes.Equal(store.objects["old/key.mp4"], body) {
		t.Fatal("copy changed bytes or deleted source")
	}
}

func TestMigrationRefusesWrongSourceDigestAndConflictingDestination(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		store := &migrationMemoryStore{map[string][]byte{"old": []byte("video-A")}}
		digest := DigestOf([]byte("wrong"))
		if conflict {
			store.objects["s3://wt-media/new"] = []byte("video-B")
			digest = DigestOf([]byte("video-A"))
		}
		if _, err := MigrateVerified(context.Background(), store, "old", "new", 7, digest, true); err == nil {
			t.Fatal("unsafe copy accepted")
		}
		if conflict && string(store.objects["s3://wt-media/new"]) != "video-B" {
			t.Fatal("overwrote destination")
		}
		if !conflict {
			if _, ok := store.objects["s3://wt-media/new"]; ok {
				t.Fatal("copied corrupt source")
			}
		}
	}
}
