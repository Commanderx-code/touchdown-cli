package main

import (
	"fmt"
	"strings"
)

func (m model) teamLines() []string {
	lines := []string{}
	if m.teamNotice != "" {
		lines = append(lines, accentStyle.Render(m.teamNotice))
	}
	if m.teamSeason != m.season || len(m.teamData.Events) == 0 {
		if m.teamError != "" {
			return append(lines, liveStyle.Render("Schedule: "+m.teamError))
		}
		return append(lines, dimStyle.Render("Loading season schedule…"))
	}
	team := m.favorite
	if !m.teamData.CachedAt.IsZero() {
		lines = append(lines, dimStyle.Render("CACHED · last saved "+m.teamData.CachedAt.Local().Format("Jan 2, 3:04 PM")))
	}
	wins, losses, ties := 0, 0, 0
	var next *scheduledGame
	for i := range m.teamData.Events {
		g := &m.teamData.Events[i]
		_, _, score := g.matchup(team)
		if strings.HasPrefix(score, "W ") {
			wins++
		} else if strings.HasPrefix(score, "L ") {
			losses++
		} else if strings.HasPrefix(score, "T ") {
			ties++
		}
		if next == nil && g.state() != "post" {
			next = g
		}
	}
	lines = append(lines, sectionStyle.Render(fmt.Sprintf("%s  %d-%d-%d  ·  %d GAMES", team, wins, losses, ties, len(m.teamData.Events))))
	if next != nil {
		opp, loc, _ := next.matchup(team)
		when := next.dateTime().Local().Format("Mon Jan 2, 3:04 PM MST")
		lines = append(lines, accentStyle.Render(fmt.Sprintf("NEXT  Week %d  %s %s  ·  %s", next.Week.Number, loc, opp, when)))
	} else {
		lines = append(lines, dimStyle.Render("Season schedule complete"))
	}
	if m.teamData.ByeWeek > 0 {
		lines = append(lines, dimStyle.Render(fmt.Sprintf("BYE  Week %d", m.teamData.ByeWeek)))
	}
	lines = append(lines, "", sectionStyle.Render("SEASON LEADERS"))
	if len(m.teamLeaders) == 0 {
		message := "Loading player totals…"
		if wins+losses+ties == 0 {
			message = "Player totals appear after the first game."
		}
		if m.teamLeaderError != "" {
			message = m.teamLeaderError
		}
		lines = append(lines, dimStyle.Render(message))
	} else {
		for _, leader := range m.teamLeaders {
			lines = append(lines, textStyle.Render(fmt.Sprintf("%-9s %-23s %4d YDS  %d TD", strings.ToUpper(leader.Category), clipText(leader.Player, 23), leader.Yards, leader.Touchdowns)))
		}
		if m.teamLeaderError != "" {
			lines = append(lines, dimStyle.Render("Partial totals: "+m.teamLeaderError))
		}
	}
	lines = append(lines, "", sectionStyle.Render("FULL SCHEDULE  ·  ENTER OPENS GAME"))
	for i, g := range m.teamData.Events {
		opp, loc, score := g.matchup(team)
		date := g.dateTime().Local().Format("Jan 2")
		if g.dateTime().IsZero() {
			date = "TBD"
		}
		if score == "" {
			if g.state() == "in" {
				score = "LIVE"
			} else {
				score = g.dateTime().Local().Format("Mon 3:04 PM")
			}
		}
		marker := " "
		if i == m.teamSelected {
			marker = "›"
		}
		row := fmt.Sprintf("%s W%02d  %-6s  %s %-3s  %s", marker, g.Week.Number, date, loc, opp, score)
		if i == m.teamSelected {
			lines = append(lines, selectedStyle.Render(row))
		} else if g.state() == "in" {
			lines = append(lines, liveStyle.Render(row))
		} else {
			lines = append(lines, textStyle.Render(row))
		}
	}
	if m.teamError != "" {
		lines = append(lines, liveStyle.Render("Refresh failed: "+m.teamError))
	}
	return lines
}

func (m model) teamPageSize() int {
	if m.compact || m.height > 0 && m.height < 16 {
		return max(1, m.height-4)
	}
	if m.height > 0 {
		return max(1, m.height-8)
	}
	return len(m.teamLines())
}

func (m model) maxTeamScroll() int { return max(0, len(m.teamLines())-m.teamPageSize()) }

func (m model) teamSelectedLine() int {
	for i, line := range m.teamLines() {
		if strings.Contains(line, "FULL SCHEDULE") {
			return i + 1 + m.teamSelected
		}
	}
	return 0
}

func (m model) renderTeam(inner int) string {
	name := m.teamData.Team.DisplayName
	if name == "" {
		name = m.favorite + " SEASON"
	}
	lines := []string{accentStyle.Render(fmt.Sprintf("%s  ·  %d", strings.ToUpper(name), m.season)), borderStyle.Render(strings.Repeat("─", inner))}
	rows := m.teamLines()
	start := min(m.teamScroll, m.maxTeamScroll())
	for _, row := range rows[start:min(len(rows), start+m.teamPageSize())] {
		lines = append(lines, clipStyled(row, inner))
	}
	return centeredFrame(lines, inner, m.width)
}
