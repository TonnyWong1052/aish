package ui

import (
    "fmt"
    "io"
    "strings"

    "github.com/charmbracelet/bubbles/key"
    "github.com/charmbracelet/bubbles/textinput"
    "github.com/charmbracelet/bubbles/list"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"

    "github.com/TonnyWong1052/aish/internal/config"
)

// KeyMap defines the key bindings for the settings TUI
type KeyMap struct {
	Up         key.Binding
	Down       key.Binding
	Left       key.Binding
	Right      key.Binding
	Enter      key.Binding
	Space      key.Binding
	Tab        key.Binding
	ShiftTab   key.Binding
	Quit       key.Binding
	Help       key.Binding
}

// DefaultKeyMap returns the default key bindings
func DefaultKeyMap() KeyMap {
    return KeyMap{
        Up: key.NewBinding(
            key.WithKeys("up", "k"),
            key.WithHelp("↑/k", "up"),
        ),
        Down: key.NewBinding(
            key.WithKeys("down", "j"),
            key.WithHelp("↓/j", "down"),
        ),
        Left: key.NewBinding(
            key.WithKeys("left", "h"),
            key.WithHelp("←/h", "previous option"),
        ),
        Right: key.NewBinding(
            key.WithKeys("right", "l"),
            key.WithHelp("→/l", "next option"),
        ),
        Enter: key.NewBinding(
            key.WithKeys("enter"),
            key.WithHelp("enter", "confirm/execute"),
        ),
        Space: key.NewBinding(
            key.WithKeys(" "),
            key.WithHelp("space", "toggle"),
        ),
        Tab: key.NewBinding(
            key.WithKeys("tab"),
            key.WithHelp("tab", "next item"),
        ),
        ShiftTab: key.NewBinding(
            key.WithKeys("shift+tab"),
            key.WithHelp("shift+tab", "previous item"),
        ),
        Quit: key.NewBinding(
            key.WithKeys("q", "esc"),
            key.WithHelp("q/esc", "quit"),
        ),
        Help: key.NewBinding(
            key.WithKeys("?"),
            key.WithHelp("?", "help"),
        ),
    }
}

// settingsItem wraps SettingItem for list.Item interface
type settingsItem struct {
	*SettingItem
	config *config.Config
}

func (i settingsItem) FilterValue() string { return i.DisplayName }

// SettingsModel is the main model for the settings TUI
type SettingsModel struct {
    list       list.Model
    keys       KeyMap
    config     *config.Config
    settings   []*SettingItem
    width      int
    height     int
    showDetail bool
    message    string
    // ensure the cursor initially lands on the first interactive item
    selectionInitialized bool
    // text editing state
    isEditing   bool
    editingItem *SettingItem
    textInput   textinput.Model

    // inline multi-select state for error triggers（整合式多選，不再呼叫外部直寫 Stdout 的函式）
    multiActive  bool
    multiPrompt  string
    multiOptions []string
    multiSelected []bool
    multiCursor  int
}

// findFirstInteractiveItem finds the index of the first interactive setting item
func findFirstInteractiveItem(settings []*SettingItem) int {
	for i, setting := range settings {
		switch setting.Type {
		case SettingTypeBoolean, SettingTypeSelect, SettingTypeAction:
			return i
		}
	}
	return 0 // fallback to first item if no interactive item found
}

// NewSettingsModel creates a new settings model
func NewSettingsModel(cfg *config.Config) *SettingsModel {
	settings := GetSettingsDefinition(cfg)
	
	items := make([]list.Item, len(settings))
	for i, setting := range settings {
		items[i] = settingsItem{SettingItem: setting, config: cfg}
	}

	l := list.New(items, itemDelegate{}, 0, 0)
	// Clean list setup - no title, no status bar, no help
	l.Title = ""
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowTitle(false)

	// Custom list styles
	l.Styles.Title = lipgloss.NewStyle()
	l.Styles.PaginationStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	l.Styles.HelpStyle = lipgloss.NewStyle()

    // Try to position the cursor on the first interactive item up-front.
    // Some terminals/UI backends may override this after the first size event,
    // so we also enforce it once on the first WindowSizeMsg (see Update).
    firstInteractiveIndex := findFirstInteractiveItem(settings)
    if firstInteractiveIndex > 0 {
        l.Select(firstInteractiveIndex)
    }

    return &SettingsModel{
        list:     l,
        keys:     DefaultKeyMap(),
        config:   cfg,
        settings: settings,
        selectionInitialized: firstInteractiveIndex == 0, // set true if already at 0 (no adjustment needed later)
    }
}

// Init implements tea.Model
func (m *SettingsModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model
func (m *SettingsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        m.list.SetWidth(msg.Width)
        m.list.SetHeight(msg.Height - 3) // Leave space for status line

        // After the first size event, enforce selection on the first interactive item
        // if we haven't done so yet (skips non-interactive headers like group/info).
        if !m.selectionInitialized {
            idx := findFirstInteractiveItem(m.settings)
            if idx > 0 {
                m.list.Select(idx)
            }
            m.selectionInitialized = true
        }

    case tea.KeyMsg:
        // 當多選面板開啟時，攔截按鍵事件處理
        if m.multiActive {
            switch msg.Type {
            case tea.KeyUp, tea.KeyCtrlP:
                if m.multiCursor > 0 { m.multiCursor-- } else { m.multiCursor = len(m.multiOptions) - 1 }
                return m, nil
            case tea.KeyDown, tea.KeyCtrlN:
                if m.multiCursor < len(m.multiOptions)-1 { m.multiCursor++ } else { m.multiCursor = 0 }
                return m, nil
            case tea.KeyRunes:
                if len(msg.Runes) == 1 {
                    switch msg.Runes[0] {
                    case 'a':
                        // 全選/全不選切換
                        all := true
                        for _, v := range m.multiSelected { if !v { all = false; break } }
                        for i := range m.multiSelected { m.multiSelected[i] = !all }
                        return m, nil
                    case 'i':
                        // 反選
                        for i := range m.multiSelected { m.multiSelected[i] = !m.multiSelected[i] }
                        return m, nil
                    }
                }
            case tea.KeySpace:
                if len(m.multiSelected) > 0 {
                    m.multiSelected[m.multiCursor] = !m.multiSelected[m.multiCursor]
                }
                return m, nil
            case tea.KeyEnter:
                // 套用選取
                var result []string
                for i, v := range m.multiSelected { if v { result = append(result, m.multiOptions[i]) } }
                // 寫入設定
                m.config.UserPreferences.EnabledLLMTriggers = result
                // 關閉面板並刷新列表
                m.multiActive = false
                m.refreshSettings()
                m.message = "Error triggers updated"
                return m, nil
            case tea.KeyEsc:
                // 取消關閉
                m.multiActive = false
                m.message = "Canceled"
                return m, nil
            }
            // 若未處理的其它鍵，忽略
            return m, nil
        }
        // If we are in editing mode, route keys to text input
        if m.isEditing {
            switch msg.Type {
            case tea.KeyEnter:
                // commit edit
                if m.editingItem != nil && m.editingItem.SetValue != nil {
                    m.editingItem.SetValue(m.config, m.textInput.Value())
                    m.message = fmt.Sprintf("%s updated", m.editingItem.DisplayName)
                }
                m.isEditing = false
                m.editingItem = nil
                return m, nil
            case tea.KeyEsc:
                // cancel edit
                m.isEditing = false
                m.editingItem = nil
                m.message = "Edit canceled"
                return m, nil
            default:
                var cmd tea.Cmd
                m.textInput, cmd = m.textInput.Update(msg)
                return m, cmd
            }
        }
        switch {
        case key.Matches(msg, m.keys.Quit):
            return m, tea.Quit

        case key.Matches(msg, m.keys.Enter):
            selectedItem := m.list.SelectedItem()
            if item, ok := selectedItem.(settingsItem); ok {
                // If this is a text setting, enter edit mode
                if item.Type == SettingTypeText {
                    return m.beginTextEdit(item.SettingItem)
                }
                return m.handleAction(item.SettingItem)
            }

		case key.Matches(msg, m.keys.Space):
			selectedItem := m.list.SelectedItem()
			if item, ok := selectedItem.(settingsItem); ok {
				if item.Type == SettingTypeBoolean {
					return m.toggleBoolean(item.SettingItem)
				}
			}

		case key.Matches(msg, m.keys.Left):
			selectedItem := m.list.SelectedItem()
			if item, ok := selectedItem.(settingsItem); ok {
				if item.Type == SettingTypeSelect {
					return m.changeSelectOption(item.SettingItem, -1)
				}
			}

		case key.Matches(msg, m.keys.Right):
			selectedItem := m.list.SelectedItem()
			if item, ok := selectedItem.(settingsItem); ok {
				if item.Type == SettingTypeSelect {
					return m.changeSelectOption(item.SettingItem, 1)
				}
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View implements tea.Model
func (m *SettingsModel) View() string {
    if m.width == 0 {
        return "Loading..."
    }

	// Create main container with proper spacing
	containerStyle := lipgloss.NewStyle().
		Padding(1, 2).
		Width(m.width).
		Height(m.height)

	// Main list view
	listContent := m.list.View()

    // Status line - only show if there's a message or editing
    var statusLine string
    if m.message != "" {
        statusStyle := lipgloss.NewStyle().
            Foreground(lipgloss.Color("11")).  // Yellow for messages
            Background(lipgloss.Color("0")).   // Black background
            Padding(0, 2).
            MarginTop(1).
            Bold(true)
        statusLine = statusStyle.Render("→ " + m.message)
        m.message = "" // Clear message after showing
    }

    // Inline editing prompt line
    var editLine string
    if m.isEditing {
        promptStyle := lipgloss.NewStyle().
            Foreground(lipgloss.Color("15")). // White
            Background(lipgloss.Color("4")).  // Blue background
            Padding(0, 2).
            MarginTop(1).
            Bold(true)
        label := "Edit"
        if m.editingItem != nil {
            label = fmt.Sprintf("Edit %s", m.editingItem.DisplayName)
        }
        editLine = promptStyle.Render(label+": ") + m.textInput.View()
    }

    // Modern help line（根據是否開啟多選面板顯示不同提示）
    helpStyle := lipgloss.NewStyle().
        Foreground(lipgloss.Color("8")).      // Gray
        Background(lipgloss.Color("0")).      // Black background
        Padding(0, 2).
        MarginTop(1).
        Border(lipgloss.RoundedBorder(), true, false, false, false).
        BorderForeground(lipgloss.Color("8"))

    helpText := "Navigate: ↑↓  Toggle: Space  Select: ←→  Action: Enter  Quit: q"
    if m.multiActive {
        helpText = "↑↓ Move  Space Toggle  a All  i Invert  Enter Confirm  Esc Cancel"
    }
    helpLine := helpStyle.Render(helpText)

    // 若開啟多選面板，覆蓋主列表，顯示簡潔的逐行多選 UI
    if m.multiActive {
        // 容器與清單樣式
        box := lipgloss.NewStyle().
            Border(lipgloss.RoundedBorder()).
            BorderForeground(lipgloss.Color("12")).
            Padding(1, 2).
            Width(m.width-6)
        title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("15")).Render(m.multiPrompt)
        // 列出逐行內容
        var lines []string
        for i, opt := range m.multiOptions {
            mark := "[ ]"
            if m.multiSelected[i] { mark = "[x]" }
            cursor := "  "
            if i == m.multiCursor { cursor = "> " }
            lines = append(lines, fmt.Sprintf("%s%s %s", cursor, mark, opt))
        }
        listBlock := strings.Join(lines, "\n")
        content := lipgloss.JoinVertical(lipgloss.Left, title, listBlock)
        panel := box.Render(content)
        return containerStyle.Render(lipgloss.JoinVertical(lipgloss.Left, panel, helpLine))
    }

    // Combine all parts（一般模式）
    var content string
    switch {
    case m.isEditing && statusLine != "":
        content = lipgloss.JoinVertical(lipgloss.Left, listContent, editLine, statusLine, helpLine)
    case m.isEditing:
        content = lipgloss.JoinVertical(lipgloss.Left, listContent, editLine, helpLine)
    case statusLine != "":
        content = lipgloss.JoinVertical(lipgloss.Left, listContent, statusLine, helpLine)
    default:
        content = lipgloss.JoinVertical(lipgloss.Left, listContent, helpLine)
    }

	return containerStyle.Render(content)
}

// handleAction handles actions for different setting types
func (m *SettingsModel) handleAction(item *SettingItem) (*SettingsModel, tea.Cmd) {
    switch item.Type {
    case SettingTypeAction:
        // 針對錯誤觸發類型，使用內嵌多選面板，不呼叫外部 Stdout UI
        if item.ID == "user_preferences.enabled_llm_triggers" {
            // 與 settings_definition 中的選項一致
            opts := []string{
                "CommandNotFound",
                "FileNotFoundOrDirectory",
                "PermissionDenied",
                "CannotExecute",
                "InvalidArgumentOrOption",
                "ResourceExists",
                "NotADirectory",
                "TerminatedBySignal",
                "GenericError",
            }
            sel := make([]bool, len(opts))
            // 依據現有設定預選
            current := map[string]bool{}
            for _, v := range m.config.UserPreferences.EnabledLLMTriggers { current[v] = true }
            for i, o := range opts { sel[i] = current[o] }
            m.multiActive = true
            m.multiPrompt = "Select error types to enable AI analysis (space to toggle, enter to confirm):"
            m.multiOptions = opts
            m.multiSelected = sel
            m.multiCursor = 0
            return m, nil
        }
        if item.Action != nil {
            if err := item.Action(); err != nil {
                m.message = fmt.Sprintf("Error: %v", err)
            } else {
                m.message = "Action completed successfully"
                // Refresh settings to update dynamic items
                m.refreshSettings()
            }
        }
    case SettingTypeBoolean:
        return m.toggleBoolean(item)
    }
    return m, nil
}

// beginTextEdit initializes the text input for a text setting
func (m *SettingsModel) beginTextEdit(item *SettingItem) (*SettingsModel, tea.Cmd) {
    m.isEditing = true
    m.editingItem = item
    ti := textinput.New()
    ti.Prompt = ""
    ti.Placeholder = "輸入新值，Enter 確認，Esc 取消"
    ti.Focus()
    // Initialize with current value except for API Key
    if item.GetValue != nil {
        if str, ok := item.GetValue(m.config).(string); ok {
            // 如果是 API Key，避免顯示遮罩值，預設空白由使用者輸入
            if strings.Contains(item.ID, "api_key") {
                ti.SetValue("")
                ti.EchoMode = textinput.EchoPassword
                ti.EchoCharacter = '•'
            } else {
                ti.SetValue(str)
            }
        }
    }
    m.textInput = ti
    return m, nil
}

// toggleBoolean toggles a boolean setting
func (m *SettingsModel) toggleBoolean(item *SettingItem) (*SettingsModel, tea.Cmd) {
	if item.GetValue != nil && item.SetValue != nil {
		currentValue := item.GetValue(m.config)
		if boolVal, ok := currentValue.(bool); ok {
			item.SetValue(m.config, !boolVal)
			// Removed confirmation message as requested
		}
	}
	return m, nil
}

// changeSelectOption changes the selected option in a select setting
func (m *SettingsModel) changeSelectOption(item *SettingItem, direction int) (*SettingsModel, tea.Cmd) {
	if item.GetValue == nil || item.SetValue == nil || len(item.Options) == 0 {
		return m, nil
	}

	currentValue := item.GetValue(m.config)
	currentStr, ok := currentValue.(string)
	if !ok {
		return m, nil
	}

	// Find current option index
	currentIndex := -1
	for i, option := range item.Options {
		if option.Value == currentStr {
			currentIndex = i
			break
		}
	}

	if currentIndex == -1 {
		return m, nil
	}

	// Calculate new index with wrapping
	newIndex := (currentIndex + direction + len(item.Options)) % len(item.Options)
	newValue := item.Options[newIndex].Value
	
    // Apply the new value without emitting a transient status message.
    // Rationale: Changing select options already updates inline UI text;
    // additional status line like "→ Language: 繁體中文" is noisy and unnecessary.
    item.SetValue(m.config, newValue)
    // 選項變更後可能影響其他動態項目（例如 provider 切換後 API Host 編輯權限），刷新列表
    m.refreshSettings()
    return m, nil
}

// refreshSettings refreshes the settings list (useful for dynamic items)
func (m *SettingsModel) refreshSettings() {
	m.settings = GetSettingsDefinition(m.config)
	
	items := make([]list.Item, len(m.settings))
	for i, setting := range m.settings {
		items[i] = settingsItem{SettingItem: setting, config: m.config}
	}
	
	m.list.SetItems(items)
}

// itemDelegate implements list.ItemDelegate for rendering list items
type itemDelegate struct{}

func (d itemDelegate) Height() int                               { return 1 }
func (d itemDelegate) Spacing() int                             { return 0 }
func (d itemDelegate) Update(tea.Msg, *list.Model) tea.Cmd      { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	if item, ok := listItem.(settingsItem); ok {
		str := d.renderItem(item, index == m.Index())
		fmt.Fprint(w, str)
	}
}

// renderItem renders a single setting item
func (d itemDelegate) renderItem(item settingsItem, isSelected bool) string {
	setting := item.SettingItem
	
	// Modern, clean style definitions
	selectedStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("4")).    // Blue background for selection
		Foreground(lipgloss.Color("15")).   // White text
		Padding(0, 1).
		MarginLeft(0)
	
	normalStyle := lipgloss.NewStyle().
		Padding(0, 1).
		MarginLeft(0)
	
	// Clean group header style - no borders, just emphasis
	groupHeaderStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("14")).   // Cyan/bright blue
		MarginTop(1).
		MarginBottom(0).
		Padding(0, 1)
	
	// Sub-group style for secondary headers
	subGroupStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).    // Gray
		Italic(true).
		MarginBottom(0).
		Padding(0, 1)

	var style lipgloss.Style
	if isSelected {
		style = selectedStyle
	} else {
		style = normalStyle
	}

    switch setting.Type {
	case SettingTypeGroup:
		// Handle empty lines for spacing
		if strings.TrimSpace(setting.DisplayName) == "" {
			return "\n" // Empty line for spacing
		}
		
		// Determine if this is a main header or sub-header
		if setting.DisplayName == "Settings" {
			// Main header - more prominent
			headerStyle := lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("15")).  // White
				Background(lipgloss.Color("0")).   // Black background
				Padding(0, 2).
				MarginTop(0).
				MarginBottom(1)
			return headerStyle.Render(setting.DisplayName)
		} else if strings.Contains(setting.DisplayName, "preferences") || 
			     strings.Contains(setting.DisplayName, "Configure") {
			// Sub-header for description
			return subGroupStyle.Render(setting.DisplayName)
		} else {
			// Section header
			return groupHeaderStyle.Render(setting.DisplayName)
		}

	case SettingTypeBoolean:
		value := "false"
		if setting.GetValue != nil {
			if boolVal, ok := setting.GetValue(item.config).(bool); ok && boolVal {
				value = "true"
			}
		}
		
		// Create justified layout
		nameStyle := lipgloss.NewStyle().Width(40).Align(lipgloss.Left)
		valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // Green for true values
		if value == "false" {
			valueStyle = valueStyle.Foreground(lipgloss.Color("8")) // Gray for false
		}
		
		content := nameStyle.Render("   "+setting.DisplayName) + valueStyle.Render(value)
		return style.Render(content)

	case SettingTypeSelect:
		value := "default"
		if setting.GetValue != nil {
			if strVal, ok := setting.GetValue(item.config).(string); ok {
				// Find display name for the value
				for _, option := range setting.Options {
					if option.Value == strVal {
						value = option.DisplayName
						break
					}
				}
			}
		}
		
		// Create justified layout with selection indicator
		nameStyle := lipgloss.NewStyle().Width(39).Align(lipgloss.Left)
		valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("12")) // Bright blue
		
		content := "❯ " + nameStyle.Render(setting.DisplayName) + valueStyle.Render(value)
		return style.Render(content)

    case SettingTypeAction:
		nameStyle := lipgloss.NewStyle().Width(40).Align(lipgloss.Left)
		actionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("11")) // Yellow
		
		content := nameStyle.Render("   "+setting.DisplayName) + actionStyle.Render("[Action]")
		return style.Render(content)

    case SettingTypeInfo:
		value := "N/A"
		if setting.GetValue != nil {
			if strVal, ok := setting.GetValue(item.config).(string); ok {
				value = strVal
			}
		}
		
		nameStyle := lipgloss.NewStyle().Width(40).Align(lipgloss.Left)
		valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8")) // Gray for info
		
        content := nameStyle.Render("   "+setting.DisplayName) + valueStyle.Render(value)
        return style.Render(content)

    case SettingTypeText:
        value := ""
        if setting.GetValue != nil {
            if strVal, ok := setting.GetValue(item.config).(string); ok {
                value = strVal
            }
        }
        nameStyle := lipgloss.NewStyle().Width(40).Align(lipgloss.Left)
        valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("12")) // Bright blue
        content := nameStyle.Render("   "+setting.DisplayName) + valueStyle.Render(value)
        return style.Render(content)

	default:
		nameStyle := lipgloss.NewStyle().Width(40).Align(lipgloss.Left)
		content := nameStyle.Render("   "+setting.DisplayName)
		return style.Render(content)
	}
}

// RunSettingsTUI runs the settings TUI
func RunSettingsTUI(cfg *config.Config) error {
	model := NewSettingsModel(cfg)
	
	// Use default input/output for proper terminal handling
	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("failed to run settings TUI: %w", err)
	}
	
	// Save configuration after TUI exits
	if settingsModel, ok := finalModel.(*SettingsModel); ok {
		if err := settingsModel.config.Save(); err != nil {
			return fmt.Errorf("failed to save configuration: %w", err)
		}
	}
	
	return nil
}
