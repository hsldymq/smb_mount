package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hsldymq/smb_mount/internal/config"
)

// SelectionResult 是选择的结果
type SelectionResult struct {
	Entries []*config.MountEntry
	Cancel  bool
}

// SelectorModel 交互式选择的 BubbleTea 模型
type SelectorModel struct {
	Title       string
	Mounts      []config.MountEntry
	Cursor      int
	Scroll      int
	Selected    []*config.MountEntry
	SelectedMap map[int]bool // 追踪选中的条目索引
	Cancelled   bool
	Height      int
	Width       int
	ShowStatus  bool
}

// NewSelectorModel 创建新的选择器模型
func NewSelectorModel(title string, mounts []config.MountEntry, showStatus bool) SelectorModel {
	return SelectorModel{
		Title:       title,
		Mounts:      mounts,
		Cursor:      0,
		Scroll:      0,
		SelectedMap: make(map[int]bool),
		ShowStatus:  showStatus,
	}
}

// Init 初始化选择器模型
func (m SelectorModel) Init() tea.Cmd {
	return nil
}

// Update 处理选择器模型的消息
func (m SelectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.Cancelled = true
			return m, tea.Quit

		case "enter":
			// 收集所有选中的条目
			m.Selected = nil
			for i := range m.Mounts {
				if m.SelectedMap[i] {
					m.Selected = append(m.Selected, &m.Mounts[i])
				}
			}
			// 如果没有选中任何条目，且当前有可用条目，则选中当前光标所在的条目
			if len(m.Selected) == 0 && len(m.Mounts) > 0 {
				m.Selected = []*config.MountEntry{&m.Mounts[m.Cursor]}
			}
			return m, tea.Quit

		case " ":
			// 空格键切换选中状态
			if len(m.Mounts) > 0 {
				if m.SelectedMap[m.Cursor] {
					delete(m.SelectedMap, m.Cursor)
				} else {
					m.SelectedMap[m.Cursor] = true
				}
			}

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

		m.adjustScroll()

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
	}

	return m, nil
}

// adjustScroll 调整滚动位置以保持光标可见
func (m *SelectorModel) adjustScroll() {
	// 为标题（4行）和帮助（3行）预留空间
	visibleItems := m.Height - 7
	if visibleItems < 1 {
		visibleItems = 1
	}

	if m.Cursor < m.Scroll {
		m.Scroll = m.Cursor
	} else if m.Cursor >= m.Scroll+visibleItems {
		m.Scroll = m.Cursor - visibleItems + 1
	}

	// 限制滚动范围
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

// View 渲染选择器模型
func (m SelectorModel) View() string {
	if m.Cancelled || len(m.Selected) > 0 {
		return ""
	}

	var b strings.Builder

	// 标题
	if m.Title != "" {
		b.WriteString(TitleStyle.Render(m.Title))
		b.WriteString("\n\n")
	}

	// 构建条目
	b.WriteString(m.renderItems())

	// 帮助文本
	b.WriteString("\n\n")
	b.WriteString(HelpStyle.Render("↑/k: up | ↓/j: down | space: select | enter: confirm | q/esc: cancel"))

	return b.String()
}

// renderItems 渲染可选择的条目
func (m SelectorModel) renderItems() string {
	if len(m.Mounts) == 0 {
		return DimStyle.Render("No mount entries available")
	}

	var b strings.Builder

	visibleItems := m.Height - 7
	if visibleItems < 1 {
		visibleItems = 1
	}

	end := min(m.Scroll+visibleItems, len(m.Mounts))

	for i := m.Scroll; i < end; i++ {
		item := m.renderItem(i, m.Mounts[i])
		cursor := " "
		if i == m.Cursor {
			cursor = "▸"
			b.WriteString(SelectedItemStyle.Render(fmt.Sprintf("%s %s", cursor, item)))
		} else {
			b.WriteString(NormalItemStyle.Render(fmt.Sprintf("%s %s", cursor, item)))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// renderItem 渲染单个条目
func (m SelectorModel) renderItem(index int, entry config.MountEntry) string {
	// 显示选中标记
	checkbox := "[ ]"
	if m.SelectedMap[index] {
		checkbox = "[✓]"
	}

	var parts []string
	parts = append(parts, checkbox)

	// 名称
	parts = append(parts, fmt.Sprintf("%s", entry.Name))

	// SMB 地址
	parts = append(parts, fmt.Sprintf("(%s:%d/%s)",
		entry.SMBAddr, entry.GetSMBPort(), entry.ShareName))

	// 状态（如果启用）
	if m.ShowStatus {
		status := "Unmounted"
		if entry.IsMounted {
			status = "Mounted"
		}
		parts = append(parts, fmt.Sprintf("[%s]", status))
	}

	return strings.Join(parts, " ")
}

// SelectEntry 显示选择器并返回选中的条目（支持多选）
func SelectEntry(title string, mounts []config.MountEntry, showStatus bool) ([]*config.MountEntry, bool) {
	model := NewSelectorModel(title, mounts, showStatus)
	program := tea.NewProgram(model)

	finalModel, err := program.Run()
	if err != nil {
		return nil, false
	}

	m, ok := finalModel.(SelectorModel)
	if !ok {
		return nil, false
	}

	if m.Cancelled {
		return nil, true
	}

	return m.Selected, false
}

// SelectMountEntry 显示挂载选择器（支持多选）
func SelectMountEntry(mounts []config.MountEntry) ([]*config.MountEntry, bool) {
	return SelectEntry("Select shares to mount:", mounts, true)
}

// SelectUnmountEntry 显示卸载选择器（支持多选）
func SelectUnmountEntry(mounts []config.MountEntry) ([]*config.MountEntry, bool) {
	// 只显示已挂载的条目
	var mounted []config.MountEntry
	for _, m := range mounts {
		if m.IsMounted {
			mounted = append(mounted, m)
		}
	}

	if len(mounted) == 0 {
		return nil, false
	}

	return SelectEntry("Select shares to unmount:", mounted, true)
}
