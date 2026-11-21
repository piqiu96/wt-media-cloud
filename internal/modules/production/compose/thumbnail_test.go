package compose

import (
	"bytes"
	"context"
	"image/png"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestExtractThumbnailUsesActualOutputFrame(t *testing.T) {
	binary, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("requires ffmpeg")
	}
	video := filepath.Join(t.TempDir(), "final.mp4")
	ctx := context.Background()
	if out, err := exec.CommandContext(ctx, binary, "-v", "error", "-nostdin", "-f", "lavfi", "-i", "color=c=red:s=64x48:d=0.2", "-c:v", "mpeg4", video).CombinedOutput(); err != nil {
		t.Fatalf("fixture: %s %v", out, err)
	}
	data, err := ExtractThumbnail(ctx, binary, video)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, _ := img.At(32, 24).RGBA()
	if img.Bounds().Dx() != 64 || img.Bounds().Dy() != 48 || r < 60000 || g > 5000 || b > 5000 {
		t.Fatalf("not the actual frame: %v %d %d %d", img.Bounds(), r, g, b)
	}
	if _, err = ExtractThumbnail(ctx, binary, filepath.Join(t.TempDir(), "missing.mp4")); err == nil {
		t.Fatal("missing output produced an image")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = ExtractThumbnail(cancelled, binary, video); err == nil {
		t.Fatal("cancelled extraction succeeded")
	}
}
