package ffmpeg

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// toolScript writes an executable stand-in for ffmpeg or ffprobe that answers
// the two questions the verifier asks: `-version` and `-hide_banner -encoders`.
// A test that wants a binary which cannot answer uses toolScriptFailing or
// writes no script at all.
func toolScript(t *testing.T, dir, name, version string, encoders []string) string {
	t.Helper()
	var builder strings.Builder
	builder.WriteString("#!/bin/sh\ncase \"$1\" in\n")
	builder.WriteString("  -version) echo \"" + name + " version " + version + " Copyright (c) test\";;\n")
	builder.WriteString("  -hide_banner) for e in " + strings.Join(encoders, " ") + "; do echo \" V....D $e\"; done;;\n")
	builder.WriteString("esac\n")
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(builder.String()), 0o755); err != nil {
		t.Fatalf("write tool script: %v", err)
	}
	return path
}

func fileSHA256(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

func matchingTools(t *testing.T) Tools {
	t.Helper()
	dir := t.TempDir()
	return Tools{
		FFmpegPath:       toolScript(t, dir, "ffmpeg", "8.1.2", []string{"libx264", "aac"}),
		FFprobePath:      toolScript(t, dir, "ffprobe", "8.1.2", nil),
		Version:          "8.1.2",
		RequiredEncoders: []string{"libx264", "aac"},
	}
}

func TestVerifyAcceptsMatchingTools(t *testing.T) {
	if err := Verify(context.Background(), matchingTools(t)); err != nil {
		t.Fatalf("Verify() error = %v, want nil for a tool set that answers every question", err)
	}
}

func TestVerifyRejectsAMissingBinary(t *testing.T) {
	tools := matchingTools(t)
	tools.FFprobePath = filepath.Join(t.TempDir(), "absent-ffprobe")
	err := Verify(context.Background(), tools)
	if err == nil {
		t.Fatal("Verify() = nil, want an error when the ffprobe binary does not exist")
	}
	if !strings.Contains(err.Error(), "ffprobe") {
		t.Fatalf("Verify() error = %v, want it to name ffprobe", err)
	}
}

func TestVerifyRejectsAVersionMismatch(t *testing.T) {
	dir := t.TempDir()
	tools := matchingTools(t)
	tools.FFmpegPath = toolScript(t, dir, "ffmpeg", "7.0.1", []string{"libx264", "aac"})
	err := Verify(context.Background(), tools)
	if err == nil {
		t.Fatal("Verify() = nil, want an error when the binary reports a different version")
	}
	if !strings.Contains(err.Error(), "8.1.2") || !strings.Contains(err.Error(), "7.0.1") {
		t.Fatalf("Verify() error = %v, want both the expected and the actual version", err)
	}
}

func TestVerifyRejectsAMissingEncoder(t *testing.T) {
	dir := t.TempDir()
	tools := matchingTools(t)
	tools.FFmpegPath = toolScript(t, dir, "ffmpeg", "8.1.2", []string{"aac"})
	err := Verify(context.Background(), tools)
	if err == nil {
		t.Fatal("Verify() = nil, want an error when a required encoder is absent")
	}
	if !strings.Contains(err.Error(), "libx264") {
		t.Fatalf("Verify() error = %v, want it to name the missing encoder", err)
	}
}

func TestVerifyRejectsAnUnexecutableBinary(t *testing.T) {
	tools := matchingTools(t)
	if err := os.Chmod(tools.FFmpegPath, 0o644); err != nil {
		t.Fatalf("chmod ffmpeg script: %v", err)
	}
	if err := Verify(context.Background(), tools); err == nil {
		t.Fatal("Verify() = nil, want an error when the binary is not executable")
	}
}

func TestVerifyRejectsABinaryThatCannotAnswer(t *testing.T) {
	dir := t.TempDir()
	tools := matchingTools(t)
	tools.FFprobePath = filepath.Join(dir, "ffprobe")
	if err := os.WriteFile(tools.FFprobePath, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("write failing script: %v", err)
	}
	if err := Verify(context.Background(), tools); err == nil {
		t.Fatal("Verify() = nil, want an error when -version exits non-zero")
	}
}

func TestVerifyRejectsEmptyPaths(t *testing.T) {
	tools := matchingTools(t)
	tools.FFmpegPath = ""
	if err := Verify(context.Background(), tools); err == nil {
		t.Fatal("Verify() = nil, want an error for an empty ffmpeg path")
	}
}

func TestVerifyChecksConfiguredSHAPins(t *testing.T) {
	tools := matchingTools(t)
	tools.FFmpegSHA256 = fileSHA256(t, tools.FFmpegPath)
	tools.FFprobeSHA256 = fileSHA256(t, tools.FFprobePath)
	if err := Verify(context.Background(), tools); err != nil {
		t.Fatalf("Verify() error = %v, want nil when both pins match", err)
	}

	tools.FFmpegSHA256 = strings.Repeat("0", 64)
	err := Verify(context.Background(), tools)
	if err == nil {
		t.Fatal("Verify() = nil, want an error when the ffmpeg pin does not match")
	}
	if !strings.Contains(err.Error(), "sha256") {
		t.Fatalf("Verify() error = %v, want it to say which pin failed", err)
	}
}
