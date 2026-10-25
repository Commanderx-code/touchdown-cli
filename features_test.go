package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestScheduleAndSeasonLeaders(t *testing.T) {
	api := &client{http: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var body string
		switch req.URL.Path {
		case "/apis/site/v2/sports/football/nfl/teams/gb/schedule":
			if req.URL.Query().Get("season") != "2026" {
				t.Errorf("wrong season: %s", req.URL)
			}
			body = `{"team":{"displayName":"Green Bay Packers"},"byeWeek":11,"events":[{"id":"2","date":"2026-10-04T17:00Z","week":{"number":4},"competitions":[{"status":{"type":{"state":"pre"}},"competitors":[{"homeAway":"home","team":{"abbreviation":"GB"}},{"homeAway":"away","team":{"abbreviation":"DET"}}]}]},{"id":"1","date":"2026-09-13T17:00Z","week":{"number":1},"competitions":[{"status":{"type":{"state":"post"}},"competitors":[{"homeAway":"away","team":{"abbreviation":"GB"},"score":{"displayValue":"24"}},{"homeAway":"home","team":{"abbreviation":"MIN"},"score":{"displayValue":"20"}}]}]}]}`
		case "/apis/site/v2/sports/football/nfl/summary":
			if req.URL.Query().Get("event") != "1" {
				t.Errorf("unexpected event: %s", req.URL)
			}
			body = `{"boxscore":{"players":[{"team":{"abbreviation":"GB"},"statistics":[{"name":"passing","labels":["YDS","TD"],"athletes":[{"athlete":{"displayName":"Jordan Love"},"stats":["250","2"]}]},{"name":"rushing","labels":["YDS","TD"],"athletes":[{"athlete":{"displayName":"Josh Jacobs"},"stats":["80","1"]}]},{"name":"receiving","labels":["YDS","TD"],"athletes":[{"athlete":{"displayName":"Christian Watson"},"stats":["105","1"]}]}]}]}}`
		default:
			t.Errorf("unexpected path: %s", req.URL.Path)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}}
	schedule, err := api.teamSchedule(context.Background(), "GB", 2026)
	if err != nil {
		t.Fatal(err)
	}
	if len(schedule.Events) != 2 || schedule.Events[0].ID != "1" || schedule.ByeWeek != 11 {
		t.Fatalf("bad schedule: %+v", schedule)
	}
	opp, loc, score := schedule.Events[0].matchup("GB")
	if opp != "MIN" || loc != "@" || score != "W 24-20" {
		t.Fatalf("bad matchup: %s %s %s", opp, loc, score)
	}
	leaders, err := api.teamLeaders(context.Background(), "GB", schedule)
	if err != nil || len(leaders) != 3 || leaders[0].Player != "Jordan Love" || leaders[0].Yards != 250 {
		t.Fatalf("bad leaders: %+v %v", leaders, err)
	}
	m := model{season: 2026, favorite: "GB", showTeam: true, teamData: schedule, teamSeason: 2026, teamLeaders: leaders, width: 80, height: 24}
	view := m.View()
	if !strings.Contains(view, "BYE") || !strings.Contains(view, "Jordan Love") || !strings.Contains(view, "W 24-20") {
		t.Fatalf("team page missing data:\n%s", view)
	}
	if strings.Count(view, "\n")+1 > 24 {
		t.Fatal("team page overflowed terminal")
	}
}

func TestCachedFetchSurvivesOffline(t *testing.T) {
	offline := false
	api := &client{cacheDir: t.TempDir(), http: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		if offline {
			return nil, errors.New("offline")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"events":[{"id":"123"}],"week":{"number":1}}`))}, nil
	})}}
	first, err := api.scoreboard(context.Background(), 2026, 1, 2)
	if err != nil || !first.CachedAt.IsZero() {
		t.Fatalf("fresh fetch: %+v %v", first, err)
	}
	offline = true
	second, err := api.scoreboard(context.Background(), 2026, 1, 2)
	if err != nil || second.CachedAt.IsZero() || len(second.Events) != 1 {
		t.Fatalf("offline cache: %+v %v", second, err)
	}
}

func TestAlertsOnlyChangedGamesAndFinal(t *testing.T) {
	decode := func(s string) scoreboard {
		var b scoreboard
		if err := json.Unmarshal([]byte(s), &b); err != nil {
			t.Fatal(err)
		}
		return b
	}
	previous := decode(`{"events":[{"id":"gb","status":{"type":{"state":"in"}},"competitions":[{"competitors":[{"homeAway":"away","score":"7","team":{"abbreviation":"GB"}},{"homeAway":"home","score":"0","team":{"abbreviation":"DET"}}]}]},{"id":"other","status":{"type":{"state":"in"}},"competitions":[{"competitors":[{"homeAway":"away","score":"0","team":{"abbreviation":"KC"}},{"homeAway":"home","score":"0","team":{"abbreviation":"DEN"}}]}]}]}`)
	current := decode(`{"events":[{"id":"gb","status":{"type":{"state":"post"}},"competitions":[{"competitors":[{"homeAway":"away","score":"14","team":{"abbreviation":"GB"}},{"homeAway":"home","score":"0","team":{"abbreviation":"DET"}}]}]},{"id":"other","status":{"type":{"state":"in"}},"competitions":[{"competitors":[{"homeAway":"away","score":"3","team":{"abbreviation":"KC"}},{"homeAway":"home","score":"0","team":{"abbreviation":"DEN"}}]}]}]}`)
	alerts := scoreAlerts(previous, current, "GB", false)
	if len(alerts) != 1 || !strings.Contains(alerts[0].title, "Final") {
		t.Fatalf("favorite alerts: %+v", alerts)
	}
	if got := scoreAlerts(previous, current, "GB", true); len(got) != 2 {
		t.Fatalf("all alerts: %+v", got)
	}
	if got := scoreAlerts(current, current, "GB", true); len(got) != 0 {
		t.Fatalf("duplicate alerts: %+v", got)
	}
}

func TestGameCenterAndAppearance(t *testing.T) {
	var board scoreboard
	if err := json.Unmarshal([]byte(`{"events":[{"id":"gb","status":{"type":{"state":"post"}},"competitions":[{"competitors":[{"homeAway":"away","score":"17","team":{"abbreviation":"GB"},"linescores":[{"displayValue":"7"},{"displayValue":"10"}]},{"homeAway":"home","score":"10","team":{"abbreviation":"DET"},"linescores":[{"displayValue":"3"},{"displayValue":"7"}]}]}]}]}`), &board); err != nil {
		t.Fatal(err)
	}
	var p play
	_ = json.Unmarshal([]byte(`{"text":"Touchdown","period":{"number":2},"clock":{"displayValue":"1:23"},"team":{"abbreviation":"GB"},"awayScore":17,"homeScore":10}`), &p)
	m := model{board: board, detailID: "gb", detail: gameDetail{ScoringPlays: []play{p}, Drives: []drive{{Result: "Touchdown"}}}, showDetail: true, detailTab: 3, width: 80, height: 24}
	view := m.View()
	if !strings.Contains(view, "QUARTER SCORES") || !strings.Contains(view, "SCORING PLAYS") || !strings.Contains(view, "DRIVE TIMELINE") {
		t.Fatalf("center missing sections:\n%s", view)
	}
	if err := applyTheme("amber"); err != nil {
		t.Fatal(err)
	}
	defer applyTheme("mint")
	m.compact = true
	view = m.View()
	if !strings.Contains(view, "CENTER") || strings.Count(view, "\n")+1 > 24 {
		t.Fatalf("compact center invalid:\n%s", view)
	}
	if err := applyTheme("invalid"); err == nil {
		t.Fatal("invalid theme accepted")
	}
	home := model{showHome: true, compact: true, width: 80, height: 24, favorite: "GB"}
	if strings.Count(home.View(), "\n")+1 > 15 {
		t.Fatal("compact home is too tall")
	}
	next, _ := home.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if !next.(model).showTeam {
		t.Fatal("p did not open season page")
	}
}
