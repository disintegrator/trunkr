// Package config loads trunkr's three-layer TOML configuration:
//
//  1. trunkr.toml in HERDR_PLUGIN_CONFIG_DIR — the user's global defaults
//  2. .trunkr.toml committed in the repo — team-shared, approval-gated
//  3. .trunkr.local.toml in the repo, untracked — the user's per-repo overrides
//
// Later layers win per knob. A repo's committed file is approved once per
// repository (keyed by the git common dir, so one approval covers every
// worktree); an unapproved file is skipped and surfaced as pending so the
// caller can prompt. A git-tracked .trunkr.local.toml is gated like the
// committed file, so a repo can't smuggle ungated config.
package config

import (
	"fmt"
	"maps"
	"sort"
)

// Container is where the generic switch action opens a worktree's new pane.
type Container string

const (
	ContainerTab       Container = "tab"
	ContainerWorkspace Container = "workspace"
	ContainerSplit     Container = "split"
)

// PickerActions are the picker bindings that [picker.keys] may remap. The
// remaining picker keys (enter, /, r, q/esc) are fixed.
var pickerKeyDefaults = map[string]string{
	"tab":       "t",
	"workspace": "w",
	"split":     "s",
	"create":    "c",
	"pr":        "p",
	"merge":     "m",
	"destroy":   "d",
}

// Config is the fully resolved configuration all action slices consume.
type Config struct {
	// AgentCommand is the argv run in new worktree panes. Empty means a
	// plain shell.
	AgentCommand []string
	// Container is the generic switch action's target: tab, workspace, or
	// split.
	Container Container
	// WTPath overrides where the wt binary is found. Empty means PATH
	// lookup.
	WTPath string
	// MergeExtraArgs are appended to every wt merge invocation.
	MergeExtraArgs []string
	// PickerKeys maps picker action names to their key, defaults already
	// applied.
	PickerKeys map[string]string
}

// Default returns the configuration used when no file sets a knob.
func Default() Config {
	keys := make(map[string]string, len(pickerKeyDefaults))
	maps.Copy(keys, pickerKeyDefaults)
	return Config{
		Container:  ContainerTab,
		PickerKeys: keys,
	}
}

// fileConfig is one layer as read from disk. Pointers distinguish "unset"
// from "set to the zero value" so later layers only override what they name.
type fileConfig struct {
	AgentCommand *[]string     `toml:"agent_command"`
	Container    *string       `toml:"container"`
	WTPath       *string       `toml:"wt_path"`
	Merge        *mergeConfig  `toml:"merge"`
	Picker       *pickerConfig `toml:"picker"`
}

type mergeConfig struct {
	ExtraArgs *[]string `toml:"extra_args"`
}

type pickerConfig struct {
	Keys map[string]string `toml:"keys"`
}

// apply overlays one layer onto cfg. Picker keys merge per key, everything
// else replaces wholesale.
func (fc fileConfig) apply(cfg *Config) error {
	if fc.AgentCommand != nil {
		cfg.AgentCommand = *fc.AgentCommand
	}
	if fc.Container != nil {
		c := Container(*fc.Container)
		switch c {
		case ContainerTab, ContainerWorkspace, ContainerSplit:
			cfg.Container = c
		default:
			return fmt.Errorf("invalid container %q: must be tab, workspace, or split", *fc.Container)
		}
	}
	if fc.WTPath != nil {
		cfg.WTPath = *fc.WTPath
	}
	if fc.Merge != nil && fc.Merge.ExtraArgs != nil {
		cfg.MergeExtraArgs = *fc.Merge.ExtraArgs
	}
	if fc.Picker != nil {
		var unknown []string
		for action, key := range fc.Picker.Keys {
			if _, ok := pickerKeyDefaults[action]; !ok {
				unknown = append(unknown, action)
				continue
			}
			cfg.PickerKeys[action] = key
		}
		if len(unknown) > 0 {
			sort.Strings(unknown)
			return fmt.Errorf("unknown picker action(s) in [picker.keys]: %v", unknown)
		}
	}
	return nil
}
