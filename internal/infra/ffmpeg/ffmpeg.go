// Package ffmpeg verifies the video-processing binaries a compose run depends
// on before any work is claimed.
//
// The compose worker does not search PATH for these tools: configuration names
// them, and Verify is the gate that turns a wrong path, a different build, or a
// build without a required encoder into a startup failure instead of a
// mid-flight one. SHA pins are optional for the same reason the object-storage
// credential is: a development tree runs the tools its platform installed,
// while a controlled deployment pins the exact bytes it ships.
package ffmpeg

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Tools names the binaries and the facts they must report. Version is the
// release token both binaries print on their first `-version` line, so one
// value covers the pair the way one archive ships them.
type Tools struct {
	FFmpegPath  string
	FFprobePath string
	Version     string

	RequiredEncoders []string

	// FFmpegSHA256 and FFprobeSHA256 pin the binary bytes when set. Empty means
	// unpinned, which is the state of a development tree.
	FFmpegSHA256  string
	FFprobeSHA256 string
}

// Verify runs every check and reports the first failure. It returns nil only
// when both binaries exist, are executable, are optionally the pinned bytes,
// answer `-version` with the expected version, and ffmpeg lists every required
// encoder.
func Verify(ctx context.Context, tools Tools) error {
	if strings.TrimSpace(tools.Version) == "" {
		return fmt.Errorf("ffmpeg tool verification needs an expected version")
	}
	if err := verifyBinary(ctx, tools.FFprobePath, tools.Version, nil, tools.FFprobeSHA256, "ffprobe"); err != nil {
		return err
	}
	return verifyBinary(ctx, tools.FFmpegPath, tools.Version, tools.RequiredEncoders, tools.FFmpegSHA256, "ffmpeg")
}

func verifyBinary(ctx context.Context, path, expectedVersion string, requiredEncoders []string, shaHex, name string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("%s path is not configured", name)
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("%s at %s: %w", name, path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("%s at %s is a directory", name, path)
	}
	if info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("%s at %s is not executable", name, path)
	}
	if err := verifySHAPin(path, shaHex, name); err != nil {
		return err
	}
	if err := verifyVersion(ctx, path, expectedVersion, name); err != nil {
		return err
	}
	if name == "ffmpeg" {
		return verifyEncoders(ctx, path, requiredEncoders)
	}
	return nil
}

func verifySHAPin(path, shaHex, name string) error {
	pin := strings.TrimSpace(shaHex)
	if pin == "" {
		return nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("%s at %s: %w", name, path, err)
	}
	digest := sha256.Sum256(content)
	actual := hex.EncodeToString(digest[:])
	if !strings.EqualFold(actual, pin) {
		return fmt.Errorf("%s at %s: sha256 mismatch: pinned %s, actual %s", name, path, pin, actual)
	}
	return nil
}

func verifyVersion(ctx context.Context, path, expectedVersion, name string) error {
	output, err := runTool(ctx, path, "-version")
	if err != nil {
		return fmt.Errorf("%s at %s: -version: %w", name, path, err)
	}
	firstLine := firstLine(output)
	if firstLine == "" {
		return fmt.Errorf("%s at %s: -version printed nothing", name, path)
	}
	if !strings.Contains(firstLine, "version "+expectedVersion) {
		return fmt.Errorf("%s at %s: version mismatch: expected %q, first line %q", name, path, expectedVersion, firstLine)
	}
	return nil
}

func verifyEncoders(ctx context.Context, path string, required []string) error {
	if len(required) == 0 {
		return nil
	}
	output, err := runTool(ctx, path, "-hide_banner", "-encoders")
	if err != nil {
		return fmt.Errorf("ffmpeg at %s: -encoders: %w", path, err)
	}
	available := make(map[string]bool)
	for _, field := range strings.Fields(string(output)) {
		available[field] = true
	}
	for _, encoder := range required {
		if !available[encoder] {
			return fmt.Errorf("ffmpeg at %s: required encoder %q is not available", path, encoder)
		}
	}
	return nil
}

func runTool(ctx context.Context, path string, args ...string) ([]byte, error) {
	command := exec.CommandContext(ctx, path, args...)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		if detail != "" {
			return nil, fmt.Errorf("%w: %s", err, detail)
		}
		return nil, err
	}
	return stdout.Bytes(), nil
}

func firstLine(output []byte) string {
	text := strings.TrimSpace(string(output))
	if text == "" {
		return ""
	}
	line, _, _ := strings.Cut(text, "\n")
	return strings.TrimSpace(line)
}
