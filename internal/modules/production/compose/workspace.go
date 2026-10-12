// Package compose executes one composition inside an isolated per-run
// directory: validate the parameters against the bound template version, check
// the input with ffprobe, run the fixed ffmpeg command, and check the output
// before reporting success. Nothing here claims tasks or writes business
// tables; M4-C5 builds the claiming loop on top of this pipeline.
package compose

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// A Workspace hands each compose run one directory under the configured root.
// Directory names carry nothing but the internal run id — no titles, no user
// text — so a run directory can never be a path the outside world influenced.
type Workspace struct {
	root string
}

// runIDPattern is what a caller may name a run: lowercase letters, digits, and
// dashes. It excludes separators and dots on purpose: a run id that could
// escape the root, or that carries a file extension, is not an internal
// identifier.
var runIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// NewWorkspace prepares the workspace root. The configured root may be
// relative to the process working directory here; config anchors it to the
// Cloud home before this package ever sees it.
func NewWorkspace(root string) (*Workspace, error) {
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("compose workspace root is not configured")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve compose workspace root: %w", err)
	}
	if err := os.MkdirAll(absolute, 0o755); err != nil {
		return nil, fmt.Errorf("create compose workspace root: %w", err)
	}
	return &Workspace{root: absolute}, nil
}

// Root is the absolute workspace root, for tests and diagnostics.
func (w *Workspace) Root() string {
	return w.root
}

// NewRun creates the directory one run works in. An id that already has a
// directory is rejected: two live runs sharing a directory is the overlap the
// isolation exists to prevent.
func (w *Workspace) NewRun(runID string) (*RunDir, error) {
	if !runIDPattern.MatchString(runID) {
		return nil, fmt.Errorf("run id %q is not a safe internal identifier", runID)
	}
	dir := filepath.Join(w.root, runID)
	if err := os.Mkdir(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create run directory: %w", err)
	}
	return &RunDir{dir: dir}, nil
}

// A RunDir is the directory one compose run owns.
type RunDir struct {
	dir string
}

// Path is the run's directory.
func (d *RunDir) Path() string {
	return d.dir
}

// Release removes this run's directory and nothing else — not the workspace
// root, not a sibling run's files, and no object anywhere else.
func (d *RunDir) Release() error {
	return os.RemoveAll(d.dir)
}
