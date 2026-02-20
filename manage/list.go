package manage

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/novembersoftware/aretheyup/storage"
)

type listModel struct {
	allServices []storage.ManageServiceRow // full unfiltered list
	filtered    []storage.ManageServiceRow // currently displayed (after filter)
	cursor      int
	offset      int // scroll offset

	// search
	searching   bool
	searchInput textinput.Model

	width  int
	height int
}

func newListModel() listModel {
	si := textinput.New()
	si.Placeholder = "type to filter…"
	si.Prompt = ""
	si.TextStyle = formValueStyle
	si.Cursor.Style = listCursorStyle
	si.CharLimit = 100

	return listModel{searchInput: si}
}

func (m *listModel) setServices(rows []storage.ManageServiceRow) {
	m.allServices = rows
	m.applyFilter()
	// keep cursor in bounds
	if m.cursor >= len(m.filtered) && len(m.filtered) > 0 {
		m.cursor = len(m.filtered) - 1
	}
}

func (m *listModel) setSize(w, h int) {
	m.width = w
	m.height = h
	m.searchInput.Width = w - 20
}

func (m *listModel) applyFilter() {
	q := strings.ToLower(strings.TrimSpace(m.searchInput.Value()))
	if q == "" {
		m.filtered = m.allServices
		return
	}
	var out []storage.ManageServiceRow
	for _, svc := range m.allServices {
		if strings.Contains(strings.ToLower(svc.Name), q) ||
			strings.Contains(strings.ToLower(svc.Slug), q) {
			out = append(out, svc)
		}
	}
	m.filtered = out
}

func (m listModel) selected() (storage.ManageServiceRow, bool) {
	if len(m.filtered) == 0 {
		return storage.ManageServiceRow{}, false
	}
	return m.filtered[m.cursor], true
}

func (m listModel) update(msg tea.Msg) (listModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Enter/exit search mode
		if !m.searching {
			switch msg.String() {
			case "/":
				m.searching = true
				m.searchInput.Focus()
				return m, nil
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
					if m.cursor < m.offset {
						m.offset = m.cursor
					}
				}
				return m, nil
			case "down", "j":
				if m.cursor < len(m.filtered)-1 {
					m.cursor++
					vis := m.visibleCount()
					if m.cursor >= m.offset+vis {
						m.offset = m.cursor - vis + 1
					}
				}
				return m, nil
			}
		} else {
			// In search mode
			switch msg.String() {
			case "esc":
				m.searching = false
				m.searchInput.Blur()
				m.searchInput.SetValue("")
				m.applyFilter()
				m.cursor = 0
				m.offset = 0
				return m, nil
			case "enter":
				// Commit search, leave search bar visible but exit typing mode
				m.searching = false
				m.searchInput.Blur()
				return m, nil
			default:
				var cmd tea.Cmd
				m.searchInput, cmd = m.searchInput.Update(msg)
				m.applyFilter()
				m.cursor = 0
				m.offset = 0
				return m, cmd
			}
		}
	}
	return m, nil
}

// visibleCount returns how many list rows fit in the current terminal height.
func (m listModel) visibleCount() int {
	// Reserve: 3 header + 1 search bar + 1 count line + 1 status + 2 help + 2 padding
	reserved := 11
	n := m.height - reserved
	if n < 1 {
		n = 1
	}
	return n
}

func (m listModel) view(status string, statusErr bool) string {
	var b strings.Builder

	// ── Header ──────────────────────────────────────────────────────────────
	title := appTitleStyle.Render("Are They Up")
	subtitle := appSubtitleStyle.Render("Service Manager")
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Bottom, title, subtitle))
	b.WriteString("\n\n")

	// ── Search bar (same underline pattern as form fields) ──────────────────
	b.WriteString(m.renderSearchBar())

	// ── List ────────────────────────────────────────────────────────────────
	if len(m.filtered) == 0 {
		if m.searchInput.Value() != "" {
			b.WriteString(mutedStyle.Render("  No services match your search."))
		} else {
			b.WriteString(mutedStyle.Render("  No services found. Press [n] to create one."))
		}
		b.WriteString("\n")
	} else {
		visibleCount := m.visibleCount()
		end := m.offset + visibleCount
		if end > len(m.filtered) {
			end = len(m.filtered)
		}

		for i := m.offset; i < end; i++ {
			b.WriteString(m.renderRow(i, m.filtered[i]))
			b.WriteString("\n")
		}

		total := len(m.filtered)
		if total > visibleCount {
			b.WriteString(mutedStyle.Render(fmt.Sprintf(
				"  %d–%d of %d  (↑↓ to scroll)",
				m.offset+1, end, total,
			)))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")

	// ── Status line ─────────────────────────────────────────────────────────
	if status != "" {
		if statusErr {
			b.WriteString(errorStyle.Render("  " + status))
		} else {
			b.WriteString(successStyle.Render("  " + status))
		}
		b.WriteString("\n")
	}

	// ── Help bar ─────────────────────────────────────────────────────────────
	if m.searching {
		b.WriteString(helpStyle.Render("  " + strings.Join([]string{
			helpEntry("enter", "confirm"),
			helpEntry("esc", "clear search"),
		}, "  ·  ")))
	} else {
		entries := []string{
			helpEntry("↑↓/jk", "navigate"),
			helpEntry("enter", "detail"),
			helpEntry("/", "search"),
			helpEntry("n", "new"),
			helpEntry("e", "edit"),
			helpEntry("d", "delete"),
			helpEntry("q", "quit"),
		}
		b.WriteString(helpStyle.Render("  " + strings.Join(entries, "  ·  ")))
	}

	return b.String()
}

func (m listModel) renderRow(i int, svc storage.ManageServiceRow) string {
	selected := i == m.cursor

	cursor := "  "
	if selected {
		cursor = listCursorStyle.Render("▶ ")
	}

	// Active badge
	var activeBadge string
	if svc.Active {
		activeBadge = badgeActiveStyle.Render("active")
	} else {
		activeBadge = badgeInactiveStyle.Render("inactive")
	}

	// Probe badge
	var probeBadge string
	if svc.HasProbeConfig {
		if svc.ProbeEnabled {
			probeBadge = badgeProbeOnStyle.Render("probe on")
		} else {
			probeBadge = badgeProbeOffStyle.Render("probe off")
		}
	} else {
		probeBadge = badgeProbeOffStyle.Render("no probe")
	}

	// Name + slug (fixed widths for alignment)
	const nameW = 30
	const slugW = 24
	const catW = 14

	name := truncate(svc.Name, nameW)
	slug := truncate(svc.Slug, slugW)
	cat := categoryStyle.Render(truncate(svc.Category, catW))

	var nameStr, slugStr string
	if selected {
		nameStr = listItemSelectedStyle.Render(fmt.Sprintf("%-*s", nameW, name))
		slugStr = listItemSelectedStyle.Copy().Foreground(colorMuted).Render(fmt.Sprintf("%-*s", slugW, slug))
	} else {
		nameStr = listItemStyle.Render(fmt.Sprintf("%-*s", nameW, name))
		slugStr = listItemStyle.Copy().Foreground(colorMuted).Render(fmt.Sprintf("%-*s", slugW, slug))
	}

	catPad := lipgloss.NewStyle().Width(catW + 2).Render(cat)

	return lipgloss.JoinHorizontal(lipgloss.Center,
		cursor,
		nameStr,
		slugStr,
		catPad,
		activeBadge,
		"  ",
		probeBadge,
	)
}

// renderSearchBar renders the search field using the same underline style as the form.
func (m listModel) renderSearchBar() string {
	lineWidth := m.width - 4
	if lineWidth < 10 {
		lineWidth = 10
	}

	var b strings.Builder

	// Label row
	if m.searching {
		b.WriteString("  " + searchLabelStyle.Render("Search") + "\n")
	} else if m.searchInput.Value() != "" {
		b.WriteString("  " + searchLabelStyle.Copy().Foreground(colorMuted).Render("Search") +
			"  " + formHintStyle.Render("(filtered — press / to edit, esc to clear)") + "\n")
	} else {
		b.WriteString("  " + mutedStyle.Render("Search") +
			"  " + formHintStyle.Render("press / to search") + "\n")
	}

	// Value row
	var valueStr string
	if m.searching {
		valueStr = fieldFocusedValueStyle.Render(m.searchInput.View())
	} else if m.searchInput.Value() != "" {
		valueStr = fieldBlurredValueStyle.Render(m.searchInput.Value())
	} else {
		valueStr = mutedStyle.Render("") // empty placeholder
	}
	b.WriteString("  " + valueStr + "\n")

	// Underline row
	var underline string
	if m.searching {
		underline = lipgloss.NewStyle().Foreground(colorPrimary).Render(strings.Repeat("─", lineWidth))
	} else {
		underline = lipgloss.NewStyle().Foreground(colorBorder).Render(strings.Repeat("─", lineWidth))
	}
	b.WriteString("  " + underline + "\n\n")

	return b.String()
}

// truncate shortens s to max runes and appends "…" if needed.
func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}
