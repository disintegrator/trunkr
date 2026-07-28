package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// approvalsFileName lives in HERDR_PLUGIN_STATE_DIR and records which
// repositories' committed config files the user has accepted.
const approvalsFileName = "approvals.json"

type approvalStore struct {
	Version int                 `json:"version"`
	Repos   map[string]approval `json:"repos"`
}

type approval struct {
	ApprovedAt time.Time `json:"approved_at"`
}

// Approved reports whether repoID's committed config has been accepted. A
// missing store means nothing is approved.
func Approved(stateDir, repoID string) (bool, error) {
	if stateDir == "" {
		return false, nil
	}
	store, err := readApprovals(stateDir)
	if err != nil {
		return false, err
	}
	_, ok := store.Repos[repoID]
	return ok, nil
}

// Approve records the user's acceptance of repoID's committed config. The
// approval is once per repository: later edits to the file apply without
// re-prompting.
func Approve(stateDir, repoID string) error {
	if stateDir == "" {
		return errors.New("cannot record approval: state directory is not set (HERDR_PLUGIN_STATE_DIR)")
	}
	if repoID == "" {
		return errors.New("cannot record approval: empty repository id")
	}
	store, err := readApprovals(stateDir)
	if err != nil {
		return err
	}
	store.Repos[repoID] = approval{ApprovedAt: time.Now().UTC()}

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(stateDir, approvalsFileName)
	tmp, err := os.CreateTemp(stateDir, approvalsFileName+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func readApprovals(stateDir string) (approvalStore, error) {
	store := approvalStore{Version: 1, Repos: map[string]approval{}}
	data, err := os.ReadFile(filepath.Join(stateDir, approvalsFileName))
	if errors.Is(err, fs.ErrNotExist) {
		return store, nil
	}
	if err != nil {
		return store, err
	}
	if err := json.Unmarshal(data, &store); err != nil {
		return store, fmt.Errorf("%s: %w", filepath.Join(stateDir, approvalsFileName), err)
	}
	if store.Repos == nil {
		store.Repos = map[string]approval{}
	}
	return store, nil
}
