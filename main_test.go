package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func jsonClient(t *testing.T, check func(*http.Request), body string) *client {
	t.Helper()
	return &client{http: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		check(req)
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}}
}

func TestScoreboardTimestampFormats(t *testing.T) {
	for _, raw := range []string{"2026-10-25T13:30Z", "2026-10-25T13:30:00Z"} {
		var g game
		if err := json.Unmarshal([]byte(`{"date":"`+raw+`","status":{"type":{"state":"pre"}}}`), &g); err != nil {
			t.Fatal(err)
		}
		if got := g.dateTime(); got.IsZero() || got.Hour() != 13 || got.Minute() != 30 {
			t.Fatalf("unexpected parsed kickoff for %s: %s", raw, got)
		}
		if g.statusText() == "Scheduled" {
			t.Fatalf("kickoff time lost for %s", raw)
		}
	}
}

func TestDetailFitsTerminalAndScrolls(t *testing.T) {
	var board scoreboard
	if err := json.Unmarshal([]byte(`{"events":[{"id":"123","date":"2026-10-25T17:00Z","status":{"type":{"state":"post","shortDetail":"Final"}},"competitions":[{"competitors":[{"homeAway":"away","score":"17","team":{"displayName":"Away","abbreviation":"AWY"}},{"homeAway":"home","score":"24","team":{"displayName":"Home","abbreviation":"HME"}}]}]}]}`), &board); err != nil {
		t.Fatal(err)
	}
	var detail boxscore
	if err := json.Unmarshal([]byte(`{"teams":[{"team":{"abbreviation":"AWY"},"statistics":[{"name":"totalYards","displayValue":"300"}]},{"team":{"abbreviation":"HME"},"statistics":[{"name":"totalYards","displayValue":"350"}]}],"players":[{"team":{"abbreviation":"AWY"},"statistics":[{"name":"passing","labels":["C/ATT","YDS","TD"],"athletes":[{"athlete":{"displayName":"A Quarterback"},"stats":["20/30","250","2"]}]}]}]}`), &detail); err != nil {
		t.Fatal(err)
	}
	m := model{board: board, detail: gameDetail{Boxscore: detail}, detailID: "123", showDetail: true, detailTab: 1, width: 80, height: 16}
	first := m.View()
	if lines := strings.Count(first, "\n") + 1; lines > m.height {
		t.Fatalf("detail overflowed terminal: %d lines", lines)
	}
	if !strings.Contains(first, "A Quarterback") {
		t.Fatal("player stats missing from first page")
	}
	m.detailScroll = 100
	last := m.View()
	if !strings.Contains(last, "RECEIVING") {
		t.Fatal("last stats category inaccessible after scrolling")
	}
}

func TestPregameStatsShowPlaceholder(t *testing.T) {
	var board scoreboard
	if err := json.Unmarshal([]byte(`{"events":[{"id":"gb","competitions":[{"competitors":[{"homeAway":"away","team":{"abbreviation":"GB"}},{"homeAway":"home","team":{"abbreviation":"DET"}}]}]}]}`), &board); err != nil {
		t.Fatal(err)
	}
	var box boxscore
	if err := json.Unmarshal([]byte(`{"teams":[{"team":{"abbreviation":"GB"}},{"team":{"abbreviation":"DET"}}]}`), &box); err != nil {
		t.Fatal(err)
	}
	m := model{board: board, detail: gameDetail{Boxscore: box}, detailID: "gb"}
	if !strings.Contains(strings.Join(m.statLines(), " "), "after kickoff") {
		t.Fatal("empty team stats need a pregame message")
	}
	m.detailTab = 1
	if !strings.Contains(strings.Join(m.statLines(), " "), "after kickoff") {
		t.Fatal("empty player stats need a pregame message")
	}
}

func TestFavoriteFilterSelectsPackersGame(t *testing.T) {
	var board scoreboard
	if err := json.Unmarshal([]byte(`{"events":[{"id":"1","competitions":[{"competitors":[{"homeAway":"away","team":{"abbreviation":"GB"}},{"homeAway":"home","team":{"abbreviation":"DET"}}]}]},{"id":"2","competitions":[{"competitors":[{"homeAway":"away","team":{"abbreviation":"KC"}},{"homeAway":"home","team":{"abbreviation":"DEN"}}]}]}]}`), &board); err != nil {
		t.Fatal(err)
	}
	m := model{board: board, favorite: "GB"}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	m = next.(model)
	if !m.favoritesOnly || len(m.visibleGames()) != 1 || m.visibleGames()[0].ID != "1" {
		t.Fatal("favorite filter did not isolate Green Bay game")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(model)
	if !m.showDetail || m.detailID != "1" {
		t.Fatal("Enter did not open filtered game")
	}
	unfiltered := model{board: board, favorite: "GB"}
	next, _ = unfiltered.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	unfiltered = next.(model)
	if !unfiltered.showDetail || unfiltered.detailID != "1" {
		t.Fatal("g did not open Packers game")
	}
}

func TestDashboardFitsTerminalSizes(t *testing.T) {
	var board scoreboard
	if err := json.Unmarshal([]byte(`{"events":[{"id":"gb","status":{"type":{"state":"pre"}},"competitions":[{"competitors":[{"homeAway":"away","team":{"abbreviation":"GB","name":"Packers"}},{"homeAway":"home","team":{"abbreviation":"DET","name":"Lions"}}]}]}]}`), &board); err != nil {
		t.Fatal(err)
	}
	for len(board.Events) < 14 {
		board.Events = append(board.Events, board.Events[0])
	}
	var schedule teamSchedule
	if err := json.Unmarshal([]byte(`{"events":[{"id":"prior","date":"2026-10-18T17:00Z","week":{"number":6},"competitions":[{"status":{"type":{"state":"post"}},"competitors":[{"homeAway":"home","score":{"displayValue":"23"},"team":{"abbreviation":"GB"}},{"homeAway":"away","score":{"displayValue":"13"},"team":{"abbreviation":"DAL"}}]}]},{"id":"gb","date":"2026-10-25T17:00Z","week":{"number":7},"competitions":[{"status":{"type":{"state":"pre"}},"competitors":[{"homeAway":"away","team":{"abbreviation":"GB"}},{"homeAway":"home","team":{"abbreviation":"DET"}}]}]}]}`), &schedule); err != nil {
		t.Fatal(err)
	}
	for _, size := range []struct{ width, height int }{{80, 24}, {150, 36}} {
		m := model{board: board, season: 2026, week: 7, seasonType: 2, favorite: "GB", teamSeason: 2026, teamData: schedule, width: size.width, height: size.height}
		view := m.View()
		lines := strings.Split(view, "\n")
		if len(lines) > size.height {
			t.Errorf("%dx%d dashboard has %d lines", size.width, size.height, len(lines))
		}
		for i, line := range lines {
			if lipgloss.Width(line) > size.width {
				t.Errorf("%dx%d line %d is %d columns", size.width, size.height, i, lipgloss.Width(line))
			}
		}
		if !strings.Contains(view, "TOUCHDOWN") || !strings.Contains(view, "PACKERS WATCH") {
			t.Errorf("%dx%d dashboard branding or favorite card missing", size.width, size.height)
		}
		if !strings.Contains(view, "NEXT") || !strings.Contains(view, "LAST") {
			t.Errorf("%dx%d dashboard next matchup or latest result missing", size.width, size.height)
		}
		if size.width >= 112 && !strings.Contains(view, "NFC SNAPSHOT") {
			t.Error("wide standings snapshot missing")
		}
	}
}

func TestFavoriteSpotlightDuringBye(t *testing.T) {
	var schedule teamSchedule
	if err := json.Unmarshal([]byte(`{"byeWeek":11,"events":[{"id":"last","date":"2026-11-08T18:00Z","week":{"number":10},"competitions":[{"status":{"type":{"state":"post"}},"competitors":[{"homeAway":"home","score":{"displayValue":"24"},"team":{"abbreviation":"GB"}},{"homeAway":"away","score":{"displayValue":"17"},"team":{"abbreviation":"MIN"}}]}]},{"id":"next","date":"2026-11-22T18:00Z","week":{"number":12},"competitions":[{"status":{"type":{"state":"pre"}},"competitors":[{"homeAway":"away","team":{"abbreviation":"GB"}},{"homeAway":"home","team":{"abbreviation":"LAR"}}]}]}]}`), &schedule); err != nil {
		t.Fatal(err)
	}
	for _, inner := range []int{32, 74} {
		m := model{favorite: "GB", season: 2026, week: 11, seasonType: 2, teamSeason: 2026, teamData: schedule}
		card := m.spotlightPanel(inner)
		for _, want := range []string{"NEXT", "W12", "@ LAR", "BYE WEEK 11", "LAST", "W 24-17"} {
			if !strings.Contains(card, want) {
				t.Errorf("width %d: missing %q from %s", inner, want, card)
			}
		}
		for _, line := range strings.Split(card, "\n") {
			if lipgloss.Width(line) > inner+4 {
				t.Errorf("width %d: spotlight overflowed: %q", inner, line)
			}
		}
	}
}

func TestFavoriteSpotlightMovesPastJustFinishedGame(t *testing.T) {
	var schedule teamSchedule
	if err := json.Unmarshal([]byte(`{"events":[{"id":"finished","date":"2026-10-25T17:00Z","week":{"number":7},"competitions":[{"status":{"type":{"state":"pre"}},"competitors":[{"homeAway":"away","team":{"abbreviation":"GB"}},{"homeAway":"home","team":{"abbreviation":"DET"}}]}]},{"id":"following","date":"2026-11-01T18:00Z","week":{"number":8},"competitions":[{"status":{"type":{"state":"pre"}},"competitors":[{"homeAway":"home","team":{"abbreviation":"GB"}},{"homeAway":"away","team":{"abbreviation":"CAR"}}]}]}]}`), &schedule); err != nil {
		t.Fatal(err)
	}
	var board scoreboard
	if err := json.Unmarshal([]byte(`{"events":[{"id":"finished","date":"2026-10-25T17:00Z","status":{"type":{"state":"post","shortDetail":"Final"}},"competitions":[{"competitors":[{"homeAway":"away","score":"27","team":{"abbreviation":"GB"}},{"homeAway":"home","score":"20","team":{"abbreviation":"DET"}}]}]}]}`), &board); err != nil {
		t.Fatal(err)
	}
	m := model{favorite: "GB", season: 2026, week: 7, seasonType: 2, board: board, teamSeason: 2026, teamData: schedule}
	card := m.spotlightPanel(74)
	for _, want := range []string{"NEXT", "W8", "vs CAR", "LAST", "W 27-20", "@ DET"} {
		if !strings.Contains(card, want) {
			t.Errorf("missing %q from %s", want, card)
		}
	}
}

func TestDashboardPrefetchesScheduleAndDefersLeaders(t *testing.T) {
	m := model{favorite: "GB", season: 2026, week: 7, seasonType: 2}
	next, cmd := m.Update(boardMsg{season: 2026, week: 7, seasonType: 2})
	m = next.(model)
	if !m.teamLoading || cmd == nil {
		t.Fatal("board load did not start favorite schedule prefetch")
	}
	var schedule teamSchedule
	schedule.Events = []scheduledGame{{ID: "game"}}
	next, cmd = m.Update(teamScheduleMsg{season: 2026, team: "GB", data: schedule})
	m = next.(model)
	if m.teamLoading || m.teamSeason != 2026 || m.teamLeadersLoading || cmd != nil {
		t.Fatal("schedule prefetch started season leader requests")
	}
	m, cmd = m.openFavoriteSeason("")
	if !m.teamLeadersLoading || cmd == nil {
		t.Fatal("opening season did not start player leader requests")
	}
}

func TestHomeMenuFitsAndOpensViews(t *testing.T) {
	for _, size := range []struct{ width, height int }{{60, 16}, {80, 24}, {150, 36}} {
		m := model{showHome: true, favorite: "GB", width: size.width, height: size.height}
		view := m.View()
		lines := strings.Split(view, "\n")
		if len(lines) > size.height {
			t.Errorf("%dx%d home has %d lines", size.width, size.height, len(lines))
		}
		for i, line := range lines {
			if lipgloss.Width(line) > size.width {
				t.Errorf("%dx%d home line %d is %d columns", size.width, size.height, i, lipgloss.Width(line))
			}
		}
		if !strings.Contains(view, "PACKERS WATCH") || !strings.Contains(view, "NFC STANDINGS") {
			t.Errorf("%dx%d home menu options missing", size.width, size.height)
		}
	}
	m := model{showHome: true, favorite: "GB", season: 2026, homeSelection: 2}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(model)
	if m.showHome || !m.favoritesOnly {
		t.Fatal("Packers menu item did not open favorite dashboard")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(model)
	if !m.showHome {
		t.Fatal("Esc did not return to home menu")
	}
	m.homeSelection = 5
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(model)
	if !m.showStandings || m.standingsConference != 0 {
		t.Fatal("AFC menu item opened wrong conference")
	}
	var data standings
	if err := json.Unmarshal([]byte(`{"children":[{"abbreviation":"AFC","standings":{"entries":[]}},{"abbreviation":"NFC","standings":{"entries":[{"team":{"abbreviation":"GB"}}]}}]}`), &data); err != nil {
		t.Fatal(err)
	}
	next, _ = m.Update(standingsMsg{season: 2026, data: data})
	m = next.(model)
	if m.standingsConference != 0 {
		t.Fatal("standings response overrode selected AFC view")
	}

	var board scoreboard
	if err := json.Unmarshal([]byte(`{"events":[{"id":"live","status":{"type":{"state":"in"}}},{"id":"next","status":{"type":{"state":"pre"}}}]}`), &board); err != nil {
		t.Fatal(err)
	}
	liveMenu := model{showHome: true, board: board, homeSelection: 1}
	next, _ = liveMenu.Update(tea.KeyMsg{Type: tea.KeyEnter})
	liveMenu = next.(model)
	if !liveMenu.liveOnly || len(liveMenu.visibleGames()) != 1 || liveMenu.visibleGames()[0].ID != "live" {
		t.Fatal("Live Now menu did not filter active game")
	}
}

func TestSummaryCollectsAndOrdersAllPlays(t *testing.T) {
	api := jsonClient(t, func(req *http.Request) {
		if req.URL.Path != "/apis/site/v2/sports/football/nfl/summary" || req.URL.Query().Get("event") != "123" {
			t.Errorf("unexpected summary request: %s", req.URL)
		}
	}, `{"boxscore":{},"drives":{"previous":[{"team":{"abbreviation":"GB"},"plays":[{"id":"1","sequenceNumber":"100","text":"First"},{"id":"2","sequenceNumber":"200","text":"Second"}]}],"current":{"team":{"abbreviation":"GB"},"plays":[{"id":"2","sequenceNumber":"200","text":"Second"},{"id":"3","sequenceNumber":"300","text":"Third"}]}}}`)
	detail, err := api.summary(context.Background(), "123")
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Plays) != 3 || detail.Plays[0].ID != "3" || detail.Plays[2].ID != "1" || detail.Plays[0].Team != "GB" {
		t.Fatalf("unexpected play order or duplicates: %+v", detail.Plays)
	}
}

func TestStandingsUsesConferenceFeed(t *testing.T) {
	api := jsonClient(t, func(req *http.Request) {
		if req.URL.Path != "/apis/v2/sports/football/nfl/standings" || req.URL.Query().Get("season") != "2026" {
			t.Errorf("unexpected standings request: %s", req.URL)
		}
	}, `{"children":[{"abbreviation":"NFC","standings":{"entries":[{"team":{"abbreviation":"GB","displayName":"Green Bay Packers"},"stats":[{"name":"wins","displayValue":"4"}]}]}}]}`)
	data, err := api.standings(context.Background(), 2026)
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Children) != 1 || data.Children[0].Standings.Entries[0].stat("wins") != "4" {
		t.Fatal("standings record missing")
	}
}

func TestStandingsKeyboardAndWheelScrolling(t *testing.T) {
	group := standingGroup{Abbreviation: "AFC"}
	for i := 1; i <= 16; i++ {
		var entry standingEntry
		entry.Team.Abbreviation = fmt.Sprintf("T%02d", i)
		entry.Team.DisplayName = fmt.Sprintf("Team %02d", i)
		group.Standings.Entries = append(group.Standings.Entries, entry)
	}
	m := model{season: 2026, standingsSeason: 2026, standings: standings{Children: []standingGroup{group}}, showStandings: true, width: 80, height: 24}
	if m.maxStandingsScroll() != 1 || !strings.Contains(m.View(), "1–15 OF 16") || strings.Contains(m.View(), "T16") {
		t.Fatal("first standings page or visible range is wrong")
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(model)
	if m.standingsScroll != 1 || !strings.Contains(m.View(), "2–16 OF 16") || !strings.Contains(m.View(), "T16") {
		t.Fatal("Down did not reveal final team")
	}
	next, _ = m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelUp})
	m = next.(model)
	if m.standingsScroll != 0 {
		t.Fatal("mouse wheel up did not scroll")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	m = next.(model)
	if m.standingsScroll != 1 {
		t.Fatal("Page Down did not scroll")
	}
	m.height = 36
	if m.maxStandingsScroll() != 0 || m.standingsRange() != "ALL 16 TEAMS" {
		t.Fatal("tall terminal should show all teams")
	}
}
