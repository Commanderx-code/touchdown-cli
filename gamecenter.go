package main

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) gameCenterLines() []string {
	g := m.currentDetailGame()
	if g == nil {
		return []string{dimStyle.Render("Game unavailable.")}
	}
	away, home := g.sides()
	lines := []string{sectionStyle.Render("QUARTER SCORES")}
	periods := max(len(away.Linescores), len(home.Linescores))
	if periods == 0 {
		lines = append(lines, dimStyle.Render("Quarter scores will appear after kickoff."))
	} else {
		header := fmt.Sprintf("%-5s", "TEAM")
		for i := 0; i < periods; i++ {
			label := fmt.Sprintf("Q%d", i+1)
			if i >= 4 {
				label = "OT"
				if i > 4 {
					label = fmt.Sprintf("OT%d", i-3)
				}
			}
			header += fmt.Sprintf(" %4s", label)
		}
		header += "  TOTAL"
		lines = append(lines, dimStyle.Render(header))
		for _, team := range []competitor{away, home} {
			row := fmt.Sprintf("%-5s", team.Team.Abbreviation)
			for i := 0; i < periods; i++ {
				value := "-"
				if i < len(team.Linescores) {
					value = team.Linescores[i].DisplayValue
				}
				row += fmt.Sprintf(" %4s", value)
			}
			row += fmt.Sprintf("  %5s", team.Score)
			lines = append(lines, textStyle.Render(row))
		}
	}
	lines = append(lines, "", sectionStyle.Render("SCORING PLAYS"))
	if len(m.detail.ScoringPlays) == 0 {
		lines = append(lines, dimStyle.Render("No scoring plays yet."))
	}
	for _, play := range m.detail.ScoringPlays {
		team := play.ScoreTeam.Abbreviation
		line := fmt.Sprintf("Q%d %-5s  %-3s  %d–%d", play.Period.Number, play.Clock.DisplayValue, team, play.AwayScore, play.HomeScore)
		lines = append(lines, accentStyle.Render(line))
		for _, text := range wrapWords(play.Text, m.centerWrapWidth()) {
			lines = append(lines, dimStyle.Render("  "+text))
		}
	}
	lines = append(lines, "", sectionStyle.Render("DRIVE TIMELINE"))
	if len(m.detail.Drives) == 0 {
		lines = append(lines, dimStyle.Render("No drives yet."))
	}
	for i, drive := range m.detail.Drives {
		label := fmt.Sprintf("%2d  Q%d %-5s  %-3s  %s", i+1, drive.Start.Period.Number, drive.Start.Clock.DisplayValue, drive.Team.Abbreviation, drive.Result)
		if drive.IsScore {
			lines = append(lines, accentStyle.Render(label))
		} else {
			lines = append(lines, sectionStyle.Render(label))
		}
		if drive.Description != "" {
			for _, row := range wrapWords(drive.Description, m.centerWrapWidth()) {
				lines = append(lines, dimStyle.Render("    "+row))
			}
		}
	}
	if injuryWindow(g.dateTime(), time.Now()) {
		lines = append(lines, "")
		lines = append(lines, m.gameInjuryLines(*g)...)
	}
	return lines
}

// injuryWindow reports whether a game is recent enough for today's injury
// report to describe it: within 14 days either side of kickoff.
func injuryWindow(kickoff, now time.Time) bool {
	if kickoff.IsZero() {
		return false
	}
	gap := now.Sub(kickoff)
	if gap < 0 {
		gap = -gap
	}
	return gap <= 14*24*time.Hour
}

const gameInjuryRows = 8

// gameInjuryLines lists each team's Out, Doubtful and Questionable players,
// away team first.
func (m model) gameInjuryLines(g game) []string {
	lines := []string{sectionStyle.Render("INJURY REPORT")}
	away, home := g.sides()
	for _, side := range []competitor{away, home} {
		team := side.Team.Abbreviation
		lines = append(lines, accentStyle.Render(plainText(team)))
		entry, ok := m.injuriesFor(team)
		switch {
		case !ok || entry.Loading && entry.Report.Team == "":
			lines = append(lines, dimStyle.Render("Loading injury report…"))
			continue
		case entry.Error != "" && entry.Report.Team == "":
			lines = append(lines, liveStyle.Render("Injuries: "+entry.Error))
			continue
		}
		rows := entry.Report.gameDay()
		if len(rows) == 0 {
			lines = append(lines, dimStyle.Render("No Out, Doubtful or Questionable players."))
			continue
		}
		for i, row := range rows {
			if i == gameInjuryRows {
				lines = append(lines, dimStyle.Render(fmt.Sprintf("+%d more", len(rows)-gameInjuryRows)))
				break
			}
			text := fmt.Sprintf("%-4s %-3s %s", statusLabel(row), row.Position, row.Name)
			if row.BodyPart != "" {
				text += " (" + row.BodyPart + ")"
			}
			lines = append(lines, textStyle.Render(clipText(strings.TrimRight(text, " "), m.centerWrapWidth())))
		}
	}
	return lines
}

// withGameInjuries batches a game-center load with both teams' injury
// reports when the game is inside the injury window.
func (m model) withGameInjuries(load tea.Cmd) (model, tea.Cmd) {
	next, cmd := m.requestGameInjuries(false)
	return next, tea.Batch(load, cmd)
}

// requestGameInjuries fetches both teams' reports for the open game center
// when the game is inside the injury window.
func (m model) requestGameInjuries(force bool) (model, tea.Cmd) {
	g := m.currentDetailGame()
	if g == nil || !injuryWindow(g.dateTime(), time.Now()) {
		return m, nil
	}
	away, home := g.sides()
	var cmds []tea.Cmd
	for _, team := range []string{away.Team.Abbreviation, home.Team.Abbreviation} {
		var cmd tea.Cmd
		m, cmd = m.requestInjuries(team, force)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if len(cmds) == 0 {
		return m, nil
	}
	return m, tea.Batch(cmds...)
}

func (m model) centerWrapWidth() int {
	width := m.width
	if width <= 0 {
		width = 80
	}
	return max(16, min(66, width-12))
}
