package main

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
)

// teamPage is everything the season page needs for one team. The favorite's
// copy lives in the model's existing fields; any other team lives in the
// viewed slot.
type teamPage struct {
	Team           string
	Season         int
	Data           teamSchedule
	Leaders        []seasonLeader
	LeaderError    string
	Error          string
	Loading        bool
	LeadersLoading bool
	LeadersReady   bool
}

// showingFavorite reports whether the season page is the favorite's.
func (m model) showingFavorite() bool {
	return m.viewTeam == "" || m.viewTeam == m.favorite
}

// activeTeamPage is the team the season page currently renders.
func (m model) activeTeamPage() teamPage {
	if m.showingFavorite() {
		return teamPage{
			Team: m.favorite, Season: m.teamSeason, Data: m.teamData, Leaders: m.teamLeaders,
			LeaderError: m.teamLeaderError, Error: m.teamError, Loading: m.teamLoading,
			LeadersLoading: m.teamLeadersLoading, LeadersReady: m.teamLeadersReady,
		}
	}
	return m.view
}

func (m model) loadTeamFor(team string) tea.Cmd {
	season := m.season
	return func() tea.Msg {
		data, err := m.api.teamSchedule(context.Background(), team, season)
		return teamScheduleMsg{season, team, data, err}
	}
}

func (m model) loadTeamLeadersFor(team string, data teamSchedule) tea.Cmd {
	season := m.season
	return func() tea.Msg {
		leaders, err := m.api.teamLeaders(context.Background(), team, data)
		return teamLeadersMsg{season, team, leaders, err}
	}
}

// openTeamSeason shows a team's season page, loading its schedule and
// leaders when they are not already in the viewed slot.
func (m model) openTeamSeason(team string) (model, tea.Cmd) {
	if team == "" || team == m.favorite {
		m.viewTeam = ""
		return m.openFavoriteSeason("")
	}
	m.showHome = false
	m.showDetail = false
	m.showStandings = false
	m.showTeam = true
	m.teamScroll = 0
	m.teamSelected = 0
	m.teamNotice = ""
	m.viewTeam = team
	if m.season <= 0 {
		return m, nil
	}
	if m.view.Team != team || m.view.Season != m.season {
		m.view = teamPage{Team: team, Season: m.season, Loading: true}
		return m, m.loadTeamFor(team)
	}
	if len(m.view.Data.Events) > 0 && !m.view.LeadersReady && !m.view.LeadersLoading {
		m.view.LeadersLoading = true
		return m, m.loadTeamLeadersFor(team, m.view.Data)
	}
	return m, nil
}

// applyViewSchedule stores a viewed team's schedule and starts its leaders.
func (m model) applyViewSchedule(msg teamScheduleMsg) (model, tea.Cmd) {
	m.view.Loading = false
	if msg.err != nil {
		m.view.Error = msg.err.Error()
		return m, nil
	}
	m.view.Data = msg.data
	m.view.Error = ""
	m.view.Leaders = nil
	m.view.LeaderError = ""
	m.view.LeadersReady = false
	m.teamSelected = max(0, min(m.teamSelected, len(msg.data.Events)-1))
	if m.showTeam {
		m.view.LeadersLoading = true
		return m, m.loadTeamLeadersFor(msg.team, msg.data)
	}
	return m, nil
}

func (m model) applyViewLeaders(msg teamLeadersMsg) model {
	m.view.Leaders = msg.leaders
	m.view.LeadersReady = true
	m.view.LeadersLoading = false
	m.view.LeaderError = ""
	if msg.err != nil {
		m.view.LeaderError = msg.err.Error()
	}
	return m
}
