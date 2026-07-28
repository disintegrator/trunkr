---
trunkr: patch
---

Fix actions failing on fresh installs: the dev `mise.toml` is now removed from installed checkouts, so mise-shimmed tools (like `wt`) no longer hard-error when run from the plugin root
