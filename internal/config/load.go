package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	// GlobalFileName is the user's global config inside HERDR_PLUGIN_CONFIG_DIR.
	GlobalFileName = "trunkr.toml"
	// RepoFileName is the team-shared, approval-gated config committed in a repo.
	RepoFileName = ".trunkr.toml"
	// LocalFileName is the user's untracked per-repo override.
	LocalFileName = ".trunkr.local.toml"
)

// Source names the directories a Load reads from. Any of them may be empty,
// which skips the corresponding layer(s).
type Source struct {
	// GlobalDir is HERDR_PLUGIN_CONFIG_DIR, holding trunkr.toml.
	GlobalDir string
	// StateDir is HERDR_PLUGIN_STATE_DIR, holding the approval store.
	StateDir string
	// RepoDir is the target repo/workspace directory holding .trunkr.toml
	// and .trunkr.local.toml. Always the workspace the action targets —
	// never the plugin root.
	RepoDir string
}

// SourceFromEnv builds a Source from the herdr plugin environment plus the
// target repo directory.
func SourceFromEnv(repoDir string) Source {
	return Source{
		GlobalDir: os.Getenv("HERDR_PLUGIN_CONFIG_DIR"),
		StateDir:  os.Getenv("HERDR_PLUGIN_STATE_DIR"),
		RepoDir:   repoDir,
	}
}

// PendingFile is a gated repo config file that exists but has no approval
// yet. It was ignored during this load; the caller shows Content in a
// confirm surface and calls Approve on acceptance, then reloads.
type PendingFile struct {
	Path    string
	Content string
}

// Result is a resolved configuration plus any repo files awaiting approval.
type Result struct {
	Config Config
	// RepoID identifies the repository for Approve. Empty when RepoDir is
	// unset or has no gated files.
	RepoID string
	// Pending lists gated repo config files that were skipped because the
	// repository is not approved.
	Pending []PendingFile
}

// Load resolves the three config layers. It never prompts: gated files in an
// unapproved repository are skipped and reported in Result.Pending.
func Load(src Source) (Result, error) {
	res := Result{Config: Default()}

	if src.GlobalDir != "" {
		if err := applyFile(&res.Config, filepath.Join(src.GlobalDir, GlobalFileName)); err != nil {
			return res, err
		}
	}
	if src.RepoDir == "" {
		return res, nil
	}

	repoPath := filepath.Join(src.RepoDir, RepoFileName)
	localPath := filepath.Join(src.RepoDir, LocalFileName)
	repoExists := fileExists(repoPath)
	localExists := fileExists(localPath)
	if !repoExists && !localExists {
		return res, nil
	}

	// A tracked .trunkr.local.toml can arrive via clone, so it is gated
	// exactly like the committed file.
	localGated := localExists && gitTracked(src.RepoDir, LocalFileName)

	var gated []string
	if repoExists {
		gated = append(gated, repoPath)
	}
	if localGated {
		gated = append(gated, localPath)
	}

	approved := false
	if len(gated) > 0 {
		res.RepoID = repoID(src.RepoDir)
		var err error
		approved, err = Approved(src.StateDir, res.RepoID)
		if err != nil {
			return res, err
		}
	}

	for _, path := range gated {
		if approved {
			if err := applyFile(&res.Config, path); err != nil {
				return res, err
			}
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return res, err
		}
		res.Pending = append(res.Pending, PendingFile{Path: path, Content: string(content)})
	}

	if localExists && !localGated {
		if err := applyFile(&res.Config, localPath); err != nil {
			return res, err
		}
	}
	return res, nil
}

// applyFile reads one TOML layer and overlays it. A missing file is not an
// error; unknown keys are, so typos fail loudly instead of silently doing
// nothing.
func applyFile(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var fc fileConfig
	md, err := toml.Decode(string(data), &fc)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, len(undecoded))
		for i, k := range undecoded {
			keys[i] = k.String()
		}
		return fmt.Errorf("%s: unknown key(s): %s", path, strings.Join(keys, ", "))
	}
	if err := fc.apply(cfg); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// repoID identifies a repository for approval keying: the git common dir, so
// one approval covers every worktree of the repo. Falls back to the resolved
// repo dir when git can't answer (not a repo, git missing).
func repoID(repoDir string) string {
	cmd := exec.Command("git", "rev-parse", "--path-format=absolute", "--git-common-dir")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err == nil {
		if dir := strings.TrimSpace(string(out)); dir != "" {
			return dir
		}
	}
	if abs, err := filepath.Abs(repoDir); err == nil {
		return abs
	}
	return repoDir
}

// gitTracked reports whether name is tracked by git in repoDir. Errors
// (untracked file, not a repo, git missing) all mean "not tracked".
func gitTracked(repoDir, name string) bool {
	cmd := exec.Command("git", "ls-files", "--error-unmatch", "--", name)
	cmd.Dir = repoDir
	return cmd.Run() == nil
}
