package compose

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wt-media/wt-media-cloud/internal/modules/production/executor"
)

// fixtureTools writes stand-in ffmpeg/ffprobe binaries. Both read the target
// file's plain-text "spec" line instead of decoding anything:
//
//	ffprobe answers a spec of `duration=<seconds>` with a video stream, and
//	fails outright on PROBEFAIL or reports no video stream on NOVIDEO.
//	ffmpeg writes the output spec the -ss/-to arguments imply, and fails on
//	ENCODEFAIL, omits the output file on NOWRITE, writes a video-less output on
//	OUTNOVIDEO, and a too-short output on OUTSHORT.
//
// That is every outcome the pipeline must tell apart, each reachable from a
// one-line fixture file.
func fixtureTools(t *testing.T) (ffmpegPath, ffprobePath string) {
	t.Helper()
	dir := t.TempDir()

	ffprobePath = filepath.Join(dir, "ffprobe")
	probeScript := `#!/bin/sh
for f in "$@"; do :; done
spec=$(cat "$f" 2>/dev/null)
case "$spec" in
	*PROBEFAIL*) echo "cannot probe" >&2; exit 1;;
	*NOVIDEO*) echo '{"format":{"duration":"30.0"},"streams":[{"codec_type":"audio"}]}';;
	*hasvideo=0*) echo '{"format":{"duration":"30.0"},"streams":[{"codec_type":"audio"}]}';;
	*duration=*)
		dur=${spec#*duration=}; dur=${dur%% *}
		echo "{\"format\":{\"duration\":\"$dur\"},\"streams\":[{\"codec_type\":\"video\"}]}";;
	*) echo '{"format":{},"streams":[]}';;
esac
`
	if err := os.WriteFile(ffprobePath, []byte(probeScript), 0o755); err != nil {
		t.Fatalf("write ffprobe fixture: %v", err)
	}

	ffmpegPath = filepath.Join(dir, "ffmpeg")
	encodeScript := `#!/bin/sh
prev=""; ss=""; to=""; input=""; output=""
for a in "$@"; do
	case "$prev" in
		-ss) ss="$a";;
		-to) to="$a";;
		-i) input="$a";;
	esac
	prev="$a"
	output="$a"
done
spec=$(cat "$input" 2>/dev/null)
case "$spec" in
	*ENCODEFAIL*) echo "encode error" >&2; exit 1;;
	*NOWRITE*) exit 0;;
	*OUTNOVIDEO*) echo "hasvideo=0" > "$output"; exit 0;;
	*OUTSHORT*) echo "duration=1" > "$output"; exit 0;;
	*OUTLONG*) echo "duration=30" > "$output"; exit 0;;
esac
echo "duration=$((to - ss))" > "$output"
`
	if err := os.WriteFile(ffmpegPath, []byte(encodeScript), 0o755); err != nil {
		t.Fatalf("write ffmpeg fixture: %v", err)
	}
	return ffmpegPath, ffprobePath
}

// fixtureInput writes the one-line spec a fixture tool reads as a video of the
// given duration, plus optional markers from the tool table above.
func fixtureInput(t *testing.T, dir, spec string) string {
	t.Helper()
	path := filepath.Join(dir, "input.mp4")
	if err := os.WriteFile(path, []byte(spec+"\n"), 0o644); err != nil {
		t.Fatalf("write input fixture: %v", err)
	}
	return path
}

func newRunner(t *testing.T) (*Runner, *Workspace) {
	t.Helper()
	ffmpegPath, ffprobePath := fixtureTools(t)
	registry, err := executor.NewRegistry(executor.BasicCutV1())
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	workspace, err := NewWorkspace(filepath.Join(t.TempDir(), "workspace"))
	if err != nil {
		t.Fatalf("NewWorkspace() error = %v", err)
	}
	return &Runner{
		FFmpegPath:  ffmpegPath,
		FFprobePath: ffprobePath,
		Registry:    registry,
	}, workspace
}

func cutRequest(inputPath string) Request {
	return Request{
		TemplateCode:    "basic_cut",
		TemplateVersion: 1,
		Params:          json.RawMessage(`{"start_seconds": 5, "end_seconds": 12}`),
		InputPath:       inputPath,
	}
}

func TestWorkspaceCreatesAnIsolatedDirectoryPerRun(t *testing.T) {
	_, workspace := newRunner(t)

	runDir, err := workspace.NewRun("run-0001")
	if err != nil {
		t.Fatalf("NewRun() error = %v", err)
	}
	if !strings.HasPrefix(runDir.Path(), workspace.Root()) {
		t.Fatalf("run directory %s is not under the workspace root %s", runDir.Path(), workspace.Root())
	}
	if filepath.Base(runDir.Path()) != "run-0001" {
		t.Fatalf("run directory is named %q, want the internal run id", filepath.Base(runDir.Path()))
	}
}

// The directory name carries nothing but an internal id: no user text, no
// titles, and never something that could escape the workspace root.
func TestWorkspaceRejectsUnsafeRunIDs(t *testing.T) {
	_, workspace := newRunner(t)
	for _, runID := range []string{"", "../escape", "a/b", "run with space", strings.Repeat("r", 65)} {
		if _, err := workspace.NewRun(runID); err == nil {
			t.Fatalf("NewRun(%q) = nil error, want a rejection", runID)
		}
	}
}

func TestWorkspaceReleaseRemovesOnlyItsOwnDirectory(t *testing.T) {
	_, workspace := newRunner(t)
	own, err := workspace.NewRun("run-own")
	if err != nil {
		t.Fatalf("NewRun() error = %v", err)
	}
	sibling, err := workspace.NewRun("run-sibling")
	if err != nil {
		t.Fatalf("NewRun() error = %v", err)
	}

	if err := own.Release(); err != nil {
		t.Fatalf("Release() error = %v", err)
	}
	if _, err := os.Stat(own.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("own run directory still exists after Release: %v", err)
	}
	if _, err := os.Stat(sibling.Path()); err != nil {
		t.Fatalf("sibling run directory must survive a sibling's Release: %v", err)
	}
}

func TestRunCutsThroughTheWholeChain(t *testing.T) {
	runner, workspace := newRunner(t)
	input := fixtureInput(t, t.TempDir(), "duration=30")
	runDir, err := workspace.NewRun("run-0001")
	if err != nil {
		t.Fatalf("NewRun() error = %v", err)
	}
	defer runDir.Release()

	output, err := runner.Run(context.Background(), runDir, cutRequest(input))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if filepath.Base(output.Path) != "output.mp4" {
		t.Fatalf("output name = %q, want the fixed output.mp4", filepath.Base(output.Path))
	}
	if _, err := os.Stat(output.Path); err != nil {
		t.Fatalf("output file missing after a successful run: %v", err)
	}
	if output.DurationMS != 7000 {
		t.Fatalf("output duration = %d ms, want 7000", output.DurationMS)
	}
}

func TestRunRejectsAnUnknownTemplateVersion(t *testing.T) {
	runner, workspace := newRunner(t)
	input := fixtureInput(t, t.TempDir(), "duration=30")
	runDir, err := workspace.NewRun("run-0001")
	if err != nil {
		t.Fatalf("NewRun() error = %v", err)
	}
	defer runDir.Release()

	request := cutRequest(input)
	request.TemplateVersion = 9
	_, err = runner.Run(context.Background(), runDir, request)
	if !errors.Is(err, executor.ErrUnknownTemplateVersion) {
		t.Fatalf("Run() error = %v, want ErrUnknownTemplateVersion", err)
	}
}

func TestRunRejectsParamsOutsideTheSchemaBeforeTouchingTools(t *testing.T) {
	runner, workspace := newRunner(t)
	input := fixtureInput(t, t.TempDir(), "duration=30")
	runDir, err := workspace.NewRun("run-0001")
	if err != nil {
		t.Fatalf("NewRun() error = %v", err)
	}
	defer runDir.Release()

	request := cutRequest(input)
	request.Params = json.RawMessage(`{"start_seconds": 5, "end_seconds": 12, "intro": "/tmp/intro.mp4"}`)
	if _, err := runner.Run(context.Background(), runDir, request); err == nil {
		t.Fatal("Run() = nil error, want a rejection for schema-external params")
	}
}

func TestRunRejectsInputThatFailsTheProbe(t *testing.T) {
	runner, workspace := newRunner(t)
	cases := []struct {
		name string
		spec string
	}{
		{"unreadable", "PROBEFAIL"},
		{"no-video-stream", "NOVIDEO"},
		{"no-duration", "nothing declared"},
		{"too-short-for-cut", "duration=6"},
	}
	for _, testCase := range cases {
		input := fixtureInput(t, t.TempDir(), testCase.spec)
		runDir, err := workspace.NewRun("run-" + testCase.name)
		if err != nil {
			t.Fatalf("NewRun(%s) error = %v", testCase.name, err)
		}
		if _, err := runner.Run(context.Background(), runDir, cutRequest(input)); err == nil {
			t.Fatalf("Run(%s) = nil error, want an input rejection", testCase.name)
		}
		runDir.Release()
	}
}

func TestRunRejectsAFailingOrDefectiveEncode(t *testing.T) {
	runner, workspace := newRunner(t)
	cases := []struct {
		name      string
		spec      string
		wantInErr string
	}{
		{"encode-exits-nonzero", "duration=30 ENCODEFAIL", "encode error"},
		{"no-output-written", "duration=30 NOWRITE", "output file"},
		{"output-has-no-video", "duration=30 OUTNOVIDEO", "video"},
		{"output-too-short", "duration=30 OUTSHORT", "duration"},
		{"output-too-long", "duration=30 OUTLONG", "duration"},
	}
	for index, testCase := range cases {
		input := fixtureInput(t, t.TempDir(), testCase.spec)
		runDir, err := workspace.NewRun("run-" + strings.Split(testCase.name, "-")[0] + "-" + string(rune('a'+index)))
		if err != nil {
			t.Fatalf("NewRun() error = %v", err)
		}
		_, err = runner.Run(context.Background(), runDir, cutRequest(input))
		if err == nil {
			t.Fatalf("Run(%s) = nil error, want an encode rejection", testCase.name)
		}
		if !strings.Contains(err.Error(), testCase.wantInErr) {
			t.Fatalf("Run(%s) error = %q, want it to mention %q", testCase.name, err.Error(), testCase.wantInErr)
		}
		runDir.Release()
	}
}
