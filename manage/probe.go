package manage

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/novembersoftware/aretheyup/structs"
)

type probeResult int

const (
	probeResultNone   probeResult = iota
	probeResultSave               // user pressed ctrl+s
	probeResultCancel             // user pressed esc
)

// probe form field indices
const (
	pfURL      = 0
	pfMethod   = 1
	pfInterval = 2
	pfTimeout  = 3
	pfExpected = 4
	pfEnabled  = 5 // toggle — not a text input
	pfCount    = 6
)

type probeModel struct {
	serviceID     uint
	existingID    uint // non-zero if updating an existing ProbeConfig
	inputs        []textinput.Model
	enabledToggle bool
	focused       int
	result        probeResult
	inputWidth    int
	width         int
	height        int
}

func newProbeModel(serviceID uint, existing *structs.ProbeConfig) probeModel {
	inputs := make([]textinput.Model, pfCount-1)

	for i := range inputs {
		t := textinput.New()
		t.Cursor.Style = listCursorStyle
		t.PromptStyle = formLabelStyle
		t.TextStyle = formValueStyle
		t.Prompt = ""
		inputs[i] = t
	}

	inputs[pfURL].Placeholder = "https://example.com/health"
	inputs[pfURL].CharLimit = 255

	inputs[pfMethod].Placeholder = "GET"
	inputs[pfMethod].CharLimit = 10

	inputs[pfInterval].Placeholder = "60"
	inputs[pfInterval].CharLimit = 6

	inputs[pfTimeout].Placeholder = "10"
	inputs[pfTimeout].CharLimit = 6

	inputs[pfExpected].Placeholder = "200"
	inputs[pfExpected].CharLimit = 5

	enabled := true
	var existingID uint

	if existing != nil {
		existingID = existing.ID
		inputs[pfURL].SetValue(existing.URL)
		inputs[pfMethod].SetValue(existing.Method)
		inputs[pfInterval].SetValue(fmt.Sprintf("%d", existing.IntervalSeconds))
		inputs[pfTimeout].SetValue(fmt.Sprintf("%d", existing.TimeoutSeconds))
		inputs[pfExpected].SetValue(fmt.Sprintf("%d", existing.ExpectedStatus))
		enabled = existing.Enabled
	} else {
		inputs[pfMethod].SetValue("GET")
		inputs[pfInterval].SetValue("60")
		inputs[pfTimeout].SetValue("10")
		inputs[pfExpected].SetValue("200")
	}

	inputs[pfURL].Focus()

	return probeModel{
		serviceID:     serviceID,
		existingID:    existingID,
		inputs:        inputs,
		enabledToggle: enabled,
		focused:       0,
	}
}

func (m *probeModel) setSize(w, h int) {
	m.width = w
	m.height = h
	iw := w - 8
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

func (m probeModel) update(msg tea.Msg) (probeModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+s":
			m.result = probeResultSave
			return m, nil
		case "esc":
			m.result = probeResultCancel
			return m, nil
		case "tab", "down":
			m.focused = (m.focused + 1) % pfCount
			m.syncFocus()
			return m, nil
		case "shift+tab", "up":
			m.focused = (m.focused - 1 + pfCount) % pfCount
			m.syncFocus()
			return m, nil
		case " ":
			if m.focused == pfEnabled {
				m.enabledToggle = !m.enabledToggle
				return m, nil
			}
		}
	}

	if m.focused < pfEnabled {
		var cmd tea.Cmd
		m.inputs[m.focused], cmd = m.inputs[m.focused].Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *probeModel) syncFocus() {
	for i := range m.inputs {
		if i == m.focused {
			m.inputs[i].Focus()
		} else {
			m.inputs[i].Blur()
		}
	}
}

func (m probeModel) toProbeConfig() *structs.ProbeConfig {
	interval, _ := strconv.Atoi(strings.TrimSpace(m.inputs[pfInterval].Value()))
	timeout, _ := strconv.Atoi(strings.TrimSpace(m.inputs[pfTimeout].Value()))
	expected, _ := strconv.Atoi(strings.TrimSpace(m.inputs[pfExpected].Value()))

	if interval <= 0 {
		interval = 60
	}
	if timeout <= 0 {
		timeout = 10
	}
	if expected <= 0 {
		expected = 200
	}

	return &structs.ProbeConfig{
		ID:              m.existingID,
		ServiceID:       m.serviceID,
		Enabled:         m.enabledToggle,
		URL:             strings.TrimSpace(m.inputs[pfURL].Value()),
		Method:          strings.TrimSpace(m.inputs[pfMethod].Value()),
		IntervalSeconds: interval,
		TimeoutSeconds:  timeout,
		ExpectedStatus:  expected,
	}
}

func (m probeModel) view(status string, statusErr bool) string {
	isNew := m.existingID == 0
	subtitle := "Edit Probe Config"
	if isNew {
		subtitle = "New Probe Config"
	}

	var b strings.Builder
	b.WriteString(appTitleStyle.Render("Are They Up") + appSubtitleStyle.Render(subtitle))
	b.WriteString("\n\n")

	fields := []struct {
		label string
		idx   int
		hint  string
	}{
		{"URL", pfURL, "endpoint to probe"},
		{"Method", pfMethod, "HTTP method: GET, POST, HEAD, …"},
		{"Interval (seconds)", pfInterval, "how often to probe"},
		{"Timeout (seconds)", pfTimeout, "request timeout"},
		{"Expected Status", pfExpected, "HTTP status considered healthy"},
	}

	for _, f := range fields {
		b.WriteString(renderFormField(
			f.label, f.hint,
			m.inputs[f.idx].View(),
			m.focused == f.idx,
			m.inputWidth,
		))
	}

	// Enabled toggle — reuse renderToggle, but label it "Enabled"
	b.WriteString(renderProbeEnabledToggle(m.enabledToggle, m.focused == pfEnabled, m.inputWidth))

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

// renderProbeEnabledToggle renders the Enabled toggle using the same underline pattern
// as the service form's Active toggle.
func renderProbeEnabledToggle(enabled, focused bool, width int) string {
	var sb strings.Builder

	var labelStr string
	if focused {
		labelStr = formLabelStyle.Copy().Foreground(colorPrimary).Render("Enabled")
	} else {
		labelStr = formLabelStyle.Render("Enabled")
	}
	sb.WriteString("  " + labelStr + "\n")

	var badge string
	if enabled {
		badge = badgeActiveStyle.Render(" ON — enabled ")
	} else {
		badge = badgeInactiveStyle.Render(" OFF — disabled ")
	}
	hint := formHintStyle.Render("  press space to toggle")
	sb.WriteString("  " + badge + hint + "\n")

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
	sb.WriteString("  " + underline + "\n\n")

	return sb.String()
}
