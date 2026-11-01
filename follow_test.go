package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// matchup is a one-game board for any two teams.
func matchup(t *testing.T, id, state, away, home, awayScore string) scoreboard {
	t.Helper()
	var b scoreboard
	raw := fmt.Sprintf(`{"week":{"number":9},"events":[{"id":%q,"date":"2026-11-01T21:25Z","status":{"type":{"state":%q,"shortDetail":"2:14 - 4th"},"displayClock":"2:14","period":4},"competitions":[{"competitors":[{"homeAway":"away","score":%q,"team":{"id":"1","abbreviation":%q}},{"homeAway":"home","score":"0","team":{"id":"2","abbreviation":%q}}]}]}]}`, id, state, awayScore, away, home)
	if err := json.Unmarshal([]byte(raw), &b); err != nil {
		t.Fatal(err)
	}
	return b
}

// twoGameBoard has GB @ CAR and KC @ DET in the given states.
func twoGameBoard(t *testing.T, gbState, kcState string) scoreboard {
	t.Helper()
	board := matchup(t, "1", gbState, "GB", "CAR", "7")
	board.Events = append(board.Events, matchup(t, "2", kcState, "KC", "DET", "7").Events...)
	return board
}

func TestLoadConfigResolvesFollowedTeams(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"favorite_team":"GB","followed_teams":["kc","Lions","GB","KC"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	config, err := loadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(config.FollowedTeams, ","); got != "KC,DET" {
		t.Fatalf("followed %s", got)
	}
	if err := os.WriteFile(path, []byte(`{"followed_teams":["nope"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadConfig(path); err == nil {
		t.Fatal("an unknown followed team must be an error")
	}
}

func TestParseFollowedAcceptsAListAndNothing(t *testing.T) {
	teams, err := parseFollowed(" kc, Lions ,GB", "GB")
	if err != nil || strings.Join(teams, ",") != "KC,DET" {
		t.Fatalf("teams %v err %v", teams, err)
	}
	if teams, err := parseFollowed("", "GB"); err != nil || len(teams) != 0 {
		t.Fatalf("teams %v err %v", teams, err)
	}
	if _, err := parseFollowed("nope", "GB"); err == nil {
		t.Fatal("an unknown team must be an error")
	}
}

func TestSaveFollowedKeepsOtherSettingsAndClears(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := saveSettings(path, "BUF", "amber"); err != nil {
		t.Fatal(err)
	}
	if err := saveFollowed(path, []string{"KC"}); err != nil {
		t.Fatal(err)
	}
	config, err := loadConfig(path)
	if err != nil || config.FavoriteTeam != "BUF" || config.Theme != "amber" || strings.Join(config.FollowedTeams, ",") != "KC" {
		t.Fatalf("config %+v err %v", config, err)
	}
	if err := saveFollowed(path, nil); err != nil {
		t.Fatal(err)
	}
	if config, _ := loadConfig(path); len(config.FollowedTeams) != 0 {
		t.Fatalf("config %+v", config)
	}
}

func TestStatusSummaryJoinsFollowedGamesFavoriteFirst(t *testing.T) {
	now := time.Now()
	board := twoGameBoard(t, "post", "in")
	got := statusSummary(board, "GB", []string{"KC"}, now, time.Time{}, false)
	if !strings.HasPrefix(got, "GB ") || !strings.Contains(got, " | KC ") {
		t.Fatalf("got %q", got)
	}
	if got := statusSummary(board, "GB", nil, now, time.Time{}, false); strings.Contains(got, "|") {
		t.Fatalf("got %q", got)
	}
}

func TestStatusSummaryShowsSharedGameOnce(t *testing.T) {
	board := twoGameBoard(t, "in", "in")
	got := statusSummary(board, "GB", []string{"CAR"}, time.Now(), time.Time{}, false)
	if strings.Contains(got, "|") {
		t.Fatalf("got %q", got)
	}
}

func TestStatusSummaryLiveOnlyKeepsLiveGames(t *testing.T) {
	now := time.Now()
	got := statusSummary(twoGameBoard(t, "post", "in"), "GB", []string{"KC"}, now, time.Time{}, true)
	if strings.HasPrefix(got, "GB") || !strings.HasPrefix(got, "KC ") {
		t.Fatalf("got %q", got)
	}
	if got := statusSummary(twoGameBoard(t, "post", "post"), "GB", []string{"KC"}, now, time.Time{}, true); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestStatusSummaryMarksCachedOnce(t *testing.T) {
	now := time.Now()
	got := statusSummary(twoGameBoard(t, "post", "post"), "GB", []string{"KC"}, now, now.Add(-time.Hour), false)
	if strings.Count(got, "(cached)") != 1 || !strings.HasSuffix(got, "(cached)") {
		t.Fatalf("got %q", got)
	}
}

func TestTickerPinsFollowedGames(t *testing.T) {
	board := twoGameBoard(t, "post", "post")
	third := matchup(t, "3", "in", "SEA", "SF", "3")
	board.Events = append(third.Events, board.Events...)
	lines := tickerLines(board, "GB", time.Now(), 80, false, "KC")
	if !strings.HasPrefix(lines[2], "GB") || !strings.HasPrefix(lines[3], "KC") {
		t.Fatalf("lines %q", lines)
	}
	if strings.Join(lines, "\n") == "" || strings.Count(strings.Join(lines, "\n"), "KC") != 1 {
		t.Fatalf("a pinned game must not repeat: %q", lines)
	}
}

func TestScoreAlertsIncludeFollowedTeams(t *testing.T) {
	before, after := matchup(t, "1", "in", "KC", "DET", "0"), matchup(t, "1", "in", "KC", "DET", "7")
	if got := scoreAlerts(before, after, "GB", false); len(got) != 0 {
		t.Fatalf("alerts %+v", got)
	}
	if got := scoreAlerts(before, after, "GB", false, "KC"); len(got) != 1 {
		t.Fatalf("alerts %+v", got)
	}
}

func TestFavoriteFilterIncludesFollowedTeams(t *testing.T) {
	m := model{board: twoGameBoard(t, "pre", "pre"), favorite: "GB", followed: []string{"KC"}, favoritesOnly: true}
	if got := len(m.visibleGames()); got != 2 {
		t.Fatalf("games %d", got)
	}
	m.showHome = true
	if option := m.homeOptions()[2]; !strings.Contains(option, "(2)") {
		t.Fatalf("option %q", option)
	}
}

func settingsModel(t *testing.T, team string) model {
	t.Helper()
	m := model{favorite: "GB", theme: "mint", configPath: filepath.Join(t.TempDir(), "config.json"), width: 100, height: 40}.openSettings()
	for i, candidate := range nflTeams {
		if candidate.abbreviation == team {
			m.settingsTeam = i
		}
	}
	return m
}

func TestSpaceInSettingsTogglesFollowing(t *testing.T) {
	m := settingsModel(t, "KC")
	m = press(t, m, " ")
	if len(m.followed) != 1 || m.followed[0] != "KC" {
		t.Fatalf("followed %v", m.followed)
	}
	if config, err := loadConfig(m.configPath); err != nil || len(config.FollowedTeams) != 1 {
		t.Fatalf("config %+v err %v", config, err)
	}
	if view := stripANSI(m.View()); !strings.Contains(view, "Kansas City Chiefs  +") {
		t.Fatalf("view:\n%s", view)
	}
	m = press(t, m, " ")
	if len(m.followed) != 0 {
		t.Fatalf("followed %v", m.followed)
	}
	m = press(t, settingsModel(t, "GB"), " ")
	if len(m.followed) != 0 {
		t.Fatalf("the favorite cannot also be followed: %v", m.followed)
	}
}

func TestChoosingAFollowedTeamAsFavoriteDropsItFromFollowed(t *testing.T) {
	m := settingsModel(t, "KC")
	m = press(t, m, " ")
	m = press(t, m, "enter")
	if m.favorite != "KC" || len(m.followed) != 0 {
		t.Fatalf("favorite %q followed %v", m.favorite, m.followed)
	}
}

func TestStatusNotifyRequiresTheTicker(t *testing.T) {
	if err := validateStatusFlags(true, true, false, false, false, true, false); err != nil {
		t.Fatal(err)
	}
	if err := validateStatusFlags(true, false, true, false, false, true, false); err != nil {
		t.Fatal(err)
	}
	if err := validateStatusFlags(true, false, false, false, false, true, false); err == nil {
		t.Fatal("-status -notify without a ticker has nothing to alert from")
	}
}

func TestForwardedFlagsKeepAlerts(t *testing.T) {
	fs := flag.NewFlagSet("td", flag.ContinueOnError)
	fs.Bool("notify", false, "")
	fs.Bool("notify-all", false, "")
	if err := fs.Parse([]string{"-notify", "-notify-all"}); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(forwardedFlags(fs), " "); got != "-notify=true -notify-all=true" {
		t.Fatalf("flags %q", got)
	}
}

func TestWatchSendsAlertsForNewScores(t *testing.T) {
	var mu sync.Mutex
	fetches := 0
	api := &client{http: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		mu.Lock()
		defer mu.Unlock()
		fetches++
		score := "0"
		if fetches > 1 {
			score = "7"
		}
		body := fmt.Sprintf(`{"events":[{"id":"1","status":{"type":{"state":"in"}},"competitions":[{"competitors":[{"homeAway":"away","score":%q,"team":{"abbreviation":"KC"}},{"homeAway":"home","score":"0","team":{"abbreviation":"DET"}}]}]}]}`, score)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var sent []scoreAlert
	options := watchOptions{followed: []string{"KC"}, notify: true, send: func(a scoreAlert) error {
		mu.Lock()
		defer mu.Unlock()
		sent = append(sent, a)
		cancel()
		return nil
	}}
	done := make(chan int, 1)
	go func() { done <- runWatch(ctx, api, "GB", 0, 0, 2, io.Discard, time.Millisecond, options) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("no alert was sent")
	}
	if len(sent) != 1 || !strings.Contains(sent[0].body, "KC 7") {
		t.Fatalf("alerts %+v", sent)
	}
}

func TestWatchReportsAFailedAlertOnce(t *testing.T) {
	var failures int
	var buffer strings.Builder
	options := watchOptions{notify: true, send: func(scoreAlert) error { return fmt.Errorf("no notify-send") }}
	alerter := options.deliver(&buffer)
	for range 3 {
		failures++
		alerter(scoreAlert{title: "t"})
	}
	if got := strings.Count(buffer.String(), "no notify-send"); got != 1 || failures != 3 {
		t.Fatalf("reported %d times: %q", got, buffer.String())
	}
}

func TestVersionDefaultsToDevelopmentBuild(t *testing.T) {
	if version == "" {
		t.Fatal("version must never be empty")
	}
}

func TestFollowNoteExplainsADroppedFavorite(t *testing.T) {
	if got := followNote("KC, packers", "GB"); got != "GB is your favorite, so it is always shown." {
		t.Fatalf("note %q", got)
	}
	if got := followNote("KC,DET", "GB"); got != "" {
		t.Fatalf("note %q", got)
	}
	if got := followNote("", "GB"); got != "" {
		t.Fatalf("note %q", got)
	}
}
