package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func scheduleJSON(team string) string {
	return `{"team":{"displayName":"` + map[string]string{"GB": "Green Bay Packers", "DET": "Detroit Lions", "MIN": "Minnesota Vikings", "DAL": "Dallas Cowboys"}[team] + `","abbreviation":"` + team + `"},"byeWeek":11,"events":[{"id":"` + team + `1","date":"2026-09-13T17:00Z","week":{"number":1},"competitions":[{"status":{"type":{"state":"post"}},"competitors":[{"homeAway":"away","team":{"abbreviation":"` + team + `"},"score":{"displayValue":"24"}},{"homeAway":"home","team":{"abbreviation":"MIN"},"score":{"displayValue":"20"}}]}]},{"id":"` + team + `2","date":"2026-12-04T17:00Z","week":{"number":14},"competitions":[{"status":{"type":{"state":"pre"}},"competitors":[{"homeAway":"home","team":{"abbreviation":"` + team + `"}},{"homeAway":"away","team":{"abbreviation":"DET"}}]}]}]}`
}

func decodeSchedule(t *testing.T, team string) teamSchedule {
	t.Helper()
	var s teamSchedule
	if err := json.Unmarshal([]byte(scheduleJSON(team)), &s); err != nil {
		t.Fatal(err)
	}
	return s
}

// scheduleClient serves any team's schedule and empty summaries.
func scheduleClient(t *testing.T, seen *[]string) *client {
	t.Helper()
	return &client{http: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if seen != nil {
			*seen = append(*seen, req.URL.Path+"?"+req.URL.RawQuery)
		}
		body := `{}`
		if parts := strings.Split(req.URL.Path, "/"); len(parts) > 3 && parts[len(parts)-1] == "schedule" {
			body = scheduleJSON(strings.ToUpper(parts[len(parts)-2]))
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}}
}

func favoriteSeasonModel(t *testing.T) model {
	t.Helper()
	return model{api: scheduleClient(t, nil), season: 2026, week: 9, seasonType: 2, favorite: "GB", teamData: decodeSchedule(t, "GB"), teamSeason: 2026, teamLeadersReady: true, width: 100, height: 30}
}

func TestOpenTeamSeasonLoadsViewedTeamWithoutTouchingFavorite(t *testing.T) {
	m := favoriteSeasonModel(t)
	m, cmd := m.openTeamSeason("DET")
	if !m.showTeam || m.viewTeam != "DET" || cmd == nil {
		t.Fatalf("open: showTeam %v viewTeam %q cmd nil %v", m.showTeam, m.viewTeam, cmd == nil)
	}
	msg, ok := cmd().(teamScheduleMsg)
	if !ok || msg.team != "DET" {
		t.Fatalf("expected a DET schedule message, got %T %+v", msg, msg)
	}
	next, _ := m.Update(msg)
	m = next.(model)
	if m.view.Data.Team.Abbreviation != "DET" || m.view.Loading {
		t.Fatalf("viewed slot not filled: %+v", m.view)
	}
	if m.teamData.Team.Abbreviation != "GB" || len(m.teamData.Events) != 2 {
		t.Fatalf("favorite data was touched: %+v", m.teamData.Team)
	}
	view := m.View()
	if !strings.Contains(view, "DETROIT LIONS") || !strings.Contains(view, "DET  1-0-0") {
		t.Fatalf("season page not showing DET:\n%s", view)
	}
}

func TestStaleTeamScheduleMsgDropped(t *testing.T) {
	m := favoriteSeasonModel(t)
	m, _ = m.openTeamSeason("DAL")
	next, _ := m.Update(teamScheduleMsg{season: 2026, team: "DET", data: decodeSchedule(t, "DET")})
	m = next.(model)
	if len(m.view.Data.Events) != 0 || m.view.Team != "DAL" {
		t.Fatalf("stale message applied: %+v", m.view)
	}
}

func TestFavoriteSeasonPageUnchanged(t *testing.T) {
	m := favoriteSeasonModel(t)
	m.showTeam = true
	before := m.View()
	m.viewTeam = "GB"
	if after := m.View(); after != before {
		t.Fatalf("favorite page differs when viewTeam is the favorite:\n%s\n---\n%s", before, after)
	}
	if !strings.Contains(before, "GREEN BAY PACKERS") {
		t.Fatalf("favorite page lost its header:\n%s", before)
	}
}
