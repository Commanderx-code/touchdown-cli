package main

import (
	"fmt"
	"strings"
)

// scheduleLines is the schedule tab: record, next game, leaders, every matchup.
func (m model) scheduleLines() []string {
	page := m.activeTeamPage()
	lines := []string{}
	if m.teamNotice != "" {
		lines = append(lines, accentStyle.Render(m.teamNotice))
	}
	if page.Season != m.season || len(page.Data.Events) == 0 {
		if page.Error != "" {
			return append(lines, liveStyle.Render("Schedule: "+page.Error))
		}
		return append(lines, dimStyle.Render("Loading season schedule…"))
	}
	team := page.Team
	if !page.Data.CachedAt.IsZero() {
		lines = append(lines, dimStyle.Render("CACHED · last saved "+page.Data.CachedAt.Local().Format("Jan 2, 3:04 PM")))
	}
	wins, losses, ties := 0, 0, 0
	var next *scheduledGame
	for i := range page.Data.Events {
		g := &page.Data.Events[i]
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
	lines = append(lines, sectionStyle.Render(fmt.Sprintf("%s  %d-%d-%d  ·  %d GAMES", team, wins, losses, ties, len(page.Data.Events))))
	if next != nil {
		opp, loc, _ := next.matchup(team)
		when := next.dateTime().Local().Format("Mon Jan 2, 3:04 PM MST")
		lines = append(lines, accentStyle.Render(fmt.Sprintf("NEXT  Week %d  %s %s  ·  %s", next.Week.Number, loc, opp, when)))
	} else {
		lines = append(lines, dimStyle.Render("Season schedule complete"))
	}
	if page.Data.ByeWeek > 0 {
		lines = append(lines, dimStyle.Render(fmt.Sprintf("BYE  Week %d", page.Data.ByeWeek)))
	}
	lines = append(lines, "", sectionStyle.Render("SEASON LEADERS"))
	if len(page.Leaders) == 0 {
		message := "Loading player totals…"
		if wins+losses+ties == 0 {
			message = "Player totals appear after the first game."
		}
		if page.LeaderError != "" {
			message = page.LeaderError
		}
		lines = append(lines, dimStyle.Render(message))
	} else {
		for _, leader := range page.Leaders {
			lines = append(lines, textStyle.Render(fmt.Sprintf("%-9s %-23s %4d YDS  %d TD", strings.ToUpper(leader.Category), clipText(leader.Player, 23), leader.Yards, leader.Touchdowns)))
		}
		if page.LeaderError != "" {
			lines = append(lines, dimStyle.Render("Partial totals: "+page.LeaderError))
		}
	}
	lines = append(lines, "", sectionStyle.Render("FULL SCHEDULE  ·  ENTER OPENS GAME"))
	for i, g := range page.Data.Events {
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
	if page.Error != "" {
		lines = append(lines, liveStyle.Render("Refresh failed: "+page.Error))
	}
	return lines
}

func (m model) teamPageSize() int {
	if m.compact || m.height > 0 && m.height < 16 {
		return max(1, m.height-4)
	}
	if m.height > 0 {
		return max(1, m.height-10)
	}
	return len(m.teamLines())
}

func (m model) maxTeamScroll() int { return max(0, len(m.teamLines())-m.teamPageSize()) }

func (m model) teamSelectedLine() int {
	for i, line := range m.scheduleLines() {
		if strings.Contains(line, "FULL SCHEDULE") {
			return i + 1 + m.teamSelected
		}
	}
	return 0
}

func (m model) renderTeam(inner int) string {
	page := m.activeTeamPage()
	name := page.Data.Team.DisplayName
	if name == "" {
		name = page.Team + " SEASON"
	}
	lines := []string{accentStyle.Render(fmt.Sprintf("%s  ·  %d", strings.ToUpper(name), m.season)), m.tabLine(), borderStyle.Render(strings.Repeat("─", inner))}
	rows := m.teamLines()
	start := min(m.teamTabScroll(), m.maxTeamScroll())
	for _, row := range rows[start:min(len(rows), start+m.teamPageSize())] {
		lines = append(lines, clipStyled(row, inner))
	}
	return centeredFrame(lines, inner, m.width)
}
