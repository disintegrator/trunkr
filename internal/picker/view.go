package picker

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle   = lipgloss.NewStyle().Bold(true)
	dimStyle     = lipgloss.NewStyle().Faint(true)
	cursorStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	trunkStyle   = lipgloss.NewStyle().Faint(true)
	dirtyStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	aheadStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	behindStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	msgStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	promptStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5"))
	helpStyle    = lipgloss.NewStyle().Faint(true)
	statusStyles = map[string]lipgloss.Style{
		"blocked": lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true),
		"working": lipgloss.NewStyle().Foreground(lipgloss.Color("3")),
		"done":    lipgloss.NewStyle().Foreground(lipgloss.Color("2")),
		"idle":    lipgloss.NewStyle().Faint(true),
		"unknown": lipgloss.NewStyle().Faint(true),
	}
	statusGlyphs = map[string]string{
		"blocked": "■", "working": "●", "done": "✓", "idle": "○", "unknown": "?",
	}
)

func (m Model) View() string {
	var b strings.Builder
	title := titleStyle.Render("trunkr — worktrees")
	if m.repoName != "" {
		title += dimStyle.Render("  (" + m.repoName + ")")
	}
	b.WriteString(title + "\n\n")

	visible := m.visible()
	if len(m.rows) == 0 {
		b.WriteString(dimStyle.Render("  no worktrees — press "+m.keyFor("create")+" to create one") + "\n")
	} else {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  %-22s %-6s %-10s %s", "BRANCH", "PANES", "AGENTS", "GIT")) + "\n")
		if len(visible) == 0 {
			b.WriteString(dimStyle.Render("  no branches match the filter") + "\n")
		}
	}

	for i, r := range visible {
		cursor := "  "
		if i == m.cursor && (m.mode == modeList || m.mode == modeConfirm) {
			cursor = cursorStyle.Render("▸ ")
		}

		branch := r.Branch
		if r.IsTrunk {
			branch = trunkStyle.Render(branch + " (trunk)")
		}
		line := fmt.Sprintf("%-22s", branch)

		panes := "–"
		if r.Panes > 0 {
			panes = fmt.Sprintf("%d", r.Panes)
		}
		line += fmt.Sprintf(" %-6s", panes)

		agents := dimStyle.Render("–") + strings.Repeat(" ", 9)
		if s := Rollup(r.Agents); s != "" {
			agents = statusStyles[s].Render(fmt.Sprintf("%s %-8s", statusGlyphs[s], s))
		}
		line += " " + agents

		var git []string
		if r.Ahead > 0 {
			git = append(git, aheadStyle.Render(fmt.Sprintf("+%d", r.Ahead)))
		}
		if r.Behind > 0 {
			git = append(git, behindStyle.Render(fmt.Sprintf("−%d", r.Behind)))
		}
		if r.Dirty {
			git = append(git, dirtyStyle.Render("~dirty"))
		}
		if len(git) == 0 {
			git = append(git, dimStyle.Render("clean"))
		}
		line += " " + strings.Join(git, " ")

		b.WriteString(cursor + line + "\n")
	}

	b.WriteString("\n")
	switch m.mode {
	case modeFilter:
		b.WriteString(promptStyle.Render("filter: ") + m.filter + "█\n")
	case modeInput:
		b.WriteString(promptStyle.Render(m.prompt) + m.input + "█\n")
	case modeConfirm:
		b.WriteString(promptStyle.Render(m.prompt) + "\n")
	default:
		if m.status != "" {
			b.WriteString(msgStyle.Render("→ "+m.status) + "\n")
		} else {
			b.WriteString("\n")
		}
	}

	b.WriteString(helpStyle.Render("\n" + m.helpLine() + "\n"))
	return b.String()
}
