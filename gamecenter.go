package main

import (
	"fmt"
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
	return lines
}

func (m model) centerWrapWidth() int {
	width := m.width
	if width <= 0 {
		width = 80
	}
	return max(16, min(66, width-12))
}
