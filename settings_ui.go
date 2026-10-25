package main

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) openSettings() model {
	m.showSettings = true
	m.settingsFromHome = m.showHome
	m.settingsError = ""
	for i, team := range nflTeams {
		if team.abbreviation == m.favorite {
			m.settingsTeam = i
			break
		}
	}
	m.settingsTheme = max(0, themeIndex(m.theme))
	m.settingsScroll = max(0, min(m.settingsTeam-m.settingsPageSize()/2, len(nflTeams)-m.settingsPageSize()))
	return m
}

func (m model) updateSettingsKey(key string) (model, tea.Cmd) {
	switch key {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc", "backspace":
		_ = applyTheme(m.theme)
		m.showSettings = false
		m.showHome = m.settingsFromHome
		m.settingsError = ""
	case "up", "k":
		m = m.scrollBy(-1)
	case "down", "j":
		m = m.scrollBy(1)
	case "pgup":
		m = m.scrollBy(-m.settingsPageSize())
	case "pgdown":
		m = m.scrollBy(m.settingsPageSize())
	case "left", "h":
		m.settingsTheme = (m.settingsTheme + len(themeNames) - 1) % len(themeNames)
		_ = applyTheme(themeNames[m.settingsTheme])
	case "right", "l":
		m.settingsTheme = (m.settingsTheme + 1) % len(themeNames)
		_ = applyTheme(themeNames[m.settingsTheme])
	case "enter":
		team := nflTeams[m.settingsTeam]
		chosenTheme := themeNames[m.settingsTheme]
		if err := saveSettings(m.configPath, team.abbreviation, chosenTheme); err != nil {
			m.settingsError = err.Error()
			return m, nil
		}
		m.favorite = team.abbreviation
		m.theme = chosenTheme
		m.showSettings = false
		if m.showDetail || m.showStandings || m.showTeam {
			m.showHome = true
			m.showDetail = false
			m.showStandings = false
			m.showTeam = false
			m.detailID = ""
		} else {
			m.showHome = m.settingsFromHome
		}
		m.settingsError = ""
		m.selected = 0
		m.teamSeason = 0
		m.teamUpdated = time.Time{}
		m.teamData = teamSchedule{}
		m.teamLoading = false
		m.teamError = ""
		m.teamLeaders = nil
		m.teamLeadersReady = false
		m.teamLeadersLoading = false
		m.teamSelected = 0
		m.teamScroll = 0
		m.standingsConference = 0
		for i, group := range m.standings.Children {
			if group.Abbreviation == team.conference {
				m.standingsConference = i
				break
			}
		}
	}
	return m, nil
}

func (m model) settingsPageSize() int {
	height := m.height
	if height <= 0 {
		height = 24
	}
	return max(1, height-10)
}

func (m model) renderSettings(width int) string {
	height := m.height
	if height <= 0 {
		height = 24
	}
	selected := nflTeams[m.settingsTeam]
	theme := strings.ToUpper(themeNames[m.settingsTheme])
	if height < 14 || width < 30 {
		lines := []string{
			accentStyle.Render("◆ TOUCHDOWN SETTINGS"),
			sectionStyle.Render(fmt.Sprintf("TEAM  %s %s (%d/32)", selected.city, selected.nickname, m.settingsTeam+1)),
			textStyle.Render("THEME  " + theme),
			dimStyle.Render("↑/↓ team · ←/→ theme"),
			dimStyle.Render("Enter save · Esc cancel · ? help"),
		}
		if m.settingsError != "" {
			lines = append(lines, liveStyle.Render(m.settingsError))
		}
		for i := range lines {
			lines[i] = clipStyled(lines[i], width)
		}
		return strings.Join(lines, "\n")
	}
	inner := min(68, width-4)
	start := min(m.settingsScroll, max(0, len(nflTeams)-m.settingsPageSize()))
	end := min(len(nflTeams), start+m.settingsPageSize())
	lines := []string{
		accentStyle.Render("◆  TOUCHDOWN SETTINGS"),
		dimStyle.Render(fmt.Sprintf("FAVORITE TEAM  ·  %d–%d OF %d", start+1, end, len(nflTeams))),
		borderStyle.Render(strings.Repeat("─", inner)),
	}
	for i := start; i < end; i++ {
		team := nflTeams[i]
		marker := " "
		if i == m.settingsTeam {
			marker = "›"
		}
		row := fmt.Sprintf("%s %-3s  %s %s", marker, team.abbreviation, team.city, team.nickname)
		if i == m.settingsTeam {
			lines = append(lines, selectedStyle.Render(padLine(row, inner)))
		} else {
			lines = append(lines, textStyle.Render(row))
		}
	}
	lines = append(lines, borderStyle.Render(strings.Repeat("─", inner)), sectionStyle.Render("THEME  ←  "+theme+"  →"))
	if m.settingsError != "" {
		lines = append(lines, liveStyle.Render(clipText(m.settingsError, inner)))
	}
	lines = append(lines, dimStyle.Render("↑/↓ team · ←/→ theme · Enter save · Esc cancel"))
	return centerBlock(framedLines(lines, inner), width)
}
