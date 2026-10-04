package deploy

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

func InstallRelease(profile Profile, sourceRoot string) error {
	if _, err := VerifyPackage(sourceRoot, profile.Deploy.Release); err != nil {
		return err
	}
	destination := profile.ReleaseRoot()
	if _, err := os.Stat(destination); err == nil {
		return fmt.Errorf("release already exists: %s", destination)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	if err := copyTree(sourceRoot, destination); err != nil {
		return err
	}
	for _, relative := range []string{"logs", "data/tmp"} {
		if err := os.MkdirAll(filepath.Join(destination, relative), 0o750); err != nil {
			return err
		}
	}
	if err := setOwnership(destination, profile.Deploy.ServiceUser); err != nil {
		return err
	}
	return nil
}

func ActivateRelease(profile Profile, release string) error {
	destination := filepath.Join(profile.Deploy.InstallRoot, "releases", release)
	if _, err := os.Stat(filepath.Join(destination, "release-info.json")); err != nil {
		return fmt.Errorf("release not found: %s", destination)
	}
	if _, err := os.Stat(filepath.Join(destination, "config", "database", "primary.toml")); err != nil {
		return errors.New("release has no rendered private config")
	}
	if _, err := os.Stat(filepath.Join(destination, "config", ".render-info.json")); err != nil {
		return errors.New("release has no configuration render record")
	}
	current := filepath.Join(profile.Deploy.InstallRoot, "current")
	temp := filepath.Join(profile.Deploy.InstallRoot, fmt.Sprintf(".current-%d", os.Getpid()))
	_ = os.Remove(temp)
	if err := os.Symlink(filepath.Join("releases", release), temp); err != nil {
		return err
	}
	defer os.Remove(temp)
	return os.Rename(temp, current)
}

func copyTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return os.MkdirAll(destination, 0o755)
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("package may not contain symlinks: %s", relative)
		}
		target := filepath.Join(destination, relative)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("package contains unsupported file: %s", relative)
		}
		return copyRegularFile(path, target, info.Mode().Perm())
	})
}

func copyRegularFile(source, destination string, mode os.FileMode) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Chmod(destination, mode)
}

func setOwnership(root, username string) error {
	target, err := user.Lookup(username)
	if err != nil {
		return fmt.Errorf("service user %q: %w", username, err)
	}
	uid, err := strconv.Atoi(target.Uid)
	if err != nil {
		return err
	}
	gid, err := strconv.Atoi(target.Gid)
	if err != nil {
		return err
	}
	current := os.Geteuid()
	if current != 0 && current != uid {
		return fmt.Errorf("installing as uid %d cannot chown release to %s", current, username)
	}
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(path, uid, gid)
	})
}

func WritePrivateFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func CurrentRelease(profile Profile) (string, error) {
	target, err := os.Readlink(filepath.Join(profile.Deploy.InstallRoot, "current"))
	if err != nil {
		return "", err
	}
	target = strings.TrimSpace(target)
	return filepath.Base(target), nil
}

func EnsureServiceUser(username string) error {
	if _, err := user.Lookup(username); err != nil {
		return fmt.Errorf("service user %q: %w", username, err)
	}
	return nil
}

func ensureWritableDirectory(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", path)
	}
	file, err := os.CreateTemp(path, ".wtmctl-write-probe-")
	if err != nil {
		return err
	}
	name := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	return os.Remove(name)
}
