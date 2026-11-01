package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// favoriteGame returns the first game on the board involving the favorite
// team. The board is sorted by kickoff, so a duplicate listing resolves to
// the earlier game.
func favoriteGame(board scoreboard, favorite string) (game, bool) {
	for _, g := range board.Events {
		away, home := g.sides()
		if away.Team.Abbreviation == favorite || home.Team.Abbreviation == favorite {
			return g, true
		}
	}
	return game{}, false
}

// statusLine renders the favorite team's game as one plain-text line for a
// shell prompt. It returns "" when the team has no game on the board.
// cachedAt is when the board was fetched; zero means unknown, and a board
// older than five minutes is marked "(cached)".
func statusLine(board scoreboard, favorite string, now time.Time, cachedAt time.Time) string {
	g, ok := favoriteGame(board, favorite)
	if !ok {
		return ""
	}
	away, home := g.sides()
	own, other, location := away, home, "@"
	if home.Team.Abbreviation == favorite {
		own, other, location = home, away, "vs"
	}
	pair := own.Team.Abbreviation + " " + location + " " + other.Team.Abbreviation
	score := fmt.Sprintf("%s %s-%s %s", own.Team.Abbreviation, scoreOrZero(own.Score), scoreOrZero(other.Score), other.Team.Abbreviation)
	var line string
	switch {
	case notPlayed(g):
		line = pair + " · " + g.Status.Type.ShortDetail
	case g.Status.Type.State == "pre":
		line = pair + " · " + kickoffLabel(g, now)
	case g.Status.Type.State == "post":
		final := "Final"
		if g.Status.Period > 4 {
			final = "Final/OT"
		}
		line = fmt.Sprintf("%s %s %s-%s %s · %s", own.Team.Abbreviation, resultLetter(own.Score, other.Score), scoreOrZero(own.Score), scoreOrZero(other.Score), other.Team.Abbreviation, final)
	default:
		line = score + " · " + liveLabel(g, own.Team.ID)
	}
	if !cachedAt.IsZero() && now.Sub(cachedAt) > 5*time.Minute {
		line += " (cached)"
	}
	return plainText(line)
}

// statusSummary is the status line for the favorite and every followed team,
// favorite first, joined with " | ". A game two of them share appears once.
// liveOnly keeps only games in progress. "(cached)" goes at the very end.
func statusSummary(board scoreboard, favorite string, followed []string, now, cachedAt time.Time, liveOnly bool) string {
	const cached = " (cached)"
	var parts []string
	seen := map[string]bool{}
	marked := false
	for _, team := range append([]string{favorite}, followed...) {
		g, ok := favoriteGame(board, team)
		if !ok || seen[g.ID] || liveOnly && g.Status.Type.State != "in" {
			continue
		}
		seen[g.ID] = true
		line := statusLine(board, team, now, cachedAt)
		if strings.HasSuffix(line, cached) {
			marked = true
			line = strings.TrimSuffix(line, cached)
		}
		parts = append(parts, line)
	}
	summary := strings.Join(parts, " | ")
	if marked {
		summary += cached
	}
	return summary
}

// plainText drops control runes from text that came from the network, so
// nothing ESPN sends can inject escape sequences into a prompt or pane.
func plainText(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}

// notPlayed reports a postponed or canceled game, which ESPN may list under
// any state.
func notPlayed(g game) bool {
	desc := g.Status.Type.Description
	return desc == "Postponed" || desc == "Canceled"
}

func kickoffLabel(g game, now time.Time) string {
	if notPlayed(g) {
		return g.Status.Type.ShortDetail
	}
	kickoff := g.dateTime()
	if kickoff.IsZero() {
		return "Scheduled"
	}
	kickoff = kickoff.In(now.Location())
	y1, m1, d1 := kickoff.Date()
	y2, m2, d2 := now.Date()
	if y1 == y2 && m1 == m2 && d1 == d2 {
		return kickoff.Format("3:04 PM")
	}
	return kickoff.Format("Mon 3:04 PM")
}

func liveLabel(g game, favoriteID string) string {
	desc := g.Status.Type.Description
	if g.Status.DisplayClock == "" || strings.Contains(desc, "Halftime") || strings.Contains(desc, "End of") {
		return g.Status.Type.ShortDetail
	}
	label := periodLabel(g.Status.Period) + " " + g.Status.DisplayClock
	if len(g.Competitions) > 0 {
		situation := g.Competitions[0].Situation
		if situation.Possession != "" && situation.Possession == favoriteID {
			label += " · BALL"
			if situation.IsRedZone {
				label += " · RZ"
			}
		}
	}
	return label
}

func resultLetter(own, other string) string {
	a, _ := strconv.Atoi(scoreOrZero(own))
	b, _ := strconv.Atoi(scoreOrZero(other))
	switch {
	case a > b:
		return "W"
	case a < b:
		return "L"
	}
	return "T"
}

// periodLabel names a quarter: Q1-Q4, OT for anything later, "" for none.
func periodLabel(period int) string {
	switch {
	case period <= 0:
		return ""
	case period <= 4:
		return fmt.Sprintf("Q%d", period)
	}
	return "OT"
}

// scoreOrZero treats a missing score, which ESPN sends briefly at kickoff, as 0.
func scoreOrZero(s string) string {
	if s == "" {
		return "0"
	}
	return s
}

// validateStatusFlags rejects flag combinations that have no meaning in
// status mode or outside it.
func validateStatusFlags(status, watch, split, refresh, once, notify, compact bool) error {
	if !status && (watch || split || refresh) {
		return errors.New("-watch, -split and -refresh require -status")
	}
	if status && (once || compact) {
		return errors.New("-status cannot be combined with -once or -compact")
	}
	if status && notify && !watch && !split {
		return errors.New("-status needs -watch or -split with -notify or -notify-all")
	}
	return nil
}
