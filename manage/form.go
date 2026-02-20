package manage

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/novembersoftware/aretheyup/structs"
)

type formResult int

const (
	formResultNone   formResult = iota
	formResultSave              // user pressed ctrl+s
	formResultCancel            // user pressed esc
)

// field indices
const (
	fName        = 0
	fSlug        = 1
	fDescription = 2
	fCategory    = 3
	fHomepage    = 4
	fActive      = 5 // toggle — not a text input
	fCount       = 6
)

type formModel struct {
	serviceID    uint
	inputs       []textinput.Model
	activeToggle bool
	focused      int
	result       formResult
	slugEdited   bool // true once user has manually changed the slug
	inputWidth   int
	width        int
	height       int
}

func newFormModel(svc *structs.Service) formModel {
	inputs := make([]textinput.Model, fCount-1) // Name, Slug, Description, Category, Homepage

	for i := range inputs {
		t := textinput.New()
		t.Cursor.Style = listCursorStyle
		t.PromptStyle = formLabelStyle
		t.TextStyle = formValueStyle
		t.Prompt = ""
		inputs[i] = t
	}

	inputs[fName].Placeholder = "My Awesome Service"
	inputs[fName].CharLimit = 100

	inputs[fSlug].Placeholder = "my-awesome-service"
	inputs[fSlug].CharLimit = 100

	inputs[fDescription].Placeholder = "Optional description"
	inputs[fDescription].CharLimit = 255

	inputs[fCategory].Placeholder = "other"
	inputs[fCategory].CharLimit = 50

	inputs[fHomepage].Placeholder = "https://example.com"
	inputs[fHomepage].CharLimit = 255

	active := true
	slugEdited := false

	if svc != nil {
		inputs[fName].SetValue(svc.Name)
		inputs[fSlug].SetValue(svc.Slug)
		inputs[fDescription].SetValue(svc.Description)
		inputs[fCategory].SetValue(svc.Category)
		inputs[fHomepage].SetValue(svc.HomepageURL)
		active = svc.Active
		slugEdited = true
	}

	inputs[fName].Focus()

	var id uint
	if svc != nil {
		id = svc.ID
	}

	return formModel{
		serviceID:    id,
		inputs:       inputs,
		activeToggle: active,
		focused:      0,
		slugEdited:   slugEdited,
	}
}

func (m *formModel) setSize(w, h int) {
	m.width = w
	m.height = h
	iw := w - 8 // leave margins
	if iw < 30 {
		iw = 30
	}
	if iw > 90 {
		iw = 90
	}
	m.inputWidth = iw
	for i := range m.inputs {
		m.inputs[i].Width = iw
	}
}

func (m formModel) update(msg tea.Msg) (formModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+s":
			m.result = formResultSave
			return m, nil
		case "esc":
			m.result = formResultCancel
			return m, nil
		case "tab", "down":
			m.focused = (m.focused + 1) % fCount
			m.syncFocus()
			return m, nil
		case "shift+tab", "up":
			m.focused = (m.focused - 1 + fCount) % fCount
			m.syncFocus()
			return m, nil
		case " ":
			if m.focused == fActive {
				m.activeToggle = !m.activeToggle
				return m, nil
			}
		}
	}

	if m.focused < fActive {
		var cmd tea.Cmd
		m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)

		// Auto-generate slug from name while user hasn't manually edited it
		if m.focused == fName && !m.slugEdited {
			m.inputs[fSlug].SetValue(slugify(m.inputs[fName].Value()))
		}
		if m.focused == fSlug {
			m.slugEdited = true
		}
		return m, cmd
	}

	return m, nil
}

func (m *formModel) syncFocus() {
	for i := range m.inputs {
		if i == m.focused {
			m.inputs[i].Focus()
		} else {
			m.inputs[i].Blur()
		}
	}
}

func (m formModel) toService() *structs.Service {
	return &structs.Service{
		ID:          m.serviceID,
		Name:        strings.TrimSpace(m.inputs[fName].Value()),
		Slug:        strings.TrimSpace(m.inputs[fSlug].Value()),
		Description: strings.TrimSpace(m.inputs[fDescription].Value()),
		Category:    strings.TrimSpace(m.inputs[fCategory].Value()),
		HomepageURL: strings.TrimSpace(m.inputs[fHomepage].Value()),
		Active:      m.activeToggle,
	}
}

func (m formModel) view(status string, statusErr bool) string {
	isCreate := m.serviceID == 0
	title := "Edit Service"
	if isCreate {
		title = "Create Service"
	}

	var b strings.Builder
	b.WriteString(appTitleStyle.Render("Are They Up") + appSubtitleStyle.Render(title))
	b.WriteString("\n\n")

	fields := []struct {
		label string
		idx   int
		hint  string
	}{
		{"Name", fName, ""},
		{"Slug", fSlug, "auto-generated from name, or edit manually"},
		{"Description", fDescription, "optional"},
		{"Category", fCategory, "e.g. finance, infra, saas, …"},
		{"Homepage URL", fHomepage, ""},
	}

	for _, f := range fields {
		b.WriteString(renderFormField(
			f.label, f.hint,
			m.inputs[f.idx].View(),
			m.focused == f.idx,
			m.inputWidth,
		))
	}

	// Active toggle
	b.WriteString(renderToggle(m.activeToggle, m.focused == fActive, m.inputWidth))

	// Status line
	if status != "" {
		if statusErr {
			b.WriteString(errorStyle.Render(status))
		} else {
			b.WriteString(successStyle.Render(status))
		}
		b.WriteString("\n")
	}

	// Help bar
	entries := []string{
		helpEntry("tab/↓", "next"),
		helpEntry("shift+tab/↑", "prev"),
		helpEntry("space", "toggle"),
		helpEntry("ctrl+s", "save"),
		helpEntry("esc", "cancel"),
	}
	b.WriteString("\n" + helpStyle.Render("  "+strings.Join(entries, "  ·  ")))

	return b.String()
}

// renderFormField renders a single label + underline-style input — no border boxes,
// which avoids the height inconsistency between focused/blurred textinput rendering.
func renderFormField(label, hint, inputView string, focused bool, width int) string {
	var b strings.Builder

	// Label row
	var labelStr string
	if focused {
		labelStr = formLabelStyle.Copy().Foreground(colorPrimary).Render(label)
	} else {
		labelStr = formLabelStyle.Render(label)
	}
	if hint != "" {
		labelStr += "  " + formHintStyle.Render(hint)
	}
	b.WriteString("  " + labelStr + "\n")

	// Value row
	var valueStr string
	if focused {
		valueStr = fieldFocusedValueStyle.Render(inputView)
	} else {
		valueStr = fieldBlurredValueStyle.Render(inputView)
	}
	b.WriteString("  " + valueStr + "\n")

	// Underline row
	lineWidth := width + 4
	if lineWidth < 10 {
		lineWidth = 10
	}
	var underline string
	if focused {
		underline = lipgloss.NewStyle().Foreground(colorPrimary).Render(strings.Repeat("─", lineWidth))
	} else {
		underline = lipgloss.NewStyle().Foreground(colorBorder).Render(strings.Repeat("─", lineWidth))
	}
	b.WriteString("  " + underline + "\n\n")

	return b.String()
}

// renderToggle renders the active/enabled toggle as a labeled row with underline.
func renderToggle(on bool, focused bool, width int) string {
	var b strings.Builder

	var labelStr string
	if focused {
		labelStr = formLabelStyle.Copy().Foreground(colorPrimary).Render("Active")
	} else {
		labelStr = formLabelStyle.Render("Active")
	}
	b.WriteString("  " + labelStr + "\n")

	var badge string
	if on {
		badge = badgeActiveStyle.Render(" ON — active ")
	} else {
		badge = badgeInactiveStyle.Render(" OFF — inactive ")
	}
	hint := formHintStyle.Render("  press space to toggle")
	b.WriteString("  " + badge + hint + "\n")

	lineWidth := width + 4
	if lineWidth < 10 {
		lineWidth = 10
	}
	var underline string
	if focused {
		underline = lipgloss.NewStyle().Foreground(colorPrimary).Render(strings.Repeat("─", lineWidth))
	} else {
		underline = lipgloss.NewStyle().Foreground(colorBorder).Render(strings.Repeat("─", lineWidth))
	}
	b.WriteString("  " + underline + "\n\n")

	return b.String()
}

// slugify converts a display name into a URL-friendly slug.
func slugify(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevDash = false
		} else if !prevDash && b.Len() > 0 {
			b.WriteRune('-')
			prevDash = true
		}
	}
	return strings.TrimRight(b.String(), "-")
}
