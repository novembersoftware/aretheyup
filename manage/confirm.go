package manage

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type confirmModel struct {
	serviceID   uint
	serviceName string
}

func newConfirmModel(serviceID uint, serviceName string) confirmModel {
	return confirmModel{
		serviceID:   serviceID,
		serviceName: serviceName,
	}
}

func (m confirmModel) view(width, height int) string {
	var body strings.Builder
	body.WriteString(dangerStyle.Render("Delete Service") + "\n\n")
	body.WriteString("Are you sure you want to delete:\n\n")
	body.WriteString("  " + formValueStyle.Render(m.serviceName) + "\n\n")
	body.WriteString(mutedStyle.Render("This action cannot be undone.") + "\n\n")
	body.WriteString(helpKeyStyle.Render("y") + helpStyle.Render(" yes, delete") +
		"   " +
		helpKeyStyle.Render("n/esc") + helpStyle.Render(" cancel"))

	dialog := dialogStyle.Render(body.String())

	// Center vertically and horizontally
	dialogLines := strings.Count(dialog, "\n") + 1
	topPad := (height - dialogLines) / 2
	if topPad < 0 {
		topPad = 0
	}

	dialogWidth := lipgloss.Width(dialog)
	leftPad := (width - dialogWidth) / 2
	if leftPad < 0 {
		leftPad = 0
	}

	var out strings.Builder
	for i := 0; i < topPad; i++ {
		out.WriteString("\n")
	}
	for _, line := range strings.Split(dialog, "\n") {
		out.WriteString(strings.Repeat(" ", leftPad) + line + "\n")
	}
	return out.String()
}
