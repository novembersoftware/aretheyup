package manage

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/novembersoftware/aretheyup/structs"
)

type detailModel struct {
	service     *structs.Service
	probeConfig *structs.ProbeConfig // nil if not configured
	width       int
	height      int
}

func newDetailModel(svc *structs.Service, pc *structs.ProbeConfig) detailModel {
	return detailModel{
		service:     svc,
		probeConfig: pc,
	}
}

func (m *detailModel) setSize(w, h int) {
	m.width = w
	m.height = h
}

func (m detailModel) view(status string, statusErr bool) string {
	svc := m.service
	var b strings.Builder

	b.WriteString(appTitleStyle.Render("Are They Up") + appSubtitleStyle.Render("Service Detail"))
	b.WriteString("\n\n")

	panelWidth := m.width - 4
	if panelWidth < 40 {
		panelWidth = 40
	}

	var panel strings.Builder

	// ── Service fields ───────────────────────────────────────────────────────
	panel.WriteString(panelTitleStyle.Render(svc.Name) + "\n")
	panel.WriteString(detailField("ID", fmt.Sprintf("%d", svc.ID)))
	panel.WriteString(detailField("Slug", svc.Slug))
	panel.WriteString(detailField("Description", valueOrMuted(svc.Description, "(none)")))
	panel.WriteString(detailField("Category", svc.Category))
	panel.WriteString(detailField("Homepage", svc.HomepageURL))

	var activeStr string
	if svc.Active {
		activeStr = badgeActiveStyle.Render("active")
	} else {
		activeStr = badgeInactiveStyle.Render("inactive")
	}
	panel.WriteString(detailLabelStyle().Render("Active") + "  " + activeStr + "\n")
	panel.WriteString("\n")
	panel.WriteString(mutedStyle.Render("Created:  "+svc.CreatedAt.Format("2006-01-02 15:04")) + "\n")
	panel.WriteString(mutedStyle.Render("Updated:  "+svc.UpdatedAt.Format("2006-01-02 15:04")) + "\n")

	// ── Probe config ─────────────────────────────────────────────────────────
	lineWidth := panelWidth - 6
	if lineWidth < 10 {
		lineWidth = 10
	}

	panel.WriteString("\n")
	panel.WriteString(probeSectionDivider(lineWidth) + "\n\n")

	if m.probeConfig == nil {
		panel.WriteString(mutedStyle.Render("No probe config — edit this service to add one.") + "\n")
	} else {
		pc := m.probeConfig

		var enabledStr string
		if pc.Enabled {
			enabledStr = badgeProbeOnStyle.Render("enabled")
		} else {
			enabledStr = badgeProbeOffStyle.Render("disabled")
		}
		panel.WriteString(detailLabelStyle().Render("Status") + "  " + enabledStr + "\n")
		panel.WriteString(detailField("URL", pc.URL))
		panel.WriteString(detailField("Method", pc.Method))
		panel.WriteString(detailField("Interval", fmt.Sprintf("%ds", pc.IntervalSeconds)))
		panel.WriteString(detailField("Timeout", fmt.Sprintf("%ds", pc.TimeoutSeconds)))
		panel.WriteString(detailField("Expected", fmt.Sprintf("HTTP %d", pc.ExpectedStatus)))
		panel.WriteString("\n")
		panel.WriteString(mutedStyle.Render("Configured:  "+pc.CreatedAt.Format("2006-01-02 15:04")) + "\n")
	}

	b.WriteString(activePanelStyle.Width(panelWidth).Render(panel.String()))
	b.WriteString("\n\n")

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
		helpEntry("e", "edit"),
		helpEntry("d", "delete"),
		helpEntry("esc", "back"),
	}
	b.WriteString(helpStyle.Render("  " + strings.Join(entries, "  ·  ")))

	return b.String()
}

func detailLabelStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(colorSecondary).Bold(true).Width(14)
}

func detailField(label, value string) string {
	return detailLabelStyle().Render(label) + "  " + formValueStyle.Render(value) + "\n"
}

func valueOrMuted(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return lipgloss.NewStyle().Foreground(colorMuted).Italic(true).Render(fallback)
	}
	return s
}
