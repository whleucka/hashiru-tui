package tui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/whleucka/hashiru-tui/internal/hashiru"
	"github.com/whleucka/hashiru-tui/internal/system"
)

// State is what several areas show, loaded once by the shell.
type State struct {
	Status     *hashiru.Status
	StatusErr  error
	StatusBusy bool

	Report     *hashiru.Report
	ReportErr  error
	ReportBusy bool

	Pending     *system.Updates
	PendingBusy bool

	News     []system.NewsItem
	NewsErr  error
	NewsBusy bool

	Conn    *system.Connectivity
	Sys     *system.SysInfo
	Machine *system.Machine
}

// UnreadNews counts news published since the machine last updated.
func (st *State) UnreadNews() int {
	if st.Status == nil {
		return 0
	}
	return system.Unread(st.News, st.Status.Updated)
}

type (
	statusMsg struct {
		s   hashiru.Status
		err error
	}
	reportMsg struct {
		r   hashiru.Report
		err error
	}
	pendingMsg struct{ u system.Updates }
	connMsg    struct{ c system.Connectivity }
	sysMsg     struct{ si system.SysInfo }
	machineMsg struct{ m system.Machine }
	newsMsg    struct {
		items []system.NewsItem
		err   error
	}
)

// status fetches upstream, which can take ~10s offline, so allow a bit more.
const (
	statusTimeout = 20 * time.Second
	doctorTimeout = 60 * time.Second
	// checkupdates syncs the package databases, so it needs the network.
	pendingTimeout = 90 * time.Second
	newsTimeout    = 15 * time.Second
	connTimeout    = 10 * time.Second
	machineTimeout = 10 * time.Second
	paneTimeout    = 5 * time.Second
)

// load reloads everything in parallel.
func (st *State) load(opts Options) tea.Cmd {
	src := opts.Source
	st.StatusBusy, st.ReportBusy, st.PendingBusy, st.NewsBusy = true, true, true, true
	return tea.Batch(
		func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), statusTimeout)
			defer cancel()
			s, err := src.Status(ctx)
			return statusMsg{s, err}
		},
		func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), doctorTimeout)
			defer cancel()
			r, err := src.Doctor(ctx)
			return reportMsg{r, err}
		},
		func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), pendingTimeout)
			defer cancel()
			return pendingMsg{system.Pending(ctx, opts.Probe)}
		},
		func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), newsTimeout)
			defer cancel()
			items, err := system.FetchNews(ctx, opts.Get)
			return newsMsg{items, err}
		},
		func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), connTimeout)
			defer cancel()
			return connMsg{system.ReadConnectivity(ctx, opts.Probe)}
		},
		func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), machineTimeout)
			defer cancel()
			return machineMsg{system.ReadMachine(ctx, opts.Probe)}
		},
		readSys(opts),
	)
}

// readSys reads uptime, memory, disk and battery: local files and a statfs,
// cheap enough for Overview to repeat every few seconds.
func readSys(opts Options) tea.Cmd {
	return func() tea.Msg { return sysMsg{system.ReadSysInfo(opts.SysRoot, opts.Statfs)} }
}

// apply records a load result. It reports whether msg was one.
func (st *State) apply(msg tea.Msg) bool {
	switch msg := msg.(type) {
	case statusMsg:
		st.StatusBusy, st.StatusErr = false, msg.err
		if msg.err == nil {
			st.Status = &msg.s
		}
	case reportMsg:
		st.ReportBusy, st.ReportErr = false, msg.err
		if msg.err == nil {
			st.Report = &msg.r
		}
	case pendingMsg:
		st.PendingBusy = false
		st.Pending = &msg.u
	case connMsg:
		st.Conn = &msg.c
	case sysMsg:
		st.Sys = &msg.si
	case machineMsg:
		st.Machine = &msg.m
	case newsMsg:
		st.NewsBusy, st.NewsErr = false, msg.err
		if msg.err == nil {
			st.News = msg.items
		}
	default:
		return false
	}
	return true
}
