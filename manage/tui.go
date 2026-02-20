package manage

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/novembersoftware/aretheyup/storage"
	"github.com/novembersoftware/aretheyup/structs"
)

// view is the current screen being rendered.
type view int

const (
	viewList    view = iota
	viewDetail       // service detail panel
	viewCreate       // create service form
	viewEdit         // edit service form
	viewProbe        // probe config form
	viewConfirm      // delete confirmation
)

// --- Messages ---

// servicesLoadedMsg carries a fresh list of services from the DB.
type servicesLoadedMsg struct {
	rows []storage.ManageServiceRow
	err  error
}

// servicesSavedMsg signals that a create/update succeeded.
type servicesSavedMsg struct{}

// serviceDeletedMsg signals that a delete succeeded.
type serviceDeletedMsg struct{}

// probeSavedMsg signals that a probe config upsert succeeded.
type probeSavedMsg struct{}

// errMsg wraps an async error.
type errMsg struct{ err error }

// --- Root model ---

// Model is the root Bubble Tea model. It owns the current view and all sub-models.
type Model struct {
	store  *storage.Storage
	width  int
	height int

	// current active view
	current view

	// sub-models
	list    listModel
	detail  detailModel
	form    formModel
	probe   probeModel
	confirm confirmModel

	// transient status line
	status    string
	statusErr bool
}

// New builds the root model and triggers the initial data load.
func New(store *storage.Storage) Model {
	m := Model{
		store:   store,
		current: viewList,
	}
	m.list = newListModel()
	return m
}

// Start launches the Bubbletea program.
func Start(store *storage.Storage) error {
	m := New(store)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// --- Init ---

func (m Model) Init() tea.Cmd {
	return m.loadServices()
}

// loadServices fetches all services from the DB asynchronously.
func (m Model) loadServices() tea.Cmd {
	return func() tea.Msg {
		rows, err := m.store.GetAllServicesForManage(context.Background())
		return servicesLoadedMsg{rows: rows, err: err}
	}
}

// --- Update ---

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.setSize(msg.Width, msg.Height)
		m.detail.setSize(msg.Width, msg.Height)
		m.form.setSize(msg.Width, msg.Height)
		m.probe.setSize(msg.Width, msg.Height)
		return m, nil

	case servicesLoadedMsg:
		if msg.err != nil {
			m.status = "Error loading services: " + msg.err.Error()
			m.statusErr = true
			return m, nil
		}
		m.list.setServices(msg.rows)
		m.status = ""
		return m, nil

	case servicesSavedMsg:
		m.status = "Service saved."
		m.statusErr = false
		m.current = viewList
		return m, m.loadServices()

	case serviceDeletedMsg:
		m.status = "Service deleted."
		m.statusErr = false
		m.current = viewList
		return m, m.loadServices()

	case probeSavedMsg:
		m.status = "Probe config saved."
		m.statusErr = false
		m.current = viewList
		return m, m.loadServices()

	case errMsg:
		m.status = msg.err.Error()
		m.statusErr = true
		return m, nil

	case tea.KeyMsg:
		// Global quit — never intercept when the list search box is active
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.current == viewList && msg.String() == "q" && !m.list.searching {
			return m, tea.Quit
		}
	}

	// Delegate to the active sub-model
	switch m.current {
	case viewList:
		return m.updateList(msg)
	case viewDetail:
		return m.updateDetail(msg)
	case viewCreate, viewEdit:
		return m.updateForm(msg)
	case viewProbe:
		return m.updateProbe(msg)
	case viewConfirm:
		return m.updateConfirm(msg)
	}

	return m, nil
}

// updateList handles messages while the list view is active.
func (m Model) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	// While search is active, send everything straight to the list model so
	// that characters like 'e', 'n', 'd', 'p', 'q' type into the search box
	// instead of triggering navigation actions.
	if m.list.searching {
		var cmd tea.Cmd
		m.list, cmd = m.list.update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "n":
			m.form = newFormModel(nil)
			m.form.setSize(m.width, m.height)
			m.current = viewCreate
			return m, nil
		case "enter":
			if sel, ok := m.list.selected(); ok {
				svc := sel.Service
				m.detail = newDetailModel(&svc, sel.HasProbeConfig, sel.ProbeEnabled)
				m.detail.setSize(m.width, m.height)
				m.current = viewDetail
			}
			return m, nil
		case "e":
			if sel, ok := m.list.selected(); ok {
				svc := sel.Service
				m.form = newFormModel(&svc)
				m.form.setSize(m.width, m.height)
				m.current = viewEdit
			}
			return m, nil
		case "p":
			if sel, ok := m.list.selected(); ok {
				svc := sel.Service
				pc, _ := m.store.GetProbeConfig(context.Background(), svc.ID)
				m.probe = newProbeModel(svc.ID, pc)
				m.probe.setSize(m.width, m.height)
				m.current = viewProbe
			}
			return m, nil
		case "d":
			if sel, ok := m.list.selected(); ok {
				svc := sel.Service
				m.confirm = newConfirmModel(svc.ID, svc.Name)
				m.current = viewConfirm
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.update(msg)
	return m, cmd
}

// updateDetail handles messages while the detail view is active.
func (m Model) updateDetail(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.current = viewList
			return m, nil
		case "e":
			svc := m.detail.service
			m.form = newFormModel(svc)
			m.form.setSize(m.width, m.height)
			m.current = viewEdit
			return m, nil
		case "d":
			m.confirm = newConfirmModel(m.detail.service.ID, m.detail.service.Name)
			m.current = viewConfirm
			return m, nil
		}
	}
	return m, nil
}

// updateForm handles messages while the create/edit form is active.
func (m Model) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.form, cmd = m.form.update(msg)

	switch m.form.result {
	case formResultSave:
		m.form.result = formResultNone
		svc := m.form.toService()
		if m.current == viewCreate {
			return m, m.cmdCreateService(svc)
		}
		return m, m.cmdUpdateService(svc)
	case formResultCancel:
		m.form.result = formResultNone
		m.current = viewList
	}

	return m, cmd
}

// updateProbe handles messages while the probe config form is active.
func (m Model) updateProbe(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.probe, cmd = m.probe.update(msg)

	switch m.probe.result {
	case probeResultSave:
		m.probe.result = probeResultNone
		pc := m.probe.toProbeConfig()
		return m, m.cmdUpsertProbe(pc)
	case probeResultCancel:
		m.probe.result = probeResultNone
		m.current = viewList
	}

	return m, cmd
}

// updateConfirm handles messages while the delete confirmation is active.
func (m Model) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y":
			id := m.confirm.serviceID
			m.current = viewList
			return m, m.cmdDeleteService(id)
		case "n", "N", "esc":
			m.current = viewList
		}
	}
	return m, nil
}

// --- Async commands ---

func (m Model) cmdCreateService(svc *structs.Service) tea.Cmd {
	return func() tea.Msg {
		if err := m.store.CreateService(context.Background(), svc); err != nil {
			return errMsg{err}
		}
		return servicesSavedMsg{}
	}
}

func (m Model) cmdUpdateService(svc *structs.Service) tea.Cmd {
	return func() tea.Msg {
		if err := m.store.UpdateService(context.Background(), svc); err != nil {
			return errMsg{err}
		}
		return servicesSavedMsg{}
	}
}

func (m Model) cmdDeleteService(id uint) tea.Cmd {
	return func() tea.Msg {
		if err := m.store.DeleteService(context.Background(), id); err != nil {
			return errMsg{err}
		}
		return serviceDeletedMsg{}
	}
}

func (m Model) cmdUpsertProbe(pc *structs.ProbeConfig) tea.Cmd {
	return func() tea.Msg {
		if err := m.store.UpsertProbeConfig(context.Background(), pc); err != nil {
			return errMsg{err}
		}
		return probeSavedMsg{}
	}
}

// --- View ---

func (m Model) View() string {
	switch m.current {
	case viewDetail:
		return m.detail.view(m.status, m.statusErr)
	case viewCreate, viewEdit:
		return m.form.view(m.status, m.statusErr)
	case viewProbe:
		return m.probe.view(m.status, m.statusErr)
	case viewConfirm:
		return m.confirm.view(m.width, m.height)
	default:
		return m.list.view(m.status, m.statusErr)
	}
}
