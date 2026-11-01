package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/charmbracelet/x/term"
)

const clearScreen = "\033[H\033[2J"

// tickerLines renders the scoreboard for a narrow pane: a header, the
// favorite team's game, then live, upcoming and finished games.
func tickerLines(board scoreboard, favorite string, now time.Time, width int, cached bool) []string {
	header := fmt.Sprintf("Week %d · updated %s", board.Week.Number, now.Format("15:04:05"))
	if cached {
		header += " · cached"
	}
	lines := []string{clipText(header, width), ""}
	fav, hasFav := favoriteGame(board, favorite)
	if hasFav {
		lines = append(lines, clipText(tickerLine(fav, now), width), "")
	}
	for _, state := range []string{"in", "pre", "post"} {
		for _, g := range board.Events {
			if hasFav && g.ID == fav.ID {
				continue
			}
			if g.Status.Type.State != state {
				continue
			}
			lines = append(lines, clipText(tickerLine(g, now), width))
		}
	}
	return lines
}

func tickerLine(g game, now time.Time) string {
	away, home := g.sides()
	if g.Status.Type.State == "pre" {
		return fmt.Sprintf("%-4s     %-4s      %s", away.Team.Abbreviation, home.Team.Abbreviation, kickoffLabel(g, now))
	}
	line := fmt.Sprintf("%-4s %2s  %-4s %2s   %s", away.Team.Abbreviation, scoreOrZero(away.Score), home.Team.Abbreviation, scoreOrZero(home.Score), g.statusText())
	if g.Status.Type.State == "in" {
		if suffix := liveSuffix(g); suffix != "" {
			line += "   " + suffix
		}
	}
	return line
}

func liveSuffix(g game) string {
	if len(g.Competitions) == 0 {
		return ""
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
		parts = append(parts, fmt.Sprintf("%s & %s", strings.ToLower(downLabel(situation.Down)), distance))
	}
	if situation.IsRedZone {
		parts = append(parts, "RZ")
	}
	return strings.Join(parts, " · ")
}

// terminalWidth reports stdout's column count, or fallback when stdout is
// not a terminal.
func terminalWidth(fallback int) int {
	if w, _, err := term.GetSize(os.Stdout.Fd()); err == nil && w > 0 {
		return w
	}
	return fallback
}

// runWatch redraws the ticker every interval until ctx is cancelled. A
// failed fetch leaves the last good screen in place.
func runWatch(ctx context.Context, api *client, favorite string, season, week, seasonType int, out io.Writer, interval time.Duration) int {
	for {
		fetchCtx, cancel := context.WithTimeout(ctx, refreshTimeout)
		board, err := api.scoreboard(fetchCtx, season, week, seasonType)
		cancel()
		if err != nil {
			if ctx.Err() == nil {
				fmt.Fprintln(os.Stderr, "fetch failed:", err)
			}
		} else {
			lines := tickerLines(board, favorite, time.Now(), terminalWidth(60), !board.CachedAt.IsZero())
			fmt.Fprint(out, clearScreen+strings.Join(lines, "\n")+"\n")
		}
		select {
		case <-ctx.Done():
			return 0
		case <-time.After(interval):
		}
	}
}

func watchContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt)
}
