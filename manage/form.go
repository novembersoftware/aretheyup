package manage

import (
	"fmt"
	"strconv"
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
	formResultSave              // ctrl+s
	formResultCancel            // esc
)

// Number of text inputs in each section
const (
	svcInputCount   = 5 // Name, Slug, Description, Category, Homepage
	probeInputCount = 5 // URL, Method, Interval, Timeout, Expected
)

// Absolute focus positions
const (
	fActive    = svcInputCount     // 5 — activeToggle
	fProbeGate = svcInputCount + 1 // 6 — "add/remove probe" gate (skipped when existingID != 0)
)

type formModel struct {
	serviceID uint

	// Service text inputs: Name, Slug, Description, Category, Homepage
	svcInputs    []textinput.Model
	activeToggle bool

	// Probe section
	probeExistingID  uint              // non-zero → editing existing config; section always visible
	showProbeSection bool              // toggled by gate when probeExistingID == 0
	probeInputs      []textinput.Model // URL, Method, Interval, Timeout, Expected
	probeEnabled     bool

	focused    int
	result     formResult
	slugEdited bool
	inputWidth int
	width      int
	height     int
}

// probeBase returns the focused index at which probe text inputs begin.
func (m formModel) probeBase() int {
	if m.probeExistingID != 0 {
		return fActive + 1 // 6 — no gate
	}
	return fActive + 2 // 7 — gate at 6, probe starts at 7
}

// totalFields returns the total number of focusable slots.
func (m formModel) totalFields() int {
	if m.probeExistingID != 0 || m.showProbeSection {
		return m.probeBase() + probeInputCount + 1 // +1 for probeEnabled toggle
	}
	return fProbeGate + 1 // just the gate
}

func newFormModel(svc *structs.Service, pc *structs.ProbeConfig) formModel {
	// ── Service inputs ───────────────────────────────────────────────────────
	svcInputs := make([]textinput.Model, svcInputCount)
	for i := range svcInputs {
		t := textinput.New()
		t.Cursor.Style = listCursorStyle
		t.PromptStyle = formLabelStyle
		t.TextStyle = formValueStyle
		t.Prompt = ""
		svcInputs[i] = t
	}
	svcInputs[0].Placeholder = "My Awesome Service"
	svcInputs[0].CharLimit = 100
	svcInputs[1].Placeholder = "my-awesome-service"
	svcInputs[1].CharLimit = 100
	svcInputs[2].Placeholder = "Optional description"
	svcInputs[2].CharLimit = 255
	svcInputs[3].Placeholder = "other"
	svcInputs[3].CharLimit = 50
	svcInputs[4].Placeholder = "https://example.com"
	svcInputs[4].CharLimit = 255

	active := true
	slugEdited := false
	var serviceID uint
	if svc != nil {
		serviceID = svc.ID
		svcInputs[0].SetValue(svc.Name)
		svcInputs[1].SetValue(svc.Slug)
		svcInputs[2].SetValue(svc.Description)
		svcInputs[3].SetValue(svc.Category)
		svcInputs[4].SetValue(svc.HomepageURL)
		active = svc.Active
		slugEdited = true
	}
	svcInputs[0].Focus()

	// ── Probe inputs ─────────────────────────────────────────────────────────
	probeInputs := make([]textinput.Model, probeInputCount)
	for i := range probeInputs {
		t := textinput.New()
		t.Cursor.Style = listCursorStyle
		t.PromptStyle = formLabelStyle
		t.TextStyle = formValueStyle
		t.Prompt = ""
		probeInputs[i] = t
	}
	probeInputs[0].Placeholder = "https://example.com/health"
	probeInputs[0].CharLimit = 255
	probeInputs[1].Placeholder = "GET"
	probeInputs[1].CharLimit = 10
	probeInputs[2].Placeholder = "60"
	probeInputs[2].CharLimit = 6
	probeInputs[3].Placeholder = "10"
	probeInputs[3].CharLimit = 6
	probeInputs[4].Placeholder = "200"
	probeInputs[4].CharLimit = 5

	probeEnabled := true
	var probeExistingID uint
	showProbeSection := false

	if pc != nil {
		probeExistingID = pc.ID
		showProbeSection = true
		probeInputs[0].SetValue(pc.URL)
		probeInputs[1].SetValue(pc.Method)
		probeInputs[2].SetValue(fmt.Sprintf("%d", pc.IntervalSeconds))
		probeInputs[3].SetValue(fmt.Sprintf("%d", pc.TimeoutSeconds))
		probeInputs[4].SetValue(fmt.Sprintf("%d", pc.ExpectedStatus))
		probeEnabled = pc.Enabled
	} else {
		// Sensible defaults for new probe config
		probeInputs[1].SetValue("GET")
		probeInputs[2].SetValue("60")
		probeInputs[3].SetValue("10")
		probeInputs[4].SetValue("200")
	}

	return formModel{
		serviceID:        serviceID,
		svcInputs:        svcInputs,
		activeToggle:     active,
		probeExistingID:  probeExistingID,
		showProbeSection: showProbeSection,
		probeInputs:      probeInputs,
		probeEnabled:     probeEnabled,
		slugEdited:       slugEdited,
	}
}

func (m *formModel) setSize(w, h int) {
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
	for i := range m.svcInputs {
		m.svcInputs[i].Width = iw
	}
	for i := range m.probeInputs {
		m.probeInputs[i].Width = iw
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
			m.focused = (m.focused + 1) % m.totalFields()
			m.syncFocus()
			return m, nil
		case "shift+tab", "up":
			m.focused = (m.focused - 1 + m.totalFields()) % m.totalFields()
			m.syncFocus()
			return m, nil
		case " ":
			switch {
			case m.focused == fActive:
				m.activeToggle = !m.activeToggle
				return m, nil
			case m.probeExistingID == 0 && m.focused == fProbeGate:
				m.showProbeSection = !m.showProbeSection
				if !m.showProbeSection {
					m.focused = fProbeGate
					m.syncFocus()
				}
				return m, nil
			case m.focused == m.probeBase()+probeInputCount:
				m.probeEnabled = !m.probeEnabled
				return m, nil
			}
		}
	}

	// Route to the appropriate text input
	if m.focused < svcInputCount {
		var cmd tea.Cmd
		m.svcInputs[m.focused], cmd = m.svcInputs[m.focused].Update(msg)
		if m.focused == 0 && !m.slugEdited {
			m.svcInputs[1].SetValue(slugify(m.svcInputs[0].Value()))
		}
		if m.focused == 1 {
			m.slugEdited = true
		}
		return m, cmd
	}

	pb := m.probeBase()
	if m.focused >= pb && m.focused < pb+probeInputCount {
		pi := m.focused - pb
		var cmd tea.Cmd
		m.probeInputs[pi], cmd = m.probeInputs[pi].Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *formModel) syncFocus() {
	for i := range m.svcInputs {
		if i == m.focused {
			m.svcInputs[i].Focus()
		} else {
			m.svcInputs[i].Blur()
		}
	}
	pb := m.probeBase()
	for i := range m.probeInputs {
		if pb+i == m.focused {
			m.probeInputs[i].Focus()
		} else {
			m.probeInputs[i].Blur()
		}
	}
}

func (m formModel) toService() *structs.Service {
	return &structs.Service{
		ID:          m.serviceID,
		Name:        strings.TrimSpace(m.svcInputs[0].Value()),
		Slug:        strings.TrimSpace(m.svcInputs[1].Value()),
		Description: strings.TrimSpace(m.svcInputs[2].Value()),
		Category:    strings.TrimSpace(m.svcInputs[3].Value()),
		HomepageURL: strings.TrimSpace(m.svcInputs[4].Value()),
		Active:      m.activeToggle,
	}
}

// toProbeConfig returns the probe config to save, or nil if the probe section
// is disabled and there is no existing probe to update.
func (m formModel) toProbeConfig() *structs.ProbeConfig {
	if !m.showProbeSection && m.probeExistingID == 0 {
		return nil
	}
	interval, _ := strconv.Atoi(strings.TrimSpace(m.probeInputs[2].Value()))
	timeout, _ := strconv.Atoi(strings.TrimSpace(m.probeInputs[3].Value()))
	expected, _ := strconv.Atoi(strings.TrimSpace(m.probeInputs[4].Value()))
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
		ID:              m.probeExistingID,
		ServiceID:       m.serviceID,
		Enabled:         m.probeEnabled,
		URL:             strings.TrimSpace(m.probeInputs[0].Value()),
		Method:          strings.TrimSpace(m.probeInputs[1].Value()),
		IntervalSeconds: interval,
		TimeoutSeconds:  timeout,
		ExpectedStatus:  expected,
	}
}

// ── View ─────────────────────────────────────────────────────────────────────

func (m formModel) view(status string, statusErr bool) string {
	isCreate := m.serviceID == 0
	title := "Edit Service"
	if isCreate {
		title = "Create Service"
	}

	var b strings.Builder
	b.WriteString(appTitleStyle.Render("Are They Up") + appSubtitleStyle.Render(title))
	b.WriteString("\n\n")

	// Service fields
	svcFields := []struct{ label, hint string }{
		{"Name", ""},
		{"Slug", "auto-generated from name, or edit manually"},
		{"Description", "optional"},
		{"Category", "e.g. finance, infra, saas, …"},
		{"Homepage URL", ""},
	}
	for i, f := range svcFields {
		b.WriteString(renderFormField(f.label, f.hint, m.svcInputs[i].View(), m.focused == i, m.inputWidth))
	}

	// Active toggle
	b.WriteString(renderToggle("Active", m.activeToggle, m.focused == fActive, m.inputWidth))

	// Probe section
	b.WriteString(m.renderProbeSection())

	// Status
	if status != "" {
		if statusErr {
			b.WriteString(errorStyle.Render(status))
		} else {
			b.WriteString(successStyle.Render(status))
		}
		b.WriteString("\n")
	}

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

func (m formModel) renderProbeSection() string {
	var b strings.Builder
	lineWidth := m.inputWidth + 4
	if lineWidth < 10 {
		lineWidth = 10
	}

	// When no existing probe, render the gate toggle first
	if m.probeExistingID == 0 {
		gateFocused := m.focused == fProbeGate
		if !m.showProbeSection {
			// "Add probe config" button — exit early, no probe fields
			var labelStr string
			if gateFocused {
				labelStr = formLabelStyle.Copy().Foreground(colorPrimary).Render("+ Add probe config")
			} else {
				labelStr = mutedStyle.Render("+ Add probe config")
			}
			b.WriteString("  " + labelStr + "  " + formHintStyle.Render("press space to add") + "\n")
			b.WriteString("  " + sectionUnderline(gateFocused, lineWidth) + "\n\n")
			return b.String()
		}
		// Probe section is being added — show a remove option at top
		b.WriteString("\n  " + probeSectionDivider(lineWidth) + "\n\n")
		var removeStr string
		if gateFocused {
			removeStr = formLabelStyle.Copy().Foreground(colorDanger).Render("− Remove probe config") +
				"  " + formHintStyle.Render("press space to remove")
		} else {
			removeStr = mutedStyle.Render("− Remove probe config") +
				"  " + formHintStyle.Render("press space to remove")
		}
		b.WriteString("  " + removeStr + "\n")
		b.WriteString("  " + sectionUnderline(gateFocused, lineWidth) + "\n\n")
	} else {
		// Existing probe — permanent separator, no gate in tab order
		b.WriteString("\n  " + probeSectionDivider(lineWidth) + "\n\n")
	}

	// Probe text fields
	pb := m.probeBase()
	probeFields := []struct{ label, hint string }{
		{"URL", "endpoint to probe"},
		{"Method", "HTTP method: GET, POST, HEAD, …"},
		{"Interval (seconds)", "how often to probe"},
		{"Timeout (seconds)", "request timeout"},
		{"Expected Status", "HTTP status considered healthy"},
	}
	for i, f := range probeFields {
		b.WriteString(renderFormField(f.label, f.hint, m.probeInputs[i].View(), m.focused == pb+i, m.inputWidth))
	}

	// Probe enabled toggle
	b.WriteString(renderToggle("Enabled", m.probeEnabled, m.focused == pb+probeInputCount, m.inputWidth))

	return b.String()
}

// probeSectionDivider renders "──── Probe Config ────" with dim dashes.
func probeSectionDivider(lineWidth int) string {
	label := " Probe Config "
	labelLen := len([]rune(label))
	dashCount := (lineWidth - labelLen) / 2
	if dashCount < 1 {
		dashCount = 1
	}
	dashes := lipgloss.NewStyle().Foreground(colorBorder).Render(strings.Repeat("─", dashCount))
	title := lipgloss.NewStyle().Foreground(colorSecondary).Bold(true).Render(label)
	return dashes + title + dashes
}

func sectionUnderline(focused bool, lineWidth int) string {
	if focused {
		return lipgloss.NewStyle().Foreground(colorPrimary).Render(strings.Repeat("─", lineWidth))
	}
	return lipgloss.NewStyle().Foreground(colorBorder).Render(strings.Repeat("─", lineWidth))
}

// ── Shared field renderers ────────────────────────────────────────────────────

// renderFormField renders a label + value + underline (no border boxes).
func renderFormField(label, hint, inputView string, focused bool, width int) string {
	var b strings.Builder
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

	if focused {
		b.WriteString("  " + fieldFocusedValueStyle.Render(inputView) + "\n")
	} else {
		b.WriteString("  " + fieldBlurredValueStyle.Render(inputView) + "\n")
	}

	b.WriteString("  " + sectionUnderline(focused, width+4) + "\n\n")
	return b.String()
}

// renderToggle renders an on/off toggle field with the underline pattern.
func renderToggle(label string, on bool, focused bool, width int) string {
	var b strings.Builder
	var labelStr string
	if focused {
		labelStr = formLabelStyle.Copy().Foreground(colorPrimary).Render(label)
	} else {
		labelStr = formLabelStyle.Render(label)
	}
	b.WriteString("  " + labelStr + "\n")

	var badge string
	if on {
		badge = badgeActiveStyle.Render(" ON — active ")
	} else {
		badge = badgeInactiveStyle.Render(" OFF — inactive ")
	}
	b.WriteString("  " + badge + formHintStyle.Render("  press space to toggle") + "\n")
	b.WriteString("  " + sectionUnderline(focused, width+4) + "\n\n")
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
