package compose

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/infra/storage"
)

// VerifyUpload uploads the file under a temporary key, checks the stored size,
// and removes the key. This is the manual `-input` self-check, not the
// production path, and it deliberately performs the same integrity checks the
// production upload does: the store verifies the Content-MD5 that `Put` sends,
// and the size is read back as an independent second fact. It does not download
// the object again — the checksum already proves the stored bytes are the ones
// uploaded.
func VerifyUpload(ctx context.Context, store storage.Store, path, runID string) (resultErr error) {
	if !runIDPattern.MatchString(runID) {
		return fmt.Errorf("unsafe run id %q", runID)
	}
	key := "compose-verification/" + runID + "/output.mp4"
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open compose output: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Size() <= 0 {
		return errors.New("compose output is empty")
	}
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := store.Remove(cleanupCtx, key); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("remove temporary object: %w", err))
		}
	}()
	if err := store.Put(ctx, key, file, info.Size(), "video/mp4"); err != nil {
		return fmt.Errorf("upload temporary object: %w", err)
	}
	remote, err := store.Stat(ctx, key)
	if err != nil {
		return fmt.Errorf("stat temporary object: %w", err)
	}
	if remote.Size != info.Size() {
		return fmt.Errorf("temporary object size mismatch: local %d, remote %d", info.Size(), remote.Size)
	}
	return nil
}
