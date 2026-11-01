package main

import tea "github.com/charmbracelet/bubbletea"

// backTarget is one screen Esc can return to while browsing schedules.
// Screen is one of home, dashboard, picker, season or overview; Team is set
// for season targets.
type backTarget struct {
	Screen string
	Team   string
}

func (m model) push(t backTarget) model {
	m.history = append(m.history, t)
	return m
}

// canGoBack reports whether Esc should pop the history stack rather than
// run the default Esc handling for the current screen.
func (m model) canGoBack() bool {
	if len(m.history) == 0 {
		return false
	}
	if m.showPicker || m.showOverview || (m.showTeam && !m.showDetail) {
		return true
	}
	onDashboard := !m.showHome && !m.showDetail && !m.showStandings && !m.showTeam && !m.showSettings
	return onDashboard && m.history[len(m.history)-1].Screen == "overview"
}

// back pops the most recent target and shows it.
func (m model) back() (model, tea.Cmd) {
	if len(m.history) == 0 {
		return m, nil
	}
	target := m.history[len(m.history)-1]
	m.history = m.history[:len(m.history)-1]
	m.showPicker = false
	m.showOverview = false
	m.showTeam = false
	m.showDetail = false
	m.showStandings = false
	m.showHome = false
	switch target.Screen {
	case "home":
		m.showHome = true
	case "picker":
		m.showPicker = true
	case "season":
		return m.openTeamSeason(target.Team)
	case "overview":
		m.showOverview = true
	}
	return m, nil
}

// consumeOverview drops a season-overview return entry when the person
// leaves the dashboard for a screen with its own Esc behaviour.
func (m model) consumeOverview() model {
	if n := len(m.history); n > 0 && m.history[n-1].Screen == "overview" {
		m.history = m.history[:n-1]
	}
	return m
}

// jumpToOpponent opens the season page of the opponent in the selected game
// on the current season page, remembering the current page for Esc.
func (m model) jumpToOpponent() (model, tea.Cmd) {
	page := m.activeTeamPage()
	if page.Season != m.season || len(page.Data.Events) == 0 {
		return m, nil
	}
	event := page.Data.Events[max(0, min(m.teamSelected, len(page.Data.Events)-1))]
	opponent, _, _ := event.matchup(page.Team)
	if opponent == "" {
		return m, nil
	}
	m = m.push(backTarget{Screen: "season", Team: page.Team})
	return m.openTeamSeason(opponent)
}
