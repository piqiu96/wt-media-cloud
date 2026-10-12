package compose

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/wt-media/wt-media-cloud/internal/infra/storage"
)

// VerifyUpload checks stored bytes through a signed GET and removes the temporary key.
func VerifyUpload(ctx context.Context, store storage.Store, grantTTL time.Duration, path, runID string) (resultErr error) {
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
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("hash compose output: %w", err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
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
	grant, err := store.PresignGet(ctx, key, grantTTL)
	if err != nil {
		return fmt.Errorf("presign temporary object: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, grant.URL, nil)
	if err != nil {
		return fmt.Errorf("readback request: %w", err)
	}
	response, err := (&http.Client{Timeout: 2 * time.Minute}).Do(request)
	if err != nil {
		return fmt.Errorf("readback temporary object: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("readback temporary object: HTTP %d", response.StatusCode)
	}
	readHash := sha256.New()
	readSize, err := io.Copy(readHash, io.LimitReader(response.Body, info.Size()+1))
	if err != nil {
		return fmt.Errorf("hash readback: %w", err)
	}
	if readSize != info.Size() {
		return fmt.Errorf("readback size mismatch: local %d, remote %d", info.Size(), readSize)
	}
	if string(readHash.Sum(nil)) != string(hash.Sum(nil)) {
		return errors.New("readback sha256 mismatch")
	}
	return nil
}
