package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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

func slotBoard(t *testing.T) scoreboard {
	t.Helper()
	game := func(id, date, away, home string) string {
		return `{"id":"` + id + `","date":"` + date + `","status":{"type":{"state":"pre"}},"competitions":[{"competitors":[{"homeAway":"away","team":{"abbreviation":"` + away + `","displayName":"` + away + `"}},{"homeAway":"home","team":{"abbreviation":"` + home + `","displayName":"` + home + `"}}]}]}`
	}
	raw := `{"week":{"number":9},"events":[` + strings.Join([]string{
		game("1", "2026-11-06T01:15Z", "LV", "DEN"),
		game("2", "2026-11-08T18:00Z", "NO", "CAR"),
		game("3", "2026-11-08T18:00Z", "NYG", "CHI"),
		game("4", "2026-11-08T18:01Z", "JAX", "HOU"),
		game("5", "2026-11-10T01:15Z", "PHI", "GB"),
		game("6", "", "ARI", "SEA"),
	}, ",") + `]}`
	var b scoreboard
	if err := json.Unmarshal([]byte(raw), &b); err != nil {
		t.Fatal(err)
	}
	return b
}

func headerLines(view, slot string) int {
	n := 0
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, slot) && !strings.Contains(line, "UPCOMING") && !strings.Contains(line, "EST") {
			n++
		}
	}
	return n
}

func TestDashboardGroupsBySlot(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip(err)
	}
	saved := time.Local
	time.Local = loc
	defer func() { time.Local = saved }()
	m := model{board: slotBoard(t), season: 2026, week: 9, seasonType: 2, favorite: "GB", width: 100, height: 36}
	view := m.View()
	last := -1
	for _, slot := range []string{"THU 8:15 PM", "SUN 1:00 PM", "SUN 1:01 PM", "MON 8:15 PM", "TBD"} {
		if headerLines(view, slot) != 1 {
			t.Fatalf("want exactly one %q header, got %d\n%s", slot, headerLines(view, slot), view)
		}
		at := -1
		for i, line := range strings.Split(view, "\n") {
			if strings.Contains(line, slot) && !strings.Contains(line, "UPCOMING") && !strings.Contains(line, "EST") {
				at = i
			}
		}
		if at < last {
			t.Fatalf("%q header out of order (line %d after %d)", slot, at, last)
		}
		last = at
	}
	for _, size := range []struct{ width, height int }{{100, 30}, {80, 24}} {
		m.width, m.height = size.width, size.height
		if lines := strings.Count(m.View(), "\n") + 1; lines > size.height {
			t.Errorf("%dx%d dashboard with slot headers has %d lines", size.width, size.height, lines)
		}
	}
	m.width, m.height = 100, 23
	if headerLines(m.View(), "SUN 1:00 PM") != 0 {
		t.Fatal("no slot headers below 24 rows")
	}
	m.height, m.favoritesOnly = 30, true
	if headerLines(m.View(), "MON 8:15 PM") != 0 {
		t.Fatal("no slot headers in the favorite filter")
	}
}

// weekBoard builds a week with GB's game first (per the given state and
// scores) and every other team not in byes paired off on Sunday.
func weekBoard(t *testing.T, week int, favoriteState, favoriteScore, opponentScore string, favoriteHome bool, byes ...string) scoreboard {
	t.Helper()
	skip := map[string]bool{}
	for _, team := range byes {
		skip[team] = true
	}
	var free []string
	for _, team := range nflTeams {
		if !skip[team.abbreviation] && team.abbreviation != "GB" && team.abbreviation != "DET" {
			free = append(free, team.abbreviation)
		}
	}
	game := func(id, date, state, away, awayScore, home, homeScore string) string {
		return `{"id":"` + id + `","date":"` + date + `","status":{"type":{"state":"` + state + `"}},"competitions":[{"competitors":[{"homeAway":"away","score":"` + awayScore + `","team":{"abbreviation":"` + away + `"}},{"homeAway":"home","score":"` + homeScore + `","team":{"abbreviation":"` + home + `"}}]}]}`
	}
	var games []string
	if !skip["GB"] {
		if favoriteHome {
			games = append(games, game("fav", "2026-11-26T17:30Z", favoriteState, "DET", opponentScore, "GB", favoriteScore))
		} else {
			games = append(games, game("fav", "2026-11-26T17:30Z", favoriteState, "GB", favoriteScore, "DET", opponentScore))
		}
	} else if !skip["DET"] {
		free = append(free, "DET")
	}
	for i := 0; i+1 < len(free); i += 2 {
		games = append(games, game(fmt.Sprint(i), "2026-11-29T18:00Z", "pre", free[i], "", free[i+1], ""))
	}
	var b scoreboard
	if err := json.Unmarshal([]byte(`{"week":{"number":`+fmt.Sprint(week)+`},"events":[`+strings.Join(games, ",")+`]}`), &b); err != nil {
		t.Fatal(err)
	}
	return b
}

func TestSummarizeWeekLineFormats(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip(err)
	}
	saved := time.Local
	time.Local = loc
	defer func() { time.Local = saved }()
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, loc)
	line := summarizeWeek(weekBoard(t, 12, "pre", "", "", false, "KC", "NE"), "GB", 12, now).line("GB")
	if line != "W12  15 GAMES  ·  GB @ DET  Thu 12:30 PM  ·  BYE: KC, NE" {
		t.Fatalf("pre line: %q", line)
	}
	line = summarizeWeek(weekBoard(t, 11, "pre", "", "", false, "GB", "LV", "KC", "NE", "ARI", "ATL"), "GB", 11, now).line("GB")
	if !strings.HasPrefix(line, "W11  GB BYE  ·  13 GAMES") || !strings.Contains(line, "BYE: ARI, ATL, GB, KC, LV, NE") {
		t.Fatalf("bye line: %q", line)
	}
	line = summarizeWeek(weekBoard(t, 10, "post", "27", "19", true), "GB", 10, now).line("GB")
	if !strings.Contains(line, "GB W 27-19 DET") || !strings.Contains(line, "16 GAMES") {
		t.Fatalf("final line: %q", line)
	}
	line = summarizeWeek(weekBoard(t, 9, "pre", "", "", false, "ARI", "ATL", "BAL", "BUF", "CAR", "CHI", "CIN", "CLE"), "GB", 9, now).line("GB")
	if !strings.HasSuffix(line, "BYE: ARI, ATL, BAL, BUF, CAR, CHI +2") || !strings.Contains(line, "12 GAMES") {
		t.Fatalf("many byes: %q", line)
	}
	if line := (overviewWeek{Week: 15}).line("GB"); line != "W15  …" {
		t.Fatalf("unloaded: %q", line)
	}
	if line := (overviewWeek{Week: 15, Err: "boom"}).line("GB"); line != "W15  unavailable · r to retry" {
		t.Fatalf("error: %q", line)
	}
}

func scoreboardWeeks(seen []string) []string {
	var weeks []string
	for _, u := range seen {
		if strings.Contains(u, "/scoreboard") {
			q, _ := url.ParseQuery(strings.SplitN(u, "?", 2)[1])
			weeks = append(weeks, q.Get("week")+"/"+q.Get("seasontype"))
		}
	}
	return weeks
}

func TestOverviewOpenFetchesAtMostFiveWeeks(t *testing.T) {
	var seen []string
	m := model{api: scheduleClient(t, &seen), season: 2026, week: 12, seasonType: 2, favorite: "GB", showHome: true, homeSelection: 7, width: 100, height: 30}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(model)
	if !m.showOverview || m.overviewCursor != 12 {
		t.Fatalf("enter on SEASON: showOverview %v cursor %d", m.showOverview, m.overviewCursor)
	}
	runCmds(cmd)
	weeks := scoreboardWeeks(seen)
	sort.Strings(weeks)
	if strings.Join(weeks, " ") != "10/2 11/2 12/2 13/2 14/2" {
		t.Fatalf("open requests: %v", weeks)
	}
	seen = nil
	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(model)
	runCmds(cmd)
	if len(scoreboardWeeks(seen)) != 0 {
		t.Fatalf("cursor on 13 must not refetch in-flight weeks: %v", scoreboardWeeks(seen))
	}
	next, cmd = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(model)
	runCmds(cmd)
	if weeks := scoreboardWeeks(seen); strings.Join(weeks, " ") != "15/2" || m.overviewCursor != 14 {
		t.Fatalf("cursor on 14: requests %v cursor %d", weeks, m.overviewCursor)
	}
}

func TestOverviewEnterJumpsWeekAndEscReturns(t *testing.T) {
	var seen []string
	m := model{api: scheduleClient(t, &seen), season: 2026, week: 1, seasonType: 3, favorite: "GB", showHome: true, homeSelection: 7, width: 100, height: 30}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(model)
	m.overviewCursor = 12
	seen = nil
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(model)
	if m.showOverview || m.week != 12 || m.seasonType != 2 || m.showHome {
		t.Fatalf("enter: showOverview %v week %d type %d home %v", m.showOverview, m.week, m.seasonType, m.showHome)
	}
	msgs := runCmds(cmd)
	if _, ok := msgs[0].(boardMsg); !ok || strings.Join(scoreboardWeeks(seen), " ") != "12/2" {
		t.Fatalf("enter must load week 12 for the dashboard: %T %v", msgs[0], scoreboardWeeks(seen))
	}
	m = key(m, "esc")
	if !m.showOverview {
		t.Fatal("esc on the dashboard must return to the overview")
	}
	m = key(m, "esc")
	if m.showOverview || !m.showHome {
		t.Fatal("esc on the overview must return home")
	}
	// Opening another screen from the dashboard consumes the overview entry.
	m.homeSelection = 7
	m = key(key(m, "enter"), "enter")
	m = key(m, "t")
	if !m.showPicker {
		t.Fatal("t on the dashboard must open the picker")
	}
	m = key(m, "esc")
	if m.showOverview || m.showPicker || m.showHome {
		t.Fatal("esc on the picker must return to the dashboard, not the overview")
	}
	m = key(m, "esc")
	if !m.showHome {
		t.Fatal("esc on the dashboard must then go home")
	}
}

func TestOverviewBeforeSeasonKnown(t *testing.T) {
	var seen []string
	m := model{api: scheduleClient(t, &seen), favorite: "GB", showHome: true, homeSelection: 7, width: 80, height: 24}
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(model)
	if !m.showOverview || cmd != nil || !strings.Contains(m.View(), "Loading season…") {
		t.Fatalf("overview before the season is known: showOverview %v cmd nil %v", m.showOverview, cmd == nil)
	}
	m = key(m, "esc")
	if m.showOverview || !m.showHome {
		t.Fatal("esc must return home")
	}
}

func TestOverviewMsgNeverTouchesBoard(t *testing.T) {
	m := model{season: 2026, week: 9, seasonType: 2, favorite: "GB", overviewSeason: 2026, overview: map[int]overviewWeek{}, board: boardAt(t, "1", "in", "19", "20")}
	next, _ := m.Update(overviewMsg{season: 2026, week: 9, board: boardAt(t, "1", "in", "19", "27")})
	m = next.(model)
	if _, home := m.board.Events[0].sides(); home.Score != "20" || len(m.feed) != 0 {
		t.Fatalf("overview data leaked into the board: score %q feed %d", home.Score, len(m.feed))
	}
	if w := m.overview[9]; !w.Loaded || w.Games != 1 {
		t.Fatalf("week 9 summary missing: %+v", w)
	}
}
