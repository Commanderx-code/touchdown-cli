package main

import (
	"context"
	"net/http"
	"testing"
	"time"
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

func statusBoard(state, clock string, period int, desc, short, awayScore, homeScore, possession string, rz bool) scoreboard {
	return scoreboard{Events: []game{{ID: "1", Date: "2026-11-01T18:00Z",
		Status: gameStatus{Type: statusType{State: state, ShortDetail: short, Description: desc}, DisplayClock: clock, Period: period},
		Competitions: []competition{{Situation: gameSituation{Possession: possession, IsRedZone: rz},
			Competitors: []competitor{{HomeAway: "away", Score: awayScore, Team: teamRef{ID: "29", Abbreviation: "CAR"}}, {HomeAway: "home", Score: homeScore, Team: teamRef{ID: "8", Abbreviation: "GB"}}}}}}}}
}

func TestStatusLine(t *testing.T) {
	now := time.Date(2026, 11, 1, 14, 30, 0, 0, time.FixedZone("EST", -5*3600))
	live := statusBoard
	cases := []struct {
		name     string
		board    scoreboard
		cachedAt time.Time
		want     string
	}{
		{"live ball", live("in", "2:14", 4, "In Progress", "2:14 - 4th", "19", "27", "8", false), time.Time{}, "GB 27-19 CAR · Q4 2:14 · BALL"},
		{"live red zone", live("in", "2:14", 4, "In Progress", "2:14 - 4th", "19", "27", "8", true), time.Time{}, "GB 27-19 CAR · Q4 2:14 · BALL · RZ"},
		{"opponent ball no rz", live("in", "2:14", 4, "In Progress", "2:14 - 4th", "19", "27", "29", true), time.Time{}, "GB 27-19 CAR · Q4 2:14"},
		{"overtime", live("in", "8:00", 5, "In Progress", "8:00 - OT", "27", "27", "", false), time.Time{}, "GB 27-27 CAR · OT 8:00"},
		{"halftime", live("in", "0:00", 2, "Halftime", "Halftime", "10", "17", "", false), time.Time{}, "GB 17-10 CAR · Halftime"},
		{"end of quarter", live("in", "0:00", 1, "End of 1st Quarter", "End of 1st", "0", "7", "", false), time.Time{}, "GB 7-0 CAR · End of 1st"},
		{"missing score", live("in", "14:55", 1, "In Progress", "14:55 - 1st", "", "7", "", false), time.Time{}, "GB 7-0 CAR · Q1 14:55"},
		{"final win", live("post", "0:00", 4, "Final", "Final", "19", "27", "", false), time.Time{}, "GB W 27-19 CAR · Final"},
		{"final loss ot", live("post", "0:00", 5, "Final", "Final/OT", "30", "27", "", false), time.Time{}, "GB L 27-30 CAR · Final/OT"},
		{"tie", live("post", "0:00", 5, "Final", "Final/OT", "27", "27", "", false), time.Time{}, "GB T 27-27 CAR · Final/OT"},
		{"stale cache", live("in", "2:14", 4, "In Progress", "2:14 - 4th", "19", "27", "", false), now.Add(-6 * time.Minute), "GB 27-19 CAR · Q4 2:14 (cached)"},
		{"fresh cache", live("in", "2:14", 4, "In Progress", "2:14 - 4th", "19", "27", "", false), now.Add(-20 * time.Second), "GB 27-19 CAR · Q4 2:14"},
	}
	for _, tc := range cases {
		if got := statusLine(tc.board, "GB", now, tc.cachedAt); got != tc.want {
			t.Errorf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
}

func TestStatusLineScheduled(t *testing.T) {
	now := time.Date(2026, 11, 1, 9, 0, 0, 0, time.FixedZone("EST", -5*3600))
	pre := func(date, desc, short string, favHome bool) scoreboard {
		fav, opp := "home", "away"
		if !favHome {
			fav, opp = "away", "home"
		}
		return scoreboard{Events: []game{{ID: "1", Date: date,
			Status:       gameStatus{Type: statusType{State: "pre", ShortDetail: short, Description: desc}},
			Competitions: []competition{{Competitors: []competitor{{HomeAway: fav, Team: teamRef{ID: "8", Abbreviation: "GB"}}, {HomeAway: opp, Team: teamRef{ID: "9", Abbreviation: "DET"}}}}}}}}
	}
	cases := []struct {
		name  string
		board scoreboard
		want  string
	}{
		{"today home", pre("2026-11-01T18:00Z", "Scheduled", "11/1 - 1:00 PM EST", true), "GB vs DET · 1:00 PM"},
		{"other day away", pre("2026-11-08T18:00Z", "Scheduled", "11/8 - 1:00 PM EST", false), "GB @ DET · Sun 1:00 PM"},
		{"postponed", pre("2026-11-01T18:00Z", "Postponed", "Postponed", true), "GB vs DET · Postponed"},
		{"canceled", pre("2026-11-01T18:00Z", "Canceled", "Canceled", false), "GB @ DET · Canceled"},
	}
	for _, tc := range cases {
		if got := statusLine(tc.board, "GB", now, time.Time{}); got != tc.want {
			t.Errorf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
}

func TestStatusLineNoGame(t *testing.T) {
	now := time.Date(2026, 11, 1, 14, 30, 0, 0, time.UTC)
	bye := scoreboard{Events: []game{{ID: "2", Date: "2026-11-01T18:00Z",
		Status:       gameStatus{Type: statusType{State: "in", ShortDetail: "8:01 - 3rd"}, DisplayClock: "8:01", Period: 3},
		Competitions: []competition{{Competitors: []competitor{{HomeAway: "away", Score: "14", Team: teamRef{ID: "33", Abbreviation: "BAL"}}, {HomeAway: "home", Score: "17", Team: teamRef{ID: "2", Abbreviation: "BUF"}}}}}}}}
	if got := statusLine(bye, "GB", now, time.Time{}); got != "" {
		t.Fatalf("bye should be empty, got %q", got)
	}
	dup := statusBoard("in", "2:14", 4, "In Progress", "2:14 - 4th", "19", "27", "", false)
	second := statusBoard("post", "0:00", 4, "Final", "Final", "3", "6", "", false).Events[0]
	second.ID = "9"
	dup.Events = append(dup.Events, second)
	if got := statusLine(dup, "GB", now, time.Time{}); got != "GB 27-19 CAR · Q4 2:14" {
		t.Fatalf("duplicate favorite should use the first game, got %q", got)
	}
	if _, ok := favoriteGame(scoreboard{}, "GB"); ok {
		t.Fatal("empty board must not find a game")
	}
}
