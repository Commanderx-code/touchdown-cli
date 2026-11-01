package main

import (
	"context"
	"net/http"
	"testing"
)

func TestScoreboardParsesClockPeriodRedZone(t *testing.T) {
	api := jsonClient(t, func(*http.Request) {}, `{"events":[{"id":"1","date":"2026-11-01T18:00Z","status":{"type":{"state":"in","shortDetail":"2:14 - 4th"},"displayClock":"2:14","period":4},"competitions":[{"situation":{"possession":"8","down":3,"distance":7,"isRedZone":true},"competitors":[{"homeAway":"home","score":"27","team":{"id":"8","abbreviation":"GB"}},{"homeAway":"away","score":"19","team":{"id":"29","abbreviation":"CAR"}}]}]}]}`)
	board, err := api.scoreboard(context.Background(), 0, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	g := board.Events[0]
	if g.Status.DisplayClock != "2:14" || g.Status.Period != 4 || !g.Competitions[0].Situation.IsRedZone {
		t.Fatalf("fields not parsed: %+v", g.Status)
	}
}
