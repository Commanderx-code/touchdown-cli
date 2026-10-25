package main

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type scoreAlert struct{ title, body string }

func scoreAlerts(previous, current scoreboard, favorite string, all bool) []scoreAlert {
	byID := make(map[string]game, len(previous.Events))
	for _, g := range previous.Events {
		if g.ID != "" {
			byID[g.ID] = g
		}
	}
	var alerts []scoreAlert
	for _, g := range current.Events {
		before, ok := byID[g.ID]
		if !ok || g.ID == "" {
			continue
		}
		away, home := g.sides()
		if !all && favorite != away.Team.Abbreviation && favorite != home.Team.Abbreviation {
			continue
		}
		oldAway, oldHome := before.sides()
		if g.Status.Type.State == "pre" || before.Status.Type.State == "post" {
			continue
		}
		changed := away.Score != oldAway.Score || home.Score != oldHome.Score
		final := g.Status.Type.State == "post" && before.Status.Type.State != "post"
		if !changed && !final {
			continue
		}
		status := "Score update"
		if final {
			status = "Final"
		}
		alerts = append(alerts, scoreAlert{title: fmt.Sprintf("Touchdown · %s", status), body: fmt.Sprintf("%s %s — %s %s", away.Team.Abbreviation, away.Score, home.Score, home.Team.Abbreviation)})
	}
	return alerts
}

func sendAlert(alert scoreAlert) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return alertMsg{err: exec.CommandContext(ctx, "notify-send", "--app-name=Touchdown", alert.title, alert.body).Run()}
	}
}
