package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// writeFile writes content at dir/name, creating dir as needed.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// initRepo makes dir a git repository.
func initRepo(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "t@t.dev"},
		{"config", "user.name", "t"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func gitTrack(t *testing.T, dir string, names ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"add", "--"}, names...)...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
}

func TestLoadLayering(t *testing.T) {
	tests := []struct {
		name    string
		global  string // trunkr.toml content; "" means no file
		repo    string // .trunkr.toml content (repo pre-approved)
		local   string // .trunkr.local.toml content, untracked
		want    func(*Config)
		wantErr string
	}{
		{
			name: "no files yields defaults",
			want: func(c *Config) {},
		},
		{
			name:   "global sets knobs",
			global: "agent_command = [\"claude\"]\ncontainer = \"split\"\nwt_path = \"/opt/wt\"\n",
			want: func(c *Config) {
				c.AgentCommand = []string{"claude"}
				c.Container = ContainerSplit
				c.WTPath = "/opt/wt"
			},
		},
		{
			name:   "repo overrides global, unset knobs survive",
			global: "agent_command = [\"claude\"]\ncontainer = \"split\"\n",
			repo:   "container = \"workspace\"\n",
			want: func(c *Config) {
				c.AgentCommand = []string{"claude"}
				c.Container = ContainerWorkspace
			},
		},
		{
			name:  "local overrides repo",
			repo:  "container = \"workspace\"\n[merge]\nextra_args = [\"--no-remove\"]\n",
			local: "container = \"tab\"\n",
			want: func(c *Config) {
				c.MergeExtraArgs = []string{"--no-remove"}
			},
		},
		{
			name:   "empty agent_command in later layer overrides earlier",
			global: "agent_command = [\"claude\"]\n",
			local:  "agent_command = []\n",
			want: func(c *Config) {
				c.AgentCommand = []string{}
			},
		},
		{
			name:   "picker keys merge per key over defaults",
			global: "[picker.keys]\nmerge = \"M\"\n",
			local:  "[picker.keys]\ndestroy = \"x\"\n",
			want: func(c *Config) {
				c.PickerKeys["merge"] = "M"
				c.PickerKeys["destroy"] = "x"
			},
		},
		{
			name:    "invalid container rejected",
			global:  "container = \"window\"\n",
			wantErr: "invalid container",
		},
		{
			name:    "unknown key rejected",
			global:  "agent_cmd = [\"claude\"]\n",
			wantErr: "unknown key",
		},
		{
			name:    "unknown picker action rejected",
			global:  "[picker.keys]\nmerg = \"m\"\n",
			wantErr: "unknown picker action",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			globalDir := t.TempDir()
			stateDir := t.TempDir()
			repoDir := t.TempDir()
			initRepo(t, repoDir)
			if tt.global != "" {
				writeFile(t, globalDir, GlobalFileName, tt.global)
			}
			if tt.repo != "" {
				writeFile(t, repoDir, RepoFileName, tt.repo)
				if err := Approve(stateDir, repoID(repoDir)); err != nil {
					t.Fatal(err)
				}
			}
			if tt.local != "" {
				writeFile(t, repoDir, LocalFileName, tt.local)
			}

			res, err := Load(Source{GlobalDir: globalDir, StateDir: stateDir, RepoDir: repoDir})
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("want error containing %q, got %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Pending) != 0 {
				t.Fatalf("unexpected pending files: %v", res.Pending)
			}
			want := Default()
			tt.want(&want)
			if !reflect.DeepEqual(res.Config, want) {
				t.Errorf("config mismatch\n got: %+v\nwant: %+v", res.Config, want)
			}
		})
	}
}

func TestLoadApprovalGate(t *testing.T) {
	t.Run("unapproved repo file is pending and skipped", func(t *testing.T) {
		stateDir := t.TempDir()
		repoDir := t.TempDir()
		initRepo(t, repoDir)
		writeFile(t, repoDir, RepoFileName, "container = \"split\"\n")

		res, err := Load(Source{StateDir: stateDir, RepoDir: repoDir})
		if err != nil {
			t.Fatal(err)
		}
		if res.Config.Container != ContainerTab {
			t.Errorf("unapproved file applied: container = %q", res.Config.Container)
		}
		if len(res.Pending) != 1 {
			t.Fatalf("want 1 pending file, got %d", len(res.Pending))
		}
		if !strings.Contains(res.Pending[0].Content, "split") {
			t.Errorf("pending content not surfaced: %q", res.Pending[0].Content)
		}
		if res.RepoID == "" {
			t.Error("RepoID empty; caller cannot Approve")
		}

		// Approving via the surfaced RepoID makes the file apply.
		if err := Approve(stateDir, res.RepoID); err != nil {
			t.Fatal(err)
		}
		res, err = Load(Source{StateDir: stateDir, RepoDir: repoDir})
		if err != nil {
			t.Fatal(err)
		}
		if res.Config.Container != ContainerSplit || len(res.Pending) != 0 {
			t.Errorf("approved file not applied: %+v pending=%v", res.Config, res.Pending)
		}
	})

	t.Run("tracked local file is gated like the repo file", func(t *testing.T) {
		stateDir := t.TempDir()
		repoDir := t.TempDir()
		initRepo(t, repoDir)
		writeFile(t, repoDir, LocalFileName, "container = \"split\"\n")
		gitTrack(t, repoDir, LocalFileName)

		res, err := Load(Source{StateDir: stateDir, RepoDir: repoDir})
		if err != nil {
			t.Fatal(err)
		}
		if res.Config.Container != ContainerTab {
			t.Errorf("tracked local file applied without approval: container = %q", res.Config.Container)
		}
		if len(res.Pending) != 1 {
			t.Fatalf("want 1 pending file, got %d", len(res.Pending))
		}
	})

	t.Run("untracked local file applies without approval", func(t *testing.T) {
		stateDir := t.TempDir()
		repoDir := t.TempDir()
		initRepo(t, repoDir)
		writeFile(t, repoDir, LocalFileName, "container = \"split\"\n")

		res, err := Load(Source{StateDir: stateDir, RepoDir: repoDir})
		if err != nil {
			t.Fatal(err)
		}
		if res.Config.Container != ContainerSplit {
			t.Errorf("untracked local file not applied: container = %q", res.Config.Container)
		}
		if len(res.Pending) != 0 {
			t.Errorf("unexpected pending: %v", res.Pending)
		}
	})

	t.Run("approval covers every worktree of the repo", func(t *testing.T) {
		stateDir := t.TempDir()
		repoDir := t.TempDir()
		initRepo(t, repoDir)
		writeFile(t, repoDir, "README.md", "hi")
		gitTrack(t, repoDir, "README.md")
		commit := exec.Command("git", "commit", "-qm", "init")
		commit.Dir = repoDir
		if out, err := commit.CombinedOutput(); err != nil {
			t.Fatalf("git commit: %v\n%s", err, out)
		}
		treeDir := filepath.Join(t.TempDir(), "tree")
		wtAdd := exec.Command("git", "worktree", "add", "-q", "-b", "feat", treeDir)
		wtAdd.Dir = repoDir
		if out, err := wtAdd.CombinedOutput(); err != nil {
			t.Fatalf("git worktree add: %v\n%s", err, out)
		}

		if repoID(repoDir) != repoID(treeDir) {
			t.Fatalf("repo identity differs across worktrees: %q vs %q", repoID(repoDir), repoID(treeDir))
		}

		writeFile(t, treeDir, RepoFileName, "container = \"split\"\n")
		if err := Approve(stateDir, repoID(repoDir)); err != nil {
			t.Fatal(err)
		}
		res, err := Load(Source{StateDir: stateDir, RepoDir: treeDir})
		if err != nil {
			t.Fatal(err)
		}
		if res.Config.Container != ContainerSplit {
			t.Errorf("approval on the main repo did not cover the worktree")
		}
	})
}

func TestApprovalStore(t *testing.T) {
	stateDir := t.TempDir()
	if ok, err := Approved(stateDir, "repo-a"); err != nil || ok {
		t.Fatalf("empty store: got ok=%v err=%v", ok, err)
	}
	if err := Approve(stateDir, "repo-a"); err != nil {
		t.Fatal(err)
	}
	if err := Approve(stateDir, "repo-b"); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]bool{"repo-a": true, "repo-b": true, "repo-c": false} {
		ok, err := Approved(stateDir, id)
		if err != nil {
			t.Fatal(err)
		}
		if ok != want {
			t.Errorf("Approved(%q) = %v, want %v", id, ok, want)
		}
	}
	if err := Approve("", "repo-a"); err == nil {
		t.Error("Approve with empty state dir should fail")
	}
	if ok, err := Approved("", "repo-a"); err != nil || ok {
		t.Errorf("empty state dir should read as unapproved, got ok=%v err=%v", ok, err)
	}
}
