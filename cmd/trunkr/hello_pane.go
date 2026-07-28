package main

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/urfave/cli/v3"
)

// helloPaneCommand is the [[panes]] smoke test: a minimal Bubble Tea program,
// so the picker's TUI stack is proven inside a real herdr overlay pane.
func helloPaneCommand() *cli.Command {
	return &cli.Command{
		Name:  "hello-pane",
		Usage: "Bubble Tea smoke-test pane",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			_, err := tea.NewProgram(helloModel{}, tea.WithAltScreen()).Run()
			return err
		},
	}
}

type helloModel struct{}

func (helloModel) Init() tea.Cmd { return nil }

func (m helloModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "q", "esc", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (helloModel) View() string {
	title := lipgloss.NewStyle().Bold(true).Render("trunkr skeleton")
	dim := lipgloss.NewStyle().Faint(true)
	return fmt.Sprintf("%s\n\nBubble Tea is running inside a herdr plugin pane.\n%s\n%s\n\n%s\n",
		title,
		dim.Render("pane:      "+os.Getenv("HERDR_PANE_ID")),
		dim.Render("workspace: "+os.Getenv("HERDR_WORKSPACE_ID")),
		dim.Render("press q to close"),
	)
}
