package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hsldymq/smb_mount/internal/config"
)

// ListModel is the BubbleTea model for listing mount entries
type ListModel struct {
	Mounts   []config.MountEntry
	Cursor   int
	Scroll   int
	Height   int
	Width    int
	Quitting bool
	ShowHelp bool
}

// NewListModel creates a new list model
func NewListModel(mounts []config.MountEntry) ListModel {
	return ListModel{
		Mounts:   mounts,
		Cursor:   0,
		Scroll:   0,
		ShowHelp: true,
	}
}

// Init initializes the list model
func (m ListModel) Init() tea.Cmd {
	return nil
}

// Update handles messages for the list model
func (m ListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.Quitting = true
			return m, tea.Quit

		case "up", "k":
			if m.Cursor > 0 {
				m.Cursor--
			}

		case "down", "j":
			if m.Cursor < len(m.Mounts)-1 {
				m.Cursor++
			}

		case "home", "g":
			m.Cursor = 0

		case "end", "G":
			m.Cursor = len(m.Mounts) - 1
		}

		// Adjust scroll if needed
		m.adjustScroll()

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
	}

	return m, nil
}

// adjustScroll adjusts the scroll position to keep cursor visible
func (m *ListModel) adjustScroll() {
	// Assuming visible items = m.Height - header - footer - margins
	// Reserve space for header (5 lines) and footer (3 lines)
	visibleItems := m.Height - 8
	if visibleItems < 1 {
		visibleItems = 1
	}

	if m.Cursor < m.Scroll {
		m.Scroll = m.Cursor
	} else if m.Cursor >= m.Scroll+visibleItems {
		m.Scroll = m.Cursor - visibleItems + 1
	}

	// Clamp scroll
	if m.Scroll < 0 {
		m.Scroll = 0
	}
	maxScroll := len(m.Mounts) - visibleItems
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.Scroll > maxScroll {
		m.Scroll = maxScroll
	}
}

// View renders the list model
func (m ListModel) View() string {
	if m.Quitting {
		return ""
	}

	var b strings.Builder

	// Title
	b.WriteString(TitleStyle.Render("SMB Mount Configuration"))
	b.WriteString("\n\n")

	// Build table header
	b.WriteString(m.renderHeader())
	b.WriteString("\n")

	// Build table rows
	b.WriteString(m.renderRows())
	b.WriteString("\n")

	// Summary
	b.WriteString("\n")
	b.WriteString(m.renderSummary())

	// Help text
	if m.ShowHelp {
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("↑/k: up | ↓/j: down | g/home: top | G/end: bottom | q/esc: quit"))
	}

	return b.String()
}

// renderHeader renders the table header
func (m ListModel) renderHeader() string {
	// Calculate column widths based on terminal width
	nameWidth := 20
	addrWidth := 20
	pathWidth := max(m.Width-nameWidth-addrWidth-20, 30)

	header := fmt.Sprintf("%-*s  %-*s  %-*s  %s",
		nameWidth, "NAME",
		addrWidth, "SMB ADDRESS",
		pathWidth, "MOUNT PATH",
		"STATUS",
	)

	return TableHeaderStyle.Render(header)
}

// renderRows renders the table rows
func (m ListModel) renderRows() string {
	var b strings.Builder

	visibleItems := m.Height - 8
	if visibleItems < 1 {
		visibleItems = 1
	}

	end := min(m.Scroll+visibleItems, len(m.Mounts))

	for i := m.Scroll; i < end; i++ {
		row := m.renderRow(i, m.Mounts[i])
		if i == m.Cursor {
			b.WriteString(SelectedItemStyle.Render(row))
		} else {
			b.WriteString(NormalItemStyle.Render(row))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// renderRow renders a single row
func (m ListModel) renderRow(index int, entry config.MountEntry) string {
	// Calculate column widths
	nameWidth := 20
	addrWidth := 20
	pathWidth := max(m.Width-nameWidth-addrWidth-20, 30)

	// Truncate values if too long
	name := truncate(entry.Name, nameWidth)
	addr := truncate(fmt.Sprintf("%s:%d", entry.SMBAddr, entry.GetSMBPort()), addrWidth)
	path := truncate(entry.ActualMountPath, pathWidth)

	// Build row
	row := fmt.Sprintf("%-*s  %-*s  %-*s  %s",
		nameWidth, name,
		addrWidth, addr,
		pathWidth, path,
		RenderStatusBadge(entry.IsMounted),
	)

	return row
}

// renderSummary renders a summary of mount status
func (m ListModel) renderSummary() string {
	mounted := 0
	for _, m := range m.Mounts {
		if m.IsMounted {
			mounted++
		}
	}

	total := len(m.Mounts)
	summary := fmt.Sprintf("Total: %d | Mounted: %d | Unmounted: %d",
		total, mounted, total-mounted)

	return SubtitleStyle.Render(summary)
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// DisplayList shows the list TUI and returns when user quits
func DisplayList(mounts []config.MountEntry) error {
	model := NewListModel(mounts)
	program := tea.NewProgram(model)

	finalModel, err := program.Run()
	if err != nil {
		return fmt.Errorf("failed to run list TUI: %w", err)
	}

	_ = finalModel // Model is discarded, just displaying info
	return nil
}
