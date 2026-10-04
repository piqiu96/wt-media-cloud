package deploy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	cloudconfig "github.com/wt-media/wt-media-cloud/internal/config"
)

func VerifyRuntime(ctx context.Context, profile Profile, releaseRoot string) error {
	cfg, err := cloudconfig.LoadFromDir(filepath.Join(releaseRoot, "config"))
	if err != nil {
		return err
	}
	base := strings.TrimRight(profile.Deploy.HealthURL, "/")
	if base == "" {
		addr := cfg.App.Server.HTTPAddr
		if strings.HasPrefix(addr, ":") {
			addr = "127.0.0.1" + addr
		}
		base = "http://" + addr
	}
	if _, err := url.ParseRequestURI(base); err != nil {
		return fmt.Errorf("invalid health URL: %w", err)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	for _, path := range []string{"/healthz", "/api/v1/health", "/", "/login"} {
		response, err := client.Get(base + path)
		if err != nil {
			return fmt.Errorf("GET %s: %w", path, err)
		}
		_, _ = io.Copy(io.Discard, response.Body)
		response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return fmt.Errorf("GET %s returned HTTP %d", path, response.StatusCode)
		}
	}
	response, err := client.Get(base + "/api/v1/not-found")
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, response.Body)
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		return fmt.Errorf("unknown API path returned HTTP %d, want 404", response.StatusCode)
	}
	loginBody := map[string]any{
		"username":         cfg.App.InitialAdmin.Username,
		"password":         cfg.App.InitialAdmin.Password,
		"replace_existing": true,
	}
	raw, err := json.Marshal(loginBody)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/v1/auth/login", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err = client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("admin login returned HTTP %d", response.StatusCode)
	}
	var result struct {
		ErrCode int `json:"errcode"`
		Data    struct {
			Username string `json:"username"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return err
	}
	if result.ErrCode != 0 || result.Data.Username != cfg.App.InitialAdmin.Username {
		return errors.New("admin login verification failed")
	}
	return nil
}

func CheckServiceProcesses(profile Profile) (map[string]bool, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("read /proc: %w", err)
	}
	wanted := map[string]string{
		"server":    filepath.Join(profile.Deploy.InstallRoot, "current", "bin", "wt-media-cloud"),
		"worker":    filepath.Join(profile.Deploy.InstallRoot, "current", "bin", "discovery-worker"),
		"scheduler": filepath.Join(profile.Deploy.InstallRoot, "current", "bin", "discovery-scheduler"),
	}
	result := map[string]bool{"server": false, "worker": false, "scheduler": false}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		raw, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "cmdline"))
		if err != nil || len(raw) == 0 {
			continue
		}
		cmdline := strings.ReplaceAll(string(raw), "\x00", " ")
		for name, binary := range wanted {
			if strings.Contains(cmdline, binary) {
				result[name] = true
			}
		}
	}
	return result, nil
}
