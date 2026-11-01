package main

import (
	"context"
	"encoding/json"
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
		feed = pushFeed(feed, []scoringEvent{{GameID: "1", Points: 3, HomeScore: i}})
	}
	if len(feed) != 20 || feed[0].HomeScore != 25 || feed[19].HomeScore != 6 {
		t.Fatalf("cap/order wrong: len %d first %d last %d", len(feed), feed[0].HomeScore, feed[19].HomeScore)
	}
	if got := pushFeed(nil, []scoringEvent{{GameID: "1", Points: 0, Final: true}}); len(got) != 0 {
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
