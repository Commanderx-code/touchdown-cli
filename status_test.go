package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"os/exec"
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

func tickerBoard() scoreboard {
	mk := func(id, date, state, short, clock string, period int, away, awayScore, home, homeScore string, sit gameSituation) game {
		return game{ID: id, Date: date, Status: gameStatus{Type: statusType{State: state, ShortDetail: short, Description: short}, DisplayClock: clock, Period: period},
			Competitions: []competition{{Situation: sit, Competitors: []competitor{{HomeAway: "away", Score: awayScore, Team: teamRef{ID: away, Abbreviation: away}}, {HomeAway: "home", Score: homeScore, Team: teamRef{ID: home, Abbreviation: home}}}}}}
	}
	b := scoreboard{Events: []game{
		mk("1", "2026-11-01T18:00Z", "post", "Final", "0:00", 4, "KC", "21", "DEN", "24", gameSituation{}),
		mk("2", "2026-11-01T18:00Z", "in", "8:01 - 3rd", "8:01", 3, "BAL", "14", "BUF", "17", gameSituation{}),
		mk("3", "2026-11-01T21:25Z", "in", "2:14 - 4th", "2:14", 4, "CAR", "19", "GB", "27", gameSituation{Possession: "GB", Down: 3, Distance: 7, IsRedZone: true}),
		mk("4", "2026-11-02T01:20Z", "pre", "8:20 PM", "", 0, "PHI", "", "WSH", "", gameSituation{}),
	}}
	b.Week.Number = 9
	return b
}

func TestTickerLinesOrderAndShape(t *testing.T) {
	now := time.Date(2026, 11, 1, 14, 32, 7, 0, time.FixedZone("EST", -5*3600))
	lines := tickerLines(tickerBoard(), "GB", now, 80, false)
	if len(lines) != 7 {
		t.Fatalf("want 7 lines, got %d:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	if lines[0] != "Week 9 · updated 14:32:07" || lines[1] != "" || lines[3] != "" {
		t.Fatalf("header/blank lines wrong:\n%s", strings.Join(lines, "\n"))
	}
	if !strings.HasPrefix(lines[2], "CAR  19  GB   27   ") || !strings.HasSuffix(lines[2], "BALL GB · 3rd & 7 · RZ") {
		t.Fatalf("favorite line wrong: %q", lines[2])
	}
	if !strings.HasPrefix(lines[4], "BAL  14  BUF  17   8:01 - 3rd") || !strings.HasPrefix(lines[5], "PHI      WSH      ") || !strings.HasSuffix(lines[5], "8:20 PM") || !strings.HasPrefix(lines[6], "KC   21  DEN  24   Final") {
		t.Fatalf("group order wrong:\n%s", strings.Join(lines, "\n"))
	}
	if strings.Count(strings.Join(lines, "\n"), "GB ") != 2 {
		t.Fatalf("favorite must appear once (score line and BALL):\n%s", strings.Join(lines, "\n"))
	}
}

func TestTickerLinesClipToWidth(t *testing.T) {
	for _, l := range tickerLines(tickerBoard(), "GB", time.Now(), 20, false) {
		if n := len([]rune(l)); n > 20 {
			t.Fatalf("line too long (%d): %q", n, l)
		}
	}
}

func TestTickerLinesCachedHeaderAndNoFavorite(t *testing.T) {
	lines := tickerLines(tickerBoard(), "SEA", time.Now(), 80, true)
	if !strings.HasSuffix(lines[0], " · cached") {
		t.Fatalf("header: %q", lines[0])
	}
	if lines[1] != "" || lines[2] == "" {
		t.Fatalf("without a favorite game the list follows the header directly:\n%s", strings.Join(lines, "\n"))
	}
}

func TestRunWatchStopsOnCancel(t *testing.T) {
	api := jsonClient(t, func(*http.Request) {}, liveBoardJSON)
	ctx, cancel := context.WithCancel(context.Background())
	var out strings.Builder
	go func() { time.Sleep(35 * time.Millisecond); cancel() }()
	if code := runWatch(ctx, api, "GB", 0, 0, 2, &out, 10*time.Millisecond); code != 0 {
		t.Fatal(code)
	}
	if n := strings.Count(out.String(), "\033[H\033[2J"); n < 2 {
		t.Fatalf("expected at least 2 redraws, got %d", n)
	}
	if !strings.Contains(out.String(), "CAR  19  GB   27") {
		t.Fatalf("ticker missing game:\n%s", out.String())
	}
}

func TestStatusOutputStripsControlCharacters(t *testing.T) {
	board := statusBoard("in", "2:14\x1b[31m", 4, "In Progress", "2:14 - 4th\r\n", "19", "27", "", false)
	board.Events[0].Competitions[0].Competitors[0].Team.Abbreviation = "CAR\x07"
	now := time.Date(2026, 11, 1, 14, 30, 0, 0, time.UTC)
	if got := statusLine(board, "GB", now, time.Time{}); got != "GB 27-19 CAR · Q4 2:14[31m" {
		t.Fatalf("status line not sanitized: %q", got)
	}
	for _, l := range tickerLines(board, "GB", now, 80, false) {
		if strings.ContainsAny(l, "\x1b\x07\r\n") {
			t.Fatalf("ticker line not sanitized: %q", l)
		}
	}
}

func TestDetectHostPrefersInnermost(t *testing.T) {
	cases := []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{"TMUX": "/tmp/tmux-1000/default,1,0", "KITTY_PID": "4242"}, "tmux"},
		{map[string]string{"ZELLIJ": "0", "WEZTERM_PANE": "3"}, "zellij"},
		{map[string]string{"HERDR_ENV": "1"}, "herdr"},
		{map[string]string{"HERDR_ENV": "0"}, ""},
		{map[string]string{"KITTY_PID": "4242"}, "kitty"},
		{map[string]string{"WEZTERM_PANE": "3"}, "wezterm"},
		{map[string]string{}, ""},
	}
	for _, tc := range cases {
		if got := detectHost(func(k string) string { return tc.env[k] }); got != tc.want {
			t.Errorf("%v: got %q want %q", tc.env, got, tc.want)
		}
	}
}

func TestShellQuote(t *testing.T) {
	if got := shellQuote([]string{"/opt/my td/td", "-favorite=Green Bay", "it's"}); got != `'/opt/my td/td' '-favorite=Green Bay' 'it'\''s'` {
		t.Fatal(got)
	}
}

func captureHostCommands(t *testing.T, respond func(name string, args []string) ([]byte, error)) *[][]string {
	t.Helper()
	var calls [][]string
	runHostCommand = func(name string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string{name}, args...))
		return respond(name, args)
	}
	t.Cleanup(func() { runHostCommand = realRunHostCommand })
	return &calls
}

func TestOpenSplitCommands(t *testing.T) {
	calls := captureHostCommands(t, func(name string, args []string) ([]byte, error) {
		if name == "herdr" && args[1] == "split" {
			return []byte(`{"result":{"pane":{"pane_id":"w1:p3"}}}`), nil
		}
		return nil, nil
	})
	child := []string{"/bin/td", "-status", "-watch", "-favorite=GB"}
	cases := []struct {
		host string
		want [][]string
	}{
		{"tmux", [][]string{append([]string{"tmux", "split-window", "-h", "-l", "36", "-d"}, child...)}},
		{"zellij", [][]string{append([]string{"zellij", "run", "-d", "right", "-n", "NFL", "--"}, child...)}},
		{"kitty", [][]string{append([]string{"kitten", "@", "launch", "--location=vsplit", "--keep-focus"}, child...)}},
		{"wezterm", [][]string{append([]string{"wezterm", "cli", "split-pane", "--right", "--percent", "35", "--"}, child...)}},
		{"herdr", [][]string{
			{"herdr", "pane", "split", "--current", "--direction", "right", "--ratio", "0.35", "--no-focus"},
			{"herdr", "pane", "run", "w1:p3", "'/bin/td' '-status' '-watch' '-favorite=GB'"},
		}},
	}
	for _, tc := range cases {
		*calls = nil
		if code := openSplit(tc.host, "/bin/td", []string{"-favorite=GB"}, io.Discard); code != 0 {
			t.Errorf("%s: exit %d", tc.host, code)
		}
		if !reflect.DeepEqual(*calls, tc.want) {
			t.Errorf("%s:\n got %q\nwant %q", tc.host, *calls, tc.want)
		}
	}
}

func TestOpenSplitFailures(t *testing.T) {
	var errOut strings.Builder
	calls := captureHostCommands(t, func(name string, args []string) ([]byte, error) {
		switch name {
		case "herdr":
			return []byte(`{"error":"no focused pane"}`), nil
		case "tmux":
			return nil, &exec.ExitError{ProcessState: &os.ProcessState{}, Stderr: []byte("no server running")}
		}
		return nil, nil
	})
	if code := openSplit("", "/bin/td", nil, &errOut); code != 1 || !strings.Contains(errOut.String(), "Run this in a new pane:") || !strings.Contains(errOut.String(), "'/bin/td' '-status' '-watch'") {
		t.Fatalf("unknown host: code %d out %q", code, errOut.String())
	}
	errOut.Reset()
	*calls = nil
	if code := openSplit("herdr", "/bin/td", nil, &errOut); code != 1 || len(*calls) != 1 || !strings.Contains(errOut.String(), "Run this in a new pane:") {
		t.Fatalf("herdr without pane id: code %d calls %q out %q", code, *calls, errOut.String())
	}
	errOut.Reset()
	if code := openSplit("tmux", "/bin/td", nil, &errOut); code != 1 || !strings.Contains(errOut.String(), "tmux failed") || !strings.Contains(errOut.String(), "no server running") {
		t.Fatalf("tmux failure: code %d out %q", code, errOut.String())
	}
}
