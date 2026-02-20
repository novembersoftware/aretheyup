package manage

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/novembersoftware/aretheyup/structs"
)

type detailModel struct {
	service        *structs.Service
	hasProbeConfig bool
	probeEnabled   bool
	width          int
	height         int
}

func newDetailModel(svc *structs.Service, hasProbeConfig, probeEnabled bool) detailModel {
	return detailModel{
		service:        svc,
		hasProbeConfig: hasProbeConfig,
		probeEnabled:   probeEnabled,
	}
}

func (m *detailModel) setSize(w, h int) {
	m.width = w
	m.height = h
}

func (m detailModel) view(status string, statusErr bool) string {
	svc := m.service
	var b strings.Builder

	// Header
	b.WriteString(appTitleStyle.Render("Are They Up") + appSubtitleStyle.Render("Service Detail"))
	b.WriteString("\n\n")

	// Panel
	panelWidth := m.width - 4
	if panelWidth < 40 {
		panelWidth = 40
	}

	var panel strings.Builder

	panel.WriteString(panelTitleStyle.Render(svc.Name))
	panel.WriteString("\n")
	panel.WriteString(renderField("ID", fmt.Sprintf("%d", svc.ID)))
	panel.WriteString(renderField("Slug", svc.Slug))
	panel.WriteString(renderField("Name", svc.Name))
	panel.WriteString(renderField("Description", valueOrMuted(svc.Description, "(none)")))
	panel.WriteString(renderField("Category", svc.Category))
	panel.WriteString(renderField("Homepage URL", svc.HomepageURL))

	var activeStr string
	if svc.Active {
		activeStr = badgeActiveStyle.Render("active")
	} else {
		activeStr = badgeInactiveStyle.Render("inactive")
	}
	panel.WriteString(formLabelStyle.Render("Active") + "  " + activeStr + "\n")

	var probeStr string
	if m.hasProbeConfig {
		if m.probeEnabled {
			probeStr = badgeProbeOnStyle.Render("probe on")
		} else {
			probeStr = badgeProbeOffStyle.Render("probe off")
		}
	} else {
		probeStr = badgeProbeOffStyle.Render("no probe configured")
	}
	panel.WriteString(formLabelStyle.Render("Probe") + "  " + probeStr + "\n")
	panel.WriteString("\n")

	panel.WriteString(mutedStyle.Render(fmt.Sprintf("Created:  %s", svc.CreatedAt.Format("2006-01-02 15:04:05"))) + "\n")
	panel.WriteString(mutedStyle.Render(fmt.Sprintf("Updated:  %s", svc.UpdatedAt.Format("2006-01-02 15:04:05"))) + "\n")

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

func renderField(label, value string) string {
	return formLabelStyle.Render(label) + "  " + formValueStyle.Render(value) + "\n"
}

func valueOrMuted(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return lipgloss.NewStyle().Foreground(colorMuted).Italic(true).Render(fallback)
	}
	return s
}
