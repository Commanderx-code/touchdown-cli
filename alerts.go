package main

import (
	"context"
	"fmt"
	"os/exec"
	"slices"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type scoreAlert struct{ title, body string }

// scoreAlerts turns scoring events into desktop alerts for the favorite
// and followed teams' games, or for every game when all is set.
func scoreAlerts(previous, current scoreboard, favorite string, all bool, followed ...string) []scoreAlert {
	var alerts []scoreAlert
	for _, e := range scoringEvents(previous, current, favorite) {
		if !all && !e.Favorite && !slices.Contains(followed, e.Away) && !slices.Contains(followed, e.Home) {
			continue
		}
		status := "Score update"
		if e.Final {
			status = "Final"
		}
		alerts = append(alerts, scoreAlert{title: fmt.Sprintf("Touchdown · %s", status), body: fmt.Sprintf("%s %d — %d %s", e.Away, e.AwayScore, e.HomeScore, e.Home)})
	}
	return alerts
}

func sendAlert(alert scoreAlert) tea.Cmd {
	return func() tea.Msg { return alertMsg{err: deliverAlert(alert)} }
}

func deliverAlert(alert scoreAlert) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "notify-send", "--app-name=Touchdown", alert.title, alert.body).Run()
}
