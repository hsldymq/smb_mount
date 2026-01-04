package tui

import (
    "github.com/charmbracelet/lipgloss"
)

// Color palette
var (
    // Primary colors
    primaryColor   = lipgloss.Color("86")  // Cyan
    secondaryColor = lipgloss.Color("236") // Dark gray
    accentColor    = lipgloss.Color("212") // Pink/purple

    // Status colors
    successColor = lipgloss.Color("86")  // Green/cyan
    errorColor   = lipgloss.Color("196") // Red
    warningColor = lipgloss.Color("226") // Yellow
    dimColor     = lipgloss.Color("241") // Dim gray

    // Text colors
    titleColor  = lipgloss.Color("212") // Pink/purple
    subtleColor = lipgloss.Color("243") // Subtle gray
)

// Base styles
var (
    // Title style
    TitleStyle = lipgloss.NewStyle().
        Bold(true).
        Foreground(titleColor).
        MarginTop(1).
        MarginBottom(1)

    // Subtitle style
    SubtitleStyle = lipgloss.NewStyle().
        Foreground(subtleColor).
        MarginBottom(1)

    // Header style
    HeaderStyle = lipgloss.NewStyle().
        Bold(true).
        Foreground(primaryColor)

    // Status indicator styles
    MountedStyle = lipgloss.NewStyle().
        Foreground(successColor).
        Bold(true)

    UnmountedStyle = lipgloss.NewStyle().
        Foreground(dimColor)

    // Error style
    ErrorStyle = lipgloss.NewStyle().
        Foreground(errorColor).
        Bold(true)

    // Warning style
    WarningStyle = lipgloss.NewStyle().
        Foreground(warningColor)

    // Success style
    SuccessStyle = lipgloss.NewStyle().
        Foreground(successColor).
        Bold(true)

    // Border styles
    BorderStyle = lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(primaryColor)

    // List item styles
    NormalItemStyle = lipgloss.NewStyle().
        PaddingLeft(1).
        PaddingRight(1)

    SelectedItemStyle = lipgloss.NewStyle().
        PaddingLeft(1).
        PaddingRight(1).
        Foreground(primaryColor).
        Background(lipgloss.Color("236"))

    // Cursor style
    CursorStyle = lipgloss.NewStyle().
        Foreground(accentColor).
        Bold(true)

    // Dimmed text
    DimStyle = lipgloss.NewStyle().
        Foreground(dimColor)

    // Help text style
    HelpStyle = lipgloss.NewStyle().
        Foreground(subtleColor).
        MarginTop(1)
)

// Component styles
var (
    // Status badge
    StatusBadgeMounted = lipgloss.NewStyle().
        Foreground(lipgloss.Color("42")). // Green
        Bold(true).
        Padding(0, 1).
        Background(lipgloss.Color("236"))

    StatusBadgeUnmounted = lipgloss.NewStyle().
        Foreground(dimColor).
        Bold(true).
        Padding(0, 1).
        Background(lipgloss.Color("236"))

    // Table row styles
    TableRowStyle = lipgloss.NewStyle().
        Padding(0, 1)

    TableHeaderStyle = lipgloss.NewStyle().
        Bold(true).
        Foreground(primaryColor).
        Padding(0, 1).
        MarginBottom(1)

    // Info field labels
    LabelStyle = lipgloss.NewStyle().
        Foreground(primaryColor).
        Bold(true).
        MarginRight(1)

    // Info field values
    ValueStyle = lipgloss.NewStyle().
        Foreground(lipgloss.Color("255"))

    // Command hint
    CommandHintStyle = lipgloss.NewStyle().
        Foreground(subtleColor).
        Italic(true)
)

// RenderStatusBadge returns a styled status badge
func RenderStatusBadge(isMounted bool) string {
    if isMounted {
        return StatusBadgeMounted.Render("MOUNTED")
    }
    return StatusBadgeUnmounted.Render("UNMOUNTED")
}
