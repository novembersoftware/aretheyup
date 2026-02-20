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
	viewConfirm      // delete confirmation
)

// --- Messages ---

type servicesLoadedMsg struct {
	rows []storage.ManageServiceRow
	err  error
}

type servicesSavedMsg struct{}
type serviceDeletedMsg struct{}
type errMsg struct{ err error }

// --- Root model ---

type Model struct {
	store  *storage.Storage
	width  int
	height int

	current view

	list    listModel
	detail  detailModel
	form    formModel
	confirm confirmModel

	status    string
	statusErr bool
}

func New(store *storage.Storage) Model {
	m := Model{
		store:   store,
		current: viewList,
	}
	m.list = newListModel()
	return m
}

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
		m.status = "Saved."
		m.statusErr = false
		m.current = viewList
		return m, m.loadServices()

	case serviceDeletedMsg:
		m.status = "Service deleted."
		m.statusErr = false
		m.current = viewList
		return m, m.loadServices()

	case errMsg:
		m.status = msg.err.Error()
		m.statusErr = true
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.current == viewList && msg.String() == "q" && !m.list.searching {
			return m, tea.Quit
		}
	}

	switch m.current {
	case viewList:
		return m.updateList(msg)
	case viewDetail:
		return m.updateDetail(msg)
	case viewCreate, viewEdit:
		return m.updateForm(msg)
	case viewConfirm:
		return m.updateConfirm(msg)
	}

	return m, nil
}

// updateList handles messages on the service list screen.
func (m Model) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	// While search is active, route everything to the list so nav keys type
	// into the search box instead of triggering navigation.
	if m.list.searching {
		var cmd tea.Cmd
		m.list, cmd = m.list.update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "n":
			m.form = newFormModel(nil, nil)
			m.form.setSize(m.width, m.height)
			m.current = viewCreate
			return m, nil

		case "enter":
			if sel, ok := m.list.selected(); ok {
				svc := sel.Service
				pc, _ := m.store.GetProbeConfig(context.Background(), svc.ID)
				m.detail = newDetailModel(&svc, pc)
				m.detail.setSize(m.width, m.height)
				m.current = viewDetail
			}
			return m, nil

		case "e":
			if sel, ok := m.list.selected(); ok {
				svc := sel.Service
				pc, _ := m.store.GetProbeConfig(context.Background(), svc.ID)
				m.form = newFormModel(&svc, pc)
				m.form.setSize(m.width, m.height)
				m.current = viewEdit
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

// updateDetail handles messages on the service detail screen.
func (m Model) updateDetail(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.current = viewList
			return m, nil
		case "e":
			svc := m.detail.service
			pc := m.detail.probeConfig
			m.form = newFormModel(svc, pc)
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

// updateForm handles messages on the create/edit form.
func (m Model) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.form, cmd = m.form.update(msg)

	switch m.form.result {
	case formResultSave:
		m.form.result = formResultNone
		svc := m.form.toService()
		pc := m.form.toProbeConfig()
		if m.current == viewCreate {
			return m, m.cmdCreate(svc, pc)
		}
		return m, m.cmdUpdate(svc, pc)
	case formResultCancel:
		m.form.result = formResultNone
		m.current = viewList
	}

	return m, cmd
}

// updateConfirm handles messages on the delete confirmation dialog.
func (m Model) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y":
			id := m.confirm.serviceID
			m.current = viewList
			return m, m.cmdDelete(id)
		case "n", "N", "esc":
			m.current = viewList
		}
	}
	return m, nil
}

// --- Async commands ---

// cmdCreate creates a service and optionally its probe config in one shot.
func (m Model) cmdCreate(svc *structs.Service, pc *structs.ProbeConfig) tea.Cmd {
	return func() tea.Msg {
		if err := m.store.CreateService(context.Background(), svc); err != nil {
			return errMsg{err}
		}
		if pc != nil {
			pc.ServiceID = svc.ID
			if err := m.store.UpsertProbeConfig(context.Background(), pc); err != nil {
				return errMsg{err}
			}
		}
		return servicesSavedMsg{}
	}
}

// cmdUpdate saves an existing service and optionally upserts its probe config.
func (m Model) cmdUpdate(svc *structs.Service, pc *structs.ProbeConfig) tea.Cmd {
	return func() tea.Msg {
		if err := m.store.UpdateService(context.Background(), svc); err != nil {
			return errMsg{err}
		}
		if pc != nil {
			pc.ServiceID = svc.ID
			if err := m.store.UpsertProbeConfig(context.Background(), pc); err != nil {
				return errMsg{err}
			}
		}
		return servicesSavedMsg{}
	}
}

func (m Model) cmdDelete(id uint) tea.Cmd {
	return func() tea.Msg {
		if err := m.store.DeleteService(context.Background(), id); err != nil {
			return errMsg{err}
		}
		return serviceDeletedMsg{}
	}
}

// --- View ---

func (m Model) View() string {
	switch m.current {
	case viewDetail:
		return m.detail.view(m.status, m.statusErr)
	case viewCreate, viewEdit:
		return m.form.view(m.status, m.statusErr)
	case viewConfirm:
		return m.confirm.view(m.width, m.height)
	default:
		return m.list.view(m.status, m.statusErr)
	}
}
