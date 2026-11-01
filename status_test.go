package main

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

const liveBoardJSON = `{"week":{"number":9},"events":[{"id":"1","date":"2026-11-01T18:00Z","status":{"type":{"state":"in","shortDetail":"2:14 - 4th","description":"In Progress"},"displayClock":"2:14","period":4},"competitions":[{"situation":{"possession":"8","down":3,"distance":7,"isRedZone":false},"competitors":[{"homeAway":"home","score":"27","team":{"id":"8","abbreviation":"GB"}},{"homeAway":"away","score":"19","team":{"id":"29","abbreviation":"CAR"}}]}]}]}`

func TestScoreboardURLMatchesCacheKey(t *testing.T) {
	var seen []string
	api := jsonClient(t, func(req *http.Request) { seen = append(seen, req.URL.String()) }, `{"week":{"number":7},"events":[{"id":"1"}]}`)
	if _, err := api.scoreboard(context.Background(), 2026, 7, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := api.scoreboard(context.Background(), 0, 0, 2); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 || seen[0] != scoreboardURL(2026, 7, 2) || seen[1] != scoreboardURL(0, 0, 2) {
		t.Fatalf("urls %v vs %q %q", seen, scoreboardURL(2026, 7, 2), scoreboardURL(0, 0, 2))
	}
}

func captureSpawns(t *testing.T) *[][]string {
	t.Helper()
	var spawned [][]string
	spawnDetached = func(self string, args []string) error {
		spawned = append(spawned, append([]string{self}, args...))
		return nil
	}
	t.Cleanup(func() { spawnDetached = realSpawnDetached })
	return &spawned
}

func TestRunStatusOnceReadsCacheWithoutNetwork(t *testing.T) {
	dir := t.TempDir()
	if err := writeCached(dir, scoreboardURL(0, 0, 2), []byte(liveBoardJSON)); err != nil {
		t.Fatal(err)
	}
	spawned := captureSpawns(t)
	var out strings.Builder
	code := runStatusOnce(dir, "GB", 0, 0, 2, time.Now(), "/bin/td", []string{"-favorite=GB"}, &out)
	if code != 0 || out.String() != "GB 27-19 CAR · Q4 2:14 · BALL\n" {
		t.Fatalf("code %d out %q", code, out.String())
	}
	if len(*spawned) != 0 {
		t.Fatalf("fresh cache must not spawn: %v", *spawned)
	}
}

func TestRunStatusOnceSpawnsRefreshWhenStale(t *testing.T) {
	dir := t.TempDir()
	if err := writeCached(dir, scoreboardURL(0, 0, 2), []byte(liveBoardJSON)); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-45 * time.Second)
	if err := os.Chtimes(cachePath(dir, scoreboardURL(0, 0, 2)), old, old); err != nil {
		t.Fatal(err)
	}
	spawned := captureSpawns(t)
	var out strings.Builder
	if code := runStatusOnce(dir, "GB", 0, 0, 2, time.Now(), "/bin/td", []string{"-favorite=GB"}, &out); code != 0 {
		t.Fatal(code)
	}
	want := []string{"/bin/td", "-status", "-refresh", "-favorite=GB"}
	if out.String() != "GB 27-19 CAR · Q4 2:14 · BALL\n" || len(*spawned) != 1 || !reflect.DeepEqual((*spawned)[0], want) {
		t.Fatalf("out %q spawned %v", out.String(), *spawned)
	}
	runStatusOnce(dir, "GB", 0, 0, 2, time.Now(), "/bin/td", nil, &out)
	if len(*spawned) != 1 {
		t.Fatalf("marker must throttle the second spawn: %v", *spawned)
	}
}

func TestRunStatusOnceNoCache(t *testing.T) {
	for _, dir := range []string{t.TempDir(), filepath.Join(t.TempDir(), "nested")} {
		spawned := captureSpawns(t)
		var out strings.Builder
		if code := runStatusOnce(dir, "GB", 0, 0, 2, time.Now(), "/bin/td", nil, &out); code != 0 || out.String() != "" {
			t.Fatalf("code %d out %q", code, out.String())
		}
		if len(*spawned) != 1 {
			t.Fatalf("expected one spawn, got %v", *spawned)
		}
		if _, err := os.Stat(filepath.Join(dir, "status-refresh")); err != nil {
			t.Fatalf("marker missing: %v", err)
		}
	}
}

func TestRunStatusOnceCorruptCache(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(cachePath(dir, scoreboardURL(0, 0, 2)), []byte("{not json"), 0600); err != nil {
		t.Fatal(err)
	}
	spawned := captureSpawns(t)
	var out strings.Builder
	if code := runStatusOnce(dir, "GB", 0, 0, 2, time.Now(), "/bin/td", nil, &out); code != 0 || out.String() != "" || len(*spawned) != 1 {
		t.Fatalf("code %d out %q spawned %v", code, out.String(), *spawned)
	}
}

func TestRunRefreshWritesCache(t *testing.T) {
	api := jsonClient(t, func(*http.Request) {}, liveBoardJSON)
	api.cacheDir = t.TempDir()
	if code := runRefresh(api, 0, 0, 2); code != 0 {
		t.Fatal(code)
	}
	board, _, ok := cachedScoreboard(api.cacheDir, 0, 0, 2)
	if !ok || len(board.Events) != 1 {
		t.Fatalf("cache not written: %v %+v", ok, board)
	}
}
