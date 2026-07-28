---
trunkr: patch
---

Fix `agent_command` entries with multiple words (e.g. `["sh", "-c", "..."]`) being split into separate shell commands when launched in a pane
