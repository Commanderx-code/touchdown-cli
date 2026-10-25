package main

import (
	"fmt"
	"strings"
)

func (m model) liveContextLines() []string {
	g := m.currentDetailGame()
	if g == nil || g.Status.Type.State != "in" || len(g.Competitions) == 0 {
		return nil
	}
	situation := g.Competitions[0].Situation
	var parts []string
	for _, side := range g.Competitions[0].Competitors {
		if situation.Possession != "" && side.Team.ID == situation.Possession {
			parts = append(parts, "BALL "+side.Team.Abbreviation)
			break
		}
	}
	if situation.Down > 0 {
		distance := fmt.Sprintf("%d", situation.Distance)
		if situation.Distance == 0 {
			distance = "GOAL"
		}
		parts = append(parts, fmt.Sprintf("%s & %s", downLabel(situation.Down), distance))
	}
	var lines []string
	if len(parts) > 0 {
		lines = append(lines, strings.Join(parts, "  ·  "))
	}
	if latest := strings.Join(strings.Fields(situation.LastPlay.Text), " "); latest != "" {
		lines = append(lines, "LATEST  "+latest)
	}
	return lines
}

func downLabel(down int) string {
	switch down {
	case 1:
		return "1ST"
	case 2:
		return "2ND"
	case 3:
		return "3RD"
	case 4:
		return "4TH"
	}
	return fmt.Sprint(down)
}
