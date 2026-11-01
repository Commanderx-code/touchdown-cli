package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"
)

const partialFeedJSON = `{"injuries":[
 {"status":"Out","type":{"abbreviation":"O"},"athlete":{"displayName":"Micah Parsons","position":{"abbreviation":"DE"},"links":[{"href":"https://www.espn.com/nfl/player/_/id/1/micah-parsons"}]},"details":{"type":"Knee"},"date":"2026-10-31T14:09Z"},
 {"status":"Active","type":{"abbreviation":"A"},"athlete":{"displayName":"Cleared Back","position":{"abbreviation":"RB"},"links":[{"href":"https://www.espn.com/nfl/player/_/id/2/cleared-back"}]},"date":"2026-10-31T14:09Z"}
]}`

const hurtRosterJSON = `{"athletes":[
 {"position":"offense","items":[
  {"id":"2","displayName":"Cleared Back","position":{"abbreviation":"RB"},"injuries":[{"status":"Out"}]},
  {"id":"5","displayName":"Healthy Guy","position":{"abbreviation":"WR"}}
 ]},
 {"position":"injuredReserveOrOut","items":[
  {"id":"1","displayName":"Micah Parsons","position":{"abbreviation":"DE"},"injuries":[{"status":"Out"}]},
  {"id":"3","displayName":"Jayden Reed","position":{"abbreviation":"WR"},"injuries":[{"status":"Injured Reserve"}]},
  {"id":"4","displayName":"Luke Musgrave","position":{"abbreviation":"TE"},"injuries":[{"status":"Out"}]},
  {"id":"6","displayName":"Banned Guy","position":{"abbreviation":"S"},"injuries":[{"status":"Suspension"}]}
 ]}
]}`

// partialFeedClient serves a feed that has lost most of the team's injured
// players, as ESPN's 25-row page does, and a roster that still lists them.
func partialFeedClient(t *testing.T, rosterFails bool) *client {
	t.Helper()
	return &client{http: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch {
		case strings.HasSuffix(req.URL.Path, "/roster"):
			if rosterFails {
				return nil, errors.New("roster down")
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(hurtRosterJSON)), Header: make(http.Header)}, nil
		case strings.HasSuffix(req.URL.Path, "/injuries"):
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(partialFeedJSON)), Header: make(http.Header)}, nil
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{}`)), Header: make(http.Header)}, nil
	})}}
}

func entryNames(r injuryReport) []string {
	var names []string
	for _, e := range r.Entries {
		names = append(names, e.Name)
	}
	return names
}

func TestFullInjuriesAddsPlayersTheFeedDropped(t *testing.T) {
	report, err := partialFeedClient(t, false).fullInjuries(context.Background(), "GB")
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(entryNames(report), ",")
	if got != "Micah Parsons,Luke Musgrave,Jayden Reed,Banned Guy" {
		t.Fatalf("entries %s", got)
	}
	if out, _, _, ir := report.counts(); out != 2 || ir != 1 {
		t.Fatalf("counts out %d ir %d", out, ir)
	}
	if report.Entries[0].BodyPart != "Knee" {
		t.Fatalf("the feed's row must win: %+v", report.Entries[0])
	}
	if report.Entries[1].AthleteID != "4" || report.Entries[1].Position != "TE" {
		t.Fatalf("roster row %+v", report.Entries[1])
	}
	if label := statusLabel(report.Entries[3]); label != "SUSP" {
		t.Fatalf("label %q", label)
	}
}

func TestFullInjuriesSurvivesRosterFailure(t *testing.T) {
	report, err := partialFeedClient(t, true).fullInjuries(context.Background(), "GB")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(entryNames(report), ","); got != "Micah Parsons" {
		t.Fatalf("entries %s", got)
	}
}

func TestRosterFromCacheMarksTheReportCached(t *testing.T) {
	saved := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	if got := (injuryReport{}).withRoster(roster{CachedAt: saved}); !got.CachedAt.Equal(saved) {
		t.Fatalf("cached at %v", got.CachedAt)
	}
}

func TestInjuryTabShowsPlayersMissingFromTheFeed(t *testing.T) {
	m := seasonTabsModel(t, nil)
	m.api = partialFeedClient(t, false)
	m = press(t, m, "3")
	if view := m.View(); !strings.Contains(view, "Luke Musgrave") {
		t.Fatalf("roster-only player missing:\n%s", view)
	}
}

func TestTabsMarkCachedData(t *testing.T) {
	saved := time.Date(2026, 10, 30, 9, 0, 0, 0, time.Local)
	m := seasonTabsModel(t, nil)
	m = press(t, m, "2")
	entry := m.rosters["GB"]
	entry.Data.CachedAt = saved
	m.rosters["GB"] = entry
	if view := stripANSI(m.View()); !strings.Contains(view, "CACHED · last saved Oct 30") {
		t.Fatalf("roster tab:\n%s", view)
	}
	m = press(t, m, "3")
	report := m.injuryReports["GB"]
	report.Report.CachedAt = saved
	m.injuryReports["GB"] = report
	if view := stripANSI(m.View()); !strings.Contains(view, "CACHED · last saved Oct 30") {
		t.Fatalf("injury tab:\n%s", view)
	}
}

func TestTabsFlagFailedRefreshOverLoadedData(t *testing.T) {
	m := seasonTabsModel(t, nil)
	m = press(t, m, "2")
	m = m.applyRoster(rosterMsg{team: "GB", err: errors.New("boom")})
	view := stripANSI(m.View())
	if !strings.Contains(view, "UPDATE FAILED") || !strings.Contains(view, "OFFENSE") {
		t.Fatalf("roster tab:\n%s", view)
	}
	m = press(t, m, "3")
	m = m.applyInjuries(injuriesMsg{team: "GB", err: errors.New("boom")})
	view = stripANSI(m.View())
	if !strings.Contains(view, "UPDATE FAILED") || !strings.Contains(view, "INJURY REPORT") {
		t.Fatalf("injury tab:\n%s", view)
	}
}

func TestGameCenterInjuriesFlagStaleData(t *testing.T) {
	saved := time.Date(2026, 10, 30, 9, 0, 0, 0, time.Local)
	g := boardAt(t, "1", "pre", "", "").Events[0]
	away, home := g.sides()
	m := model{injuryReports: map[string]injuryEntry{
		away.Team.Abbreviation: {Report: injuryReport{Team: away.Team.Abbreviation, CachedAt: saved}},
		home.Team.Abbreviation: {Report: injuryReport{Team: home.Team.Abbreviation}, Error: "boom"},
	}}
	text := stripANSI(strings.Join(m.gameInjuryLines(g), "\n"))
	if !strings.Contains(text, "CACHED · last saved Oct 30") || !strings.Contains(text, "UPDATE FAILED") {
		t.Fatalf("lines:\n%s", text)
	}
}

func TestPlayerCardMarksCachedStats(t *testing.T) {
	saved := time.Date(2026, 10, 30, 9, 0, 0, 0, time.Local)
	m := model{player: playerRef{ID: "1", Name: "Jordan Love", Team: "GB"}, players: map[string]playerEntry{"1": {Stats: playerStats{ID: "1", CachedAt: saved}}}}
	if text := stripANSI(strings.Join(m.playerLines(60), "\n")); !strings.Contains(text, "CACHED · last saved Oct 30") {
		t.Fatalf("lines:\n%s", text)
	}
}

func TestOpponentJumpOnlyFromScheduleTab(t *testing.T) {
	for _, tab := range []string{"2", "3"} {
		m := press(t, seasonTabsModel(t, nil), tab)
		m = press(t, m, "t")
		if m.viewTeam != "" || len(m.history) != 0 {
			t.Fatalf("tab %s jumped to %q", tab, m.viewTeam)
		}
	}
	m := press(t, seasonTabsModel(t, nil), "t")
	if m.viewTeam == "" {
		t.Fatal("schedule tab must still jump to the opponent")
	}
}

var opponentHint = regexp.MustCompile(`·\s+t( opponent)?\s+·`)

func lastScreenLine(view string) string {
	lines := strings.Split(strings.TrimRight(stripANSI(view), "\n "), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

func TestSeasonFooterFitsEveryWidth(t *testing.T) {
	for width := 40; width <= 130; width++ {
		for _, tab := range []string{"1", "2", "3"} {
			m := press(t, seasonTabsModel(t, nil), tab)
			m.width, m.height = width, 30
			footer := lastScreenLine(m.View())
			if strings.Contains(footer, "…") || !strings.HasSuffix(footer, "q quit") && !strings.HasSuffix(footer, "· q") {
				t.Fatalf("width %d tab %s footer clipped: %q", width, tab, footer)
			}
			if onSchedule := tab == "1"; opponentHint.MatchString(footer) != onSchedule {
				t.Fatalf("width %d tab %s opponent hint wrong: %q", width, tab, footer)
			}
		}
	}
}

func TestCompactSeasonFooterNamesTabKeys(t *testing.T) {
	m := seasonTabsModel(t, nil)
	m.compact = true
	if view := stripANSI(m.View()); !strings.Contains(view, "1/2/3") {
		t.Fatalf("compact view:\n%s", view)
	}
}

func TestCompactSeasonHeaderNamesTheViewedTeam(t *testing.T) {
	m, _ := seasonTabsModel(t, nil).openTeamSeason("DET")
	m.compact = true
	view := stripANSI(m.View())
	if !strings.Contains(view, "LIONS SEASON") || strings.Contains(view, "PACKERS SEASON") {
		t.Fatalf("compact view:\n%s", view)
	}
}

func TestStartupInjuryLoadIsNotRepeated(t *testing.T) {
	var seen []string
	m := seasonTabsModel(t, &seen).markStartupLoads()
	m = press(t, m, "3")
	if got := fmt.Sprint(injuryRequests(seen)); got != "[]" {
		t.Fatalf("a report already loading must not be requested again: %s", got)
	}
	m = m.applyInjuries(injuriesMsg{team: "GB", report: injuryReport{Team: "GB"}})
	if m.injuryReports["GB"].Loading {
		t.Fatal("the startup reply must clear loading")
	}
}
