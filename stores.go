package main

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Session stores for data that several screens share. Each entry carries its
// own loading and error state; a reply for any team or player is stored, so
// nothing is ever stale or dropped.

type injuryEntry struct {
	Report  injuryReport
	Loading bool
	Error   string
}

type rosterEntry struct {
	Data    roster
	Loading bool
	Error   string
}

type playerEntry struct {
	Stats   playerStats
	Loading bool
	Error   string
}

type injuriesMsg struct {
	team   string
	report injuryReport
	err    error
}

type rosterMsg struct {
	team string
	data roster
	err  error
}

type playerMsg struct {
	id    string
	stats playerStats
	err   error
}

func (m model) injuriesFor(team string) (injuryEntry, bool) {
	entry, ok := m.injuryReports[team]
	return entry, ok
}

func (m model) rosterFor(team string) (rosterEntry, bool) {
	entry, ok := m.rosters[team]
	return entry, ok
}

func (m model) playerFor(id string) (playerEntry, bool) {
	entry, ok := m.players[id]
	return entry, ok
}

// requestInjuries starts a fetch unless one is in flight, or the report is
// already loaded and force is false.
func (m model) requestInjuries(team string, force bool) (model, tea.Cmd) {
	if team == "" {
		return m, nil
	}
	if m.injuryReports == nil {
		m.injuryReports = map[string]injuryEntry{}
	}
	entry := m.injuryReports[team]
	if entry.Loading || (!force && entry.Error == "" && entry.Report.Team != "") {
		return m, nil
	}
	entry.Loading, entry.Error = true, ""
	m.injuryReports[team] = entry
	return m, m.loadInjuries(team)
}

func (m model) requestRoster(team string, force bool) (model, tea.Cmd) {
	if team == "" {
		return m, nil
	}
	if m.rosters == nil {
		m.rosters = map[string]rosterEntry{}
	}
	entry := m.rosters[team]
	if entry.Loading || (!force && entry.Error == "" && entry.Data.Team != "") {
		return m, nil
	}
	entry.Loading, entry.Error = true, ""
	m.rosters[team] = entry
	return m, m.loadRoster(team)
}

func (m model) requestPlayer(id string, force bool) (model, tea.Cmd) {
	if id == "" {
		return m, nil
	}
	if m.players == nil {
		m.players = map[string]playerEntry{}
	}
	entry := m.players[id]
	if entry.Loading || (!force && entry.Error == "" && entry.Stats.ID != "") {
		return m, nil
	}
	entry.Loading, entry.Error = true, ""
	m.players[id] = entry
	return m, m.loadPlayer(id)
}

func (m model) loadInjuries(team string) tea.Cmd {
	return func() tea.Msg {
		report, err := m.api.fullInjuries(context.Background(), team)
		return injuriesMsg{team: team, report: report, err: err}
	}
}

func (m model) loadRoster(team string) tea.Cmd {
	return func() tea.Msg {
		data, err := m.api.roster(context.Background(), team)
		return rosterMsg{team: team, data: data, err: err}
	}
}

func (m model) loadPlayer(id string) tea.Cmd {
	return func() tea.Msg {
		stats, err := m.api.playerOverview(context.Background(), id)
		return playerMsg{id: id, stats: stats, err: err}
	}
}

func (m model) applyInjuries(msg injuriesMsg) model {
	if m.injuryReports == nil {
		m.injuryReports = map[string]injuryEntry{}
	}
	entry := m.injuryReports[msg.team]
	entry.Loading = false
	if msg.err != nil {
		entry.Error = plainText(msg.err.Error())
	} else {
		entry.Error, entry.Report = "", msg.report
	}
	m.injuryReports[msg.team] = entry
	return m
}

func (m model) applyRoster(msg rosterMsg) model {
	if m.rosters == nil {
		m.rosters = map[string]rosterEntry{}
	}
	entry := m.rosters[msg.team]
	entry.Loading = false
	if msg.err != nil {
		entry.Error = plainText(msg.err.Error())
	} else {
		entry.Error, entry.Data = "", msg.data
	}
	m.rosters[msg.team] = entry
	return m
}

func (m model) applyPlayer(msg playerMsg) model {
	if m.players == nil {
		m.players = map[string]playerEntry{}
	}
	entry := m.players[msg.id]
	entry.Loading = false
	if msg.err != nil {
		entry.Error = plainText(msg.err.Error())
	} else {
		entry.Error, entry.Stats = "", msg.stats
	}
	m.players[msg.id] = entry
	return m
}

// markStartupLoads records the fetches Init starts, so nothing requests them
// a second time before their replies arrive.
func (m model) markStartupLoads() model {
	if m.favorite != "" {
		m.injuryReports = map[string]injuryEntry{m.favorite: {Loading: true}}
	}
	return m
}

// staleLine says when data on screen is not current: an update failed, or it
// came from the disk cache. It is empty for fresh data.
func staleLine(cachedAt time.Time, failed string) string {
	saved := "last saved " + cachedAt.Local().Format("Jan 2, 3:04 PM")
	switch {
	case failed != "" && !cachedAt.IsZero():
		return "UPDATE FAILED · " + saved
	case failed != "":
		return "UPDATE FAILED · showing the last data loaded"
	case !cachedAt.IsZero():
		return "CACHED · " + saved
	}
	return ""
}
