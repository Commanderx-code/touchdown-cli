package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func boardAt(t *testing.T, id, state, awayScore, homeScore string) scoreboard {
	t.Helper()
	var b scoreboard
	raw := `{"week":{"number":9},"events":[{"id":"` + id + `","date":"2026-11-01T21:25Z","status":{"type":{"state":"` + state + `","shortDetail":"2:14 - 4th"},"displayClock":"2:14","period":4},"competitions":[{"competitors":[{"homeAway":"away","score":"` + awayScore + `","team":{"id":"29","abbreviation":"CAR"}},{"homeAway":"home","score":"` + homeScore + `","team":{"id":"8","abbreviation":"GB"}}]}]}]}`
	if err := json.Unmarshal([]byte(raw), &b); err != nil {
		t.Fatal(err)
	}
	return b
}

func TestScoringEventsDetectsChangesAndFinals(t *testing.T) {
	events := scoringEvents(boardAt(t, "1", "in", "19", "20"), boardAt(t, "1", "in", "19", "27"), "GB")
	if len(events) != 1 || events[0].Team != "GB" || events[0].Points != 7 || events[0].Final || !events[0].Favorite || events[0].AwayScore != 19 || events[0].HomeScore != 27 || events[0].Period != 4 || events[0].Clock != "2:14" {
		t.Fatalf("touchdown: %+v", events)
	}
	events = scoringEvents(boardAt(t, "1", "in", "19", "27"), boardAt(t, "1", "post", "22", "27"), "DET")
	if len(events) != 1 || events[0].Team != "CAR" || events[0].Points != 3 || !events[0].Final || events[0].Favorite {
		t.Fatalf("walk-off must be one event: %+v", events)
	}
	events = scoringEvents(boardAt(t, "1", "in", "19", "27"), boardAt(t, "1", "post", "19", "27"), "GB")
	if len(events) != 1 || events[0].Points != 0 || !events[0].Final {
		t.Fatalf("plain final: %+v", events)
	}
	if events := scoringEvents(boardAt(t, "1", "pre", "", ""), boardAt(t, "1", "in", "0", "7"), "GB"); len(events) != 0 {
		t.Fatalf("kickoff score from pre must not be an event: %+v", events)
	}
	if events := scoringEvents(boardAt(t, "1", "in", "19", "27"), boardAt(t, "1", "in", "19", "27"), "GB"); len(events) != 0 {
		t.Fatalf("no change: %+v", events)
	}
}

func TestFeedLineFormats(t *testing.T) {
	e := scoringEvent{GameID: "1", Team: "GB", Points: 7, Away: "CAR", Home: "GB", AwayScore: 19, HomeScore: 27, Period: 4, Clock: "2:14"}
	if got := e.feedLine(); got != "Q4 2:14  GB +7  CAR 19 – 27 GB" {
		t.Fatalf("diff line: %q", got)
	}
	e.Favorite = true
	if got := e.feedLine(); got != "★ Q4 2:14  GB +7  CAR 19 – 27 GB" {
		t.Fatalf("favorite line: %q", got)
	}
	e.Favorite = false
	e.PlayType, e.PlayText = "TD", "J.Love pass to C.Watson for 12 yards, TOUCHDOWN"
	if got := e.feedLine(); got != "Q4 2:14  GB TD  CAR 19 – 27 GB  J.Love pass to C.Watson for 12 yards, TOUCHDOWN" {
		t.Fatalf("enriched line: %q", got)
	}
}

func scoringDetail(t *testing.T) gameDetail {
	t.Helper()
	api := jsonClient(t, func(*http.Request) {}, `{"scoringPlays":[{"id":"a","text":"FG","awayScore":19,"homeScore":20,"scoringType":{"abbreviation":"FG"},"team":{"abbreviation":"GB"}},{"id":"b","text":"J.Love pass to C.Watson for 12 yards, TOUCHDOWN","awayScore":19,"homeScore":27,"team":{"abbreviation":"GB"}}]}`)
	detail, err := api.summary(context.Background(), "1")
	if err != nil {
		t.Fatal(err)
	}
	return detail
}

func TestEnrichEventMatchesScores(t *testing.T) {
	detail := scoringDetail(t)
	e := scoringEvent{GameID: "1", Points: 7, AwayScore: 19, HomeScore: 27}
	got, ok := enrichEvent(e, detail)
	if !ok || got.PlayType != "+7" || !strings.HasPrefix(got.PlayText, "J.Love") {
		t.Fatalf("enrich without scoringType: %v %+v", ok, got)
	}
	got, ok = enrichEvent(scoringEvent{GameID: "1", Points: 3, AwayScore: 19, HomeScore: 20}, detail)
	if !ok || got.PlayType != "FG" {
		t.Fatalf("enrich FG: %v %+v", ok, got)
	}
	if _, ok := enrichEvent(scoringEvent{GameID: "1", AwayScore: 19, HomeScore: 24}, detail); ok {
		t.Fatal("no matching play must not enrich")
	}
}

func TestPushFeedCapsAndSkipsZero(t *testing.T) {
	var feed []scoringEvent
	for i := 1; i <= 25; i++ {
		feed, _ = pushFeed(feed, []scoringEvent{{GameID: "1", Points: 3, HomeScore: i}})
	}
	if len(feed) != 20 || feed[0].HomeScore != 25 || feed[19].HomeScore != 6 {
		t.Fatalf("cap/order wrong: len %d first %d last %d", len(feed), feed[0].HomeScore, feed[19].HomeScore)
	}
	if got, added := pushFeed(nil, []scoringEvent{{GameID: "1", Points: 0, Final: true}}); len(got) != 0 || added != 0 {
		t.Fatalf("plain finals must not enter the feed: %+v", got)
	}
}

// runCmds executes a Bubble Tea command tree and returns the messages it produces.
func runCmds(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, runCmds(c)...)
		}
		return out
	}
	return []tea.Msg{msg}
}

func feedModel(t *testing.T, summaryJSON string, requests *int32) model {
	t.Helper()
	api := &client{http: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if strings.HasSuffix(req.URL.Path, "/summary") {
			atomic.AddInt32(requests, 1)
		}
		return &http.Response{StatusCode: 200, Body: http.NoBody, Header: make(http.Header)}, nil
	})}}
	_ = summaryJSON
	return model{api: api, season: 2026, week: 9, seasonType: 2, favorite: "GB", width: 100, height: 30, board: boardAt(t, "1", "in", "19", "20")}
}

func TestFeedRetriesStopAfterTwo(t *testing.T) {
	var requests int32
	m := feedModel(t, "", &requests)
	next, cmd := m.Update(boardMsg{board: boardAt(t, "1", "in", "19", "27"), season: 2026, week: 9, seasonType: 2})
	m = next.(model)
	if len(m.feed) != 1 || m.feed[0].PlayText != "" {
		t.Fatalf("feed after score: %+v", m.feed)
	}
	msgs := runCmds(cmd)
	fetches := 0
	for _, msg := range msgs {
		if fm, ok := msg.(feedDetailMsg); ok {
			fetches++
			next, _ = m.Update(feedDetailMsg{id: fm.id, detail: gameDetail{}})
			m = next.(model)
		}
	}
	if fetches != 1 {
		t.Fatalf("expected one summary fetch after a score, got %d", fetches)
	}
	for i := 0; i < 3; i++ {
		next, cmd = m.Update(boardMsg{board: boardAt(t, "1", "in", "19", "27"), season: 2026, week: 9, seasonType: 2})
		m = next.(model)
		for _, msg := range runCmds(cmd) {
			if fm, ok := msg.(feedDetailMsg); ok {
				fetches++
				next, _ = m.Update(feedDetailMsg{id: fm.id, detail: gameDetail{}})
				m = next.(model)
			}
		}
	}
	if fetches != 3 {
		t.Fatalf("expected initial fetch plus two retries, got %d", fetches)
	}
	if m.feed[0].PlayText != "" {
		t.Fatalf("unmatched event must stay a diff entry: %+v", m.feed[0])
	}
}

func TestFeedEnrichedByFeedDetailMsg(t *testing.T) {
	var requests int32
	m := feedModel(t, "", &requests)
	next, _ := m.Update(boardMsg{board: boardAt(t, "1", "in", "19", "27"), season: 2026, week: 9, seasonType: 2})
	m = next.(model)
	next, _ = m.Update(feedDetailMsg{id: "1", detail: scoringDetail(t)})
	m = next.(model)
	if m.feed[0].PlayType != "+7" || !strings.HasPrefix(m.feed[0].PlayText, "J.Love") {
		t.Fatalf("not enriched: %+v", m.feed[0])
	}
}

func TestDetailMsgEnrichesOpenGameWithoutExtraFetch(t *testing.T) {
	var requests int32
	m := feedModel(t, "", &requests)
	m.showDetail, m.detailID, m.detailGame = true, "1", m.board.Events[0]
	next, cmd := m.Update(boardMsg{board: boardAt(t, "1", "in", "19", "27"), season: 2026, week: 9, seasonType: 2})
	m = next.(model)
	feedFetches := 0
	for _, msg := range runCmds(cmd) {
		if _, ok := msg.(feedDetailMsg); ok {
			feedFetches++
		}
	}
	if feedFetches != 0 {
		t.Fatalf("open game must reuse its summary refresh, got %d feed fetches", feedFetches)
	}
	next, _ = m.Update(detailMsg{id: "1", detail: scoringDetail(t)})
	m = next.(model)
	if !strings.HasPrefix(m.feed[0].PlayText, "J.Love") {
		t.Fatalf("detailMsg did not enrich the feed: %+v", m.feed[0])
	}
}

func TestWeekChangeClearsFeed(t *testing.T) {
	var requests int32
	m := feedModel(t, "", &requests)
	m.feed = []scoringEvent{{GameID: "1", Points: 3}}
	m.feedPending = map[string]int{"1": 1}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	m = next.(model)
	if m.feed != nil || m.feedPending != nil {
		t.Fatalf("week change must clear the feed: %+v %+v", m.feed, m.feedPending)
	}
}

func sampleFeed(n int) []scoringEvent {
	var feed []scoringEvent
	for i := 0; i < n; i++ {
		feed = append(feed, scoringEvent{GameID: "1", Team: "GB", Points: 3, Away: "CAR", Home: "GB", AwayScore: 19, HomeScore: 20 + i, Period: 4, Clock: fmt.Sprintf("%d:%02d", 14-i, 7*i%60), Favorite: i%2 == 0})
	}
	return feed
}

func TestWideDashboardFeedPanel(t *testing.T) {
	m := liveBoardModel(112, 34, false)
	m.favorite = "GB"
	m.feed = sampleFeed(8)
	view := m.View()
	if !strings.Contains(view, "SCORING FEED") || strings.Contains(view, "QUICK KEYS") {
		t.Fatalf("feed panel missing or quick keys still present:\n%s", view)
	}
	for i := 0; i < 6; i++ {
		if !strings.Contains(view, m.feed[i].Clock) {
			t.Fatalf("entry %d (%s) missing:\n%s", i, m.feed[i].Clock, view)
		}
	}
	if strings.Contains(view, m.feed[6].Clock) {
		t.Fatalf("more than 6 entries shown:\n%s", view)
	}
	if n := strings.Count(view, "\n") + 1; n > 34 {
		t.Fatalf("wide dashboard overflowed: %d lines", n)
	}
	m.feed = nil
	if view := m.View(); !strings.Contains(view, "No scores yet this session") {
		t.Fatalf("empty feed state missing:\n%s", view)
	}
}

func TestDashboardFitsWithLiveFavoriteAndFeed(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {100, 30}, {112, 34}, {150, 36}} {
		m := liveBoardModel(size[0], size[1], false)
		m.favorite = "GB"
		m.board.Events[0].Competitions[0].Situation.LastPlay.Text = strings.Repeat("word ", 80)
		m.feed = sampleFeed(8)
		view := m.View()
		if n := strings.Count(view, "\n") + 1; n > size[1] {
			t.Fatalf("%dx%d overflowed: %d lines\n%s", size[0], size[1], n, view)
		}
		if !strings.Contains(view, "›") {
			t.Fatalf("%dx%d lost the game list:\n%s", size[0], size[1], view)
		}
	}
}

func TestNarrowDashboardFeedLine(t *testing.T) {
	m := liveBoardModel(90, 24, false)
	m.feed = sampleFeed(2)
	view := m.View()
	if !strings.Contains(view, "⚡ "+m.feed[0].feedLine()[:12]) || strings.Contains(view, m.feed[1].Clock) {
		t.Fatalf("narrow feed line wrong:\n%s", view)
	}
	if n := strings.Count(view, "\n") + 1; n > 24 {
		t.Fatalf("narrow dashboard overflowed: %d lines", n)
	}
	m.feed = nil
	if view := m.View(); strings.Contains(view, "⚡") {
		t.Fatalf("empty feed must not draw a line:\n%s", view)
	}
}

func countFeedFetches(cmd tea.Cmd) int {
	n := 0
	for _, msg := range runCmds(cmd) {
		if _, ok := msg.(feedDetailMsg); ok {
			n++
		}
	}
	return n
}

func TestFullFeedStillFetchesNewScores(t *testing.T) {
	var requests int32
	m := feedModel(t, "", &requests)
	m.feed = sampleFeed(20)
	for i := range m.feed {
		m.feed[i].PlayText = "done"
	}
	next, cmd := m.Update(boardMsg{board: boardAt(t, "1", "in", "19", "27"), season: 2026, week: 9, seasonType: 2})
	m = next.(model)
	if len(m.feed) != 20 || m.feed[0].HomeScore != 27 || m.feed[0].PlayText != "" {
		t.Fatalf("new score not at the head of a full feed: %+v", m.feed[0])
	}
	if got := countFeedFetches(cmd); got != 1 {
		t.Fatalf("full feed must still fetch the new score's play, got %d fetches", got)
	}
}

func TestExhaustedGameFetchesAgainOnNewScore(t *testing.T) {
	var requests int32
	m := feedModel(t, "", &requests)
	next, cmd := m.Update(boardMsg{board: boardAt(t, "1", "in", "19", "26"), season: 2026, week: 9, seasonType: 2})
	m = next.(model)
	total := countFeedFetches(cmd)
	for i := 0; i < 3; i++ {
		next, cmd = m.Update(boardMsg{board: boardAt(t, "1", "in", "19", "26"), season: 2026, week: 9, seasonType: 2})
		m = next.(model)
		total += countFeedFetches(cmd)
	}
	if total != 3 {
		t.Fatalf("setup: expected 3 fetches before exhaustion, got %d", total)
	}
	next, cmd = m.Update(boardMsg{board: boardAt(t, "1", "in", "19", "27"), season: 2026, week: 9, seasonType: 2})
	m = next.(model)
	if got := countFeedFetches(cmd); got != 1 {
		t.Fatalf("a new score after exhaustion must fetch again, got %d", got)
	}
}

func TestEnrichEventTakesScorerFromPlay(t *testing.T) {
	api := jsonClient(t, func(*http.Request) {}, `{"scoringPlays":[{"id":"a","text":"E.Pineiro 40 yd Field Goal","awayScore":22,"homeScore":27,"scoringType":{"abbreviation":"FG"},"team":{"abbreviation":"CAR"}}]}`)
	detail, err := api.summary(context.Background(), "1")
	if err != nil {
		t.Fatal(err)
	}
	got, ok := enrichEvent(scoringEvent{GameID: "1", Team: "GB", Points: 7, AwayScore: 22, HomeScore: 27}, detail)
	if !ok || got.Team != "CAR" || got.PlayType != "FG" {
		t.Fatalf("scorer should follow the matched play: %+v", got)
	}
}
