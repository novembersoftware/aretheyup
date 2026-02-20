package manage

import "github.com/charmbracelet/lipgloss"

// Palette
const (
	colorPrimary   = lipgloss.Color("#818CF8") // indigo-400 (bright, readable)
	colorSecondary = lipgloss.Color("#C4B5FD") // violet-300 (lighter accent)
	colorSuccess   = lipgloss.Color("#34D399") // emerald-400
	colorDanger    = lipgloss.Color("#F87171") // red-400
	colorWarning   = lipgloss.Color("#FBBF24") // amber-400
	colorMuted     = lipgloss.Color("#6B7280") // gray-500
	colorText      = lipgloss.Color("#F9FAFB") // near white
	colorSubtext   = lipgloss.Color("#D1D5DB") // gray-300
	colorBorder    = lipgloss.Color("#374151") // gray-700
	colorHighlight = lipgloss.Color("#1E1B4B") // deep indigo bg (selected row)
)

// Base styles
var (
	// App chrome
	appTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPrimary).
			Padding(0, 1)

	appSubtitleStyle = lipgloss.NewStyle().
				Foreground(colorMuted).
				Padding(0, 1)

	// Panels / containers
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(1, 2)

	activePanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorPrimary).
				Padding(1, 2)

	// List items
	listItemStyle = lipgloss.NewStyle().
			Foreground(colorText).
			Padding(0, 1)

	listItemSelectedStyle = lipgloss.NewStyle().
				Background(colorHighlight).
				Foreground(colorSecondary).
				Bold(true).
				Padding(0, 1)

	listCursorStyle = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)

	// Labels / badges
	badgeStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Bold(true)

	badgeActiveStyle = badgeStyle.
				Background(colorSuccess).
				Foreground(lipgloss.Color("#022C22"))

	badgeInactiveStyle = badgeStyle.
				Background(colorMuted).
				Foreground(lipgloss.Color("#F9FAFB"))

	badgeProbeOnStyle = badgeStyle.
				Background(colorPrimary).
				Foreground(lipgloss.Color("#1E1B4B"))

	badgeProbeOffStyle = badgeStyle.
				Background(colorBorder).
				Foreground(colorSubtext)

	// Category tag
	categoryStyle = lipgloss.NewStyle().
			Foreground(colorSecondary).
			Italic(true)

	// Form styles — label goes ABOVE the input, so no fixed width needed
	formLabelStyle = lipgloss.NewStyle().
			Foreground(colorSecondary).
			Bold(true)

	formValueStyle = lipgloss.NewStyle().
			Foreground(colorText)

	formHintStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true)

	// inputFocusedStyle / inputBlurredStyle are kept for the search bar and toggle,
	// but form text inputs use the underline pattern instead.
	inputFocusedStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorPrimary).
				Padding(0, 1)

	inputBlurredStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorBorder).
				Padding(0, 1)

	// Form input underline styles (no box border — avoids height inconsistencies)
	fieldFocusedValueStyle = lipgloss.NewStyle().
				Foreground(colorText)

	fieldBlurredValueStyle = lipgloss.NewStyle().
				Foreground(colorSubtext)

	// Section header inside a view
	sectionHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorSecondary).
				BorderBottom(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(colorBorder).
				MarginBottom(1)

	// Title inside a panel
	panelTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorText).
			MarginBottom(1)

	// Help / keybinding bar
	helpStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Padding(0, 1)

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)

	// Search bar
	searchLabelStyle = lipgloss.NewStyle().
				Foreground(colorPrimary).
				Bold(true)

	// Error / success messages
	errorStyle = lipgloss.NewStyle().
			Foreground(colorDanger).
			Bold(true).
			Padding(0, 1)

	successStyle = lipgloss.NewStyle().
			Foreground(colorSuccess).
			Bold(true).
			Padding(0, 1)

	// Danger text (e.g. delete confirmation)
	dangerStyle = lipgloss.NewStyle().
			Foreground(colorDanger).
			Bold(true)

	// Muted / secondary text
	mutedStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	// Dialog box
	dialogStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorDanger).
			Padding(1, 3).
			Width(50)
)

// helpEntry renders a single key+description pair for the help bar.
func helpEntry(key, desc string) string {
	return helpKeyStyle.Render(key) + helpStyle.Render(" "+desc)
}
