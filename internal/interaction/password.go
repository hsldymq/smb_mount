package interaction

import (
    "fmt"
    "strings"

    tea "github.com/charmbracelet/bubbletea"
)

// PasswordModel 密码输入的 BubbleTea 模型
type PasswordModel struct {
    Prompt       string
    Password     []rune
    ShowAsterisk bool // If true, show asterisks instead of hiding completely
    Quitting     bool
    Err          error
    Width        int
}

// NewPasswordModel 创建新的密码输入模型
func NewPasswordModel(prompt string, showAsterisk bool) PasswordModel {
    return PasswordModel{
        Prompt:       prompt,
        ShowAsterisk: showAsterisk,
    }
}

// Init initializes the model
func (m PasswordModel) Init() tea.Cmd {
    return nil
}

// Update handles messages
func (m PasswordModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.Type {
        case tea.KeyEnter, tea.KeyCtrlC:
            m.Quitting = true
            return m, tea.Quit

        case tea.KeyBackspace:
            if len(m.Password) > 0 {
                m.Password = m.Password[:len(m.Password)-1]
            }
            return m, nil

        case tea.KeyCtrlD: // Delete key
            if len(m.Password) > 0 {
                m.Password = m.Password[:0]
            }
            return m, nil

        default:
            // Only accept regular characters
            if msg.Type == tea.KeyRunes {
                for _, r := range msg.Runes {
                    // Filter out control characters
                    if r >= 32 && r != 127 {
                        m.Password = append(m.Password, r)
                    }
                }
            }
            return m, nil
        }

    case tea.WindowSizeMsg:
        m.Width = msg.Width
        return m, nil
    }

    return m, nil
}

// View renders the password input
func (m PasswordModel) View() string {
    if m.Quitting {
        return ""
    }

    var displayPassword string
    if m.ShowAsterisk {
        displayPassword = strings.Repeat("*", len(m.Password))
    } else {
        displayPassword = strings.Repeat("•", len(m.Password))
    }

    return fmt.Sprintf("%s%s", m.Prompt, displayPassword)
}

// GetPassword 返回输入的密码
func (m PasswordModel) GetPassword() string {
    return string(m.Password)
}

// PromptPassword 使用 BubbleTea 提示用户输入密码
func PromptPassword(promptText string, showAsterisk bool) (string, error) {
    if promptText == "" {
        promptText = "Enter password: "
    }

    model := NewPasswordModel(promptText, showAsterisk)
    program := tea.NewProgram(model)

    finalModel, err := program.Run()
    if err != nil {
        return "", fmt.Errorf("failed to run password prompt: %w", err)
    }

    m, ok := finalModel.(PasswordModel)
    if !ok {
        return "", fmt.Errorf("unexpected model type")
    }

    if m.Err != nil {
        return "", m.Err
    }

    return m.GetPassword(), nil
}
