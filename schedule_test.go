package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
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

func dashboardModel(t *testing.T) model {
	t.Helper()
	m := favoriteSeasonModel(t)
	m.board = boardAt(t, "1", "pre", "", "")
	return m
}

func TestPickerOpensFromDashboardAndHome(t *testing.T) {
	m := dashboardModel(t)
	m.width, m.height = 80, 24
	m = key(m, "t")
	if !m.showPicker {
		t.Fatal("t on the dashboard must open the picker")
	}
	view := m.View()
	if !strings.Contains(view, "TEAM SCHEDULES") || !strings.Contains(view, "Green Bay") {
		t.Fatalf("picker view:\n%s", view)
	}
	if n := strings.Count(view, "\n") + 1; n > 24 {
		t.Fatalf("picker overflowed: %d lines", n)
	}
	home := dashboardModel(t)
	home.showHome = true
	home.homeSelection = 6
	home = key(home, "enter")
	if !home.showPicker || home.showHome {
		t.Fatal("home item 6 must open the picker")
	}
}

func TestPickerTypeToJumpAndEnter(t *testing.T) {
	m := dashboardModel(t).openPicker()
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	m, _ = m.updatePickerKey("d", now)
	m, _ = m.updatePickerKey("e", now.Add(200*time.Millisecond))
	if got := nflTeams[m.pickerIndex].abbreviation; got != "DEN" {
		t.Fatalf("after d,e want DEN got %s", got)
	}
	m, _ = m.updatePickerKey("t", now.Add(400*time.Millisecond))
	if got := nflTeams[m.pickerIndex].abbreviation; got != "DET" {
		t.Fatalf("after d,e,t want DET got %s", got)
	}
	m, _ = m.updatePickerKey("k", now.Add(3*time.Second))
	if got := nflTeams[m.pickerIndex].abbreviation; got != "KC" {
		t.Fatalf("after timeout and k want KC got %s", got)
	}
	m, cmd := m.updatePickerKey("enter", now.Add(4*time.Second))
	if !m.showTeam || m.viewTeam != "KC" || m.showPicker || cmd == nil {
		t.Fatalf("enter: showTeam %v viewTeam %q showPicker %v", m.showTeam, m.viewTeam, m.showPicker)
	}
}

func TestOpponentJumpAndEscWalksBack(t *testing.T) {
	m := favoriteSeasonModel(t)
	m.showTeam = true
	m.teamSelected = 0 // W1 @ MIN
	m = key(m, "t")
	if !m.showTeam || m.viewTeam != "MIN" {
		t.Fatalf("t on GB's page: showTeam %v viewTeam %q, want MIN", m.showTeam, m.viewTeam)
	}
	m, _ = m.applyViewSchedule(teamScheduleMsg{team: "MIN", data: decodeSchedule(t, "MIN")})
	m.teamSelected = 1 // W14 vs DET
	m = key(m, "t")
	if m.viewTeam != "DET" {
		t.Fatalf("t on MIN's page: viewTeam %q, want DET", m.viewTeam)
	}
	m = key(m, "esc")
	if !m.showTeam || m.viewTeam != "MIN" {
		t.Fatalf("first esc: showTeam %v viewTeam %q, want MIN", m.showTeam, m.viewTeam)
	}
	m = key(m, "esc")
	if !m.showTeam || !m.showingFavorite() {
		t.Fatalf("second esc: showTeam %v viewTeam %q, want favorite", m.showTeam, m.viewTeam)
	}
	m = key(m, "esc")
	if m.showTeam || m.showPicker {
		t.Fatalf("third esc must leave the season page: showTeam %v showPicker %v", m.showTeam, m.showPicker)
	}
}

func TestOpponentJumpIgnoresEmptySelection(t *testing.T) {
	m := model{favorite: "GB", season: 2026, teamSeason: 2026, showTeam: true, width: 80, height: 24}
	m = key(m, "t")
	if !m.showTeam || m.viewTeam != "" || m.showPicker {
		t.Fatalf("t with no games must be a no-op: showTeam %v viewTeam %q showPicker %v", m.showTeam, m.viewTeam, m.showPicker)
	}
}

func TestPickerEscReturnsToOrigin(t *testing.T) {
	m := dashboardModel(t)
	m = key(key(m, "t"), "esc")
	if m.showPicker || m.showHome {
		t.Fatalf("dashboard → t → esc must return to the dashboard: showPicker %v showHome %v", m.showPicker, m.showHome)
	}
	m = dashboardModel(t)
	m.showHome, m.homeSelection = true, 6
	m = key(key(m, "enter"), "esc")
	if m.showPicker || !m.showHome {
		t.Fatalf("home → picker → esc must return home: showPicker %v showHome %v", m.showPicker, m.showHome)
	}
	m = dashboardModel(t)
	m = key(key(m, "t"), "enter") // favorite preselected
	if !m.showTeam || m.showPicker {
		t.Fatalf("picker → enter must open the season page: showTeam %v showPicker %v", m.showTeam, m.showPicker)
	}
	m = key(m, "esc")
	if !m.showPicker {
		t.Fatal("esc on a season page opened from the picker must return to the picker")
	}
	m = key(m, "esc")
	if m.showPicker || m.showHome || m.showTeam {
		t.Fatal("esc on the picker must then return to the dashboard")
	}
}
