package main

import (
	"fmt"
	"strings"
)

// situationText describes who has the ball and where for a game in
// progress, for example "GB ball  3rd & 7 at CAR 22  ·  RED ZONE". It is
// empty when the game is not live or ESPN reports no possession.
func situationText(g game) string {
	if g.Status.Type.State != "in" || len(g.Competitions) == 0 {
		return ""
	}
	situation := g.Competitions[0].Situation
	holder := ""
	for _, side := range g.Competitions[0].Competitors {
		if situation.Possession != "" && side.Team.ID == situation.Possession {
			holder = side.Team.Abbreviation
			break
		}
	}
	if holder == "" {
		return ""
	}
	text := holder + " ball"
	switch {
	case situation.DownDistanceText != "":
		text += "  " + situation.DownDistanceText
	case situation.Down > 0:
		distance := fmt.Sprintf("%d", situation.Distance)
		if situation.Distance == 0 {
			distance = "GOAL"
		}
		text += fmt.Sprintf("  %s & %s", downLabel(situation.Down), distance)
	}
	if situation.IsRedZone {
		text += "  ·  RED ZONE"
	}
	return plainText(text)
}

// timeoutsText shows timeouts remaining for both teams, away first, as
// "CAR TO ●●○   GB TO ●●●". It is empty unless ESPN sends both counts.
func timeoutsText(g game) string {
	if len(g.Competitions) == 0 {
		return ""
	}
	situation := g.Competitions[0].Situation
	if situation.AwayTimeouts == nil || situation.HomeTimeouts == nil {
		return ""
	}
	away, home := g.sides()
	return plainText(fmt.Sprintf("%s TO %s   %s TO %s", away.Team.Abbreviation, timeoutGlyphs(*situation.AwayTimeouts), home.Team.Abbreviation, timeoutGlyphs(*situation.HomeTimeouts)))
}

func timeoutGlyphs(remaining int) string {
	remaining = max(0, min(3, remaining))
	return strings.Repeat("●", remaining) + strings.Repeat("○", 3-remaining)
}

// latestPlayText is ESPN's last-play description with whitespace collapsed.
func latestPlayText(g game) string {
	if len(g.Competitions) == 0 {
		return ""
	}
	return plainText(strings.Join(strings.Fields(g.Competitions[0].Situation.LastPlay.Text), " "))
}

func (m model) liveContextLines() []string {
	g := m.currentDetailGame()
	if g == nil || g.Status.Type.State != "in" {
		return nil
	}
	var lines []string
	for _, line := range []string{situationText(*g), timeoutsText(*g)} {
		if line != "" {
			lines = append(lines, line)
		}
	}
	if latest := latestPlayText(*g); latest != "" {
		lines = append(lines, "LATEST  "+latest)
	}
	return lines
}

func downLabel(down int) string {
	switch down {
	case 1:
		return "1st"
	case 2:
		return "2nd"
	case 3:
		return "3rd"
	case 4:
		return "4th"
	}
	return fmt.Sprint(down)
}
