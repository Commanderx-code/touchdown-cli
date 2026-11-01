package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// feedLimit is how many scoring events the dashboard keeps for the session.
const feedLimit = 20

// feedRetries is how many extra summary fetches a scoring event gets when
// ESPN has not yet published its scoring play.
const feedRetries = 2

// scoringEvent is one change to a game's score, or a new final, noticed
// between two scoreboard refreshes.
type scoringEvent struct {
	GameID    string
	Team      string
	Points    int
	AwayScore int
	HomeScore int
	Away      string
	Home      string
	Period    int
	Clock     string
	Favorite  bool
	Final     bool
	PlayType  string
	PlayText  string
}

type feedDetailMsg struct {
	id     string
	detail gameDetail
	err    error
}

// scoringEvents compares two boards and returns every score change and
// every game that just went final. Games that were not yet underway on the
// previous board are skipped, so a first load produces nothing.
func scoringEvents(previous, current scoreboard, favorite string) []scoringEvent {
	byID := make(map[string]game, len(previous.Events))
	for _, g := range previous.Events {
		if g.ID != "" {
			byID[g.ID] = g
		}
	}
	var events []scoringEvent
	for _, g := range current.Events {
		before, ok := byID[g.ID]
		if !ok || g.ID == "" || g.Status.Type.State == "pre" || before.Status.Type.State == "pre" || before.Status.Type.State == "post" {
			continue
		}
		away, home := g.sides()
		oldAway, oldHome := before.sides()
		awayNow, homeNow := scoreInt(away.Score), scoreInt(home.Score)
		awayRise, homeRise := awayNow-scoreInt(oldAway.Score), homeNow-scoreInt(oldHome.Score)
		final := g.Status.Type.State == "post"
		if awayRise <= 0 && homeRise <= 0 && !final {
			continue
		}
		e := scoringEvent{
			GameID: g.ID, Away: away.Team.Abbreviation, Home: home.Team.Abbreviation,
			AwayScore: awayNow, HomeScore: homeNow,
			Period: g.Status.Period, Clock: g.Status.DisplayClock, Final: final,
			Favorite: favorite != "" && (favorite == away.Team.Abbreviation || favorite == home.Team.Abbreviation),
		}
		switch {
		case awayRise > homeRise && awayRise > 0:
			e.Team, e.Points = away.Team.Abbreviation, awayRise
		case homeRise > 0:
			e.Team, e.Points = home.Team.Abbreviation, homeRise
		}
		events = append(events, e)
	}
	return events
}

func scoreInt(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// feedLine renders the event for the scoring feed. Scores read away – home
// with each abbreviation beside its score; play text, when known, comes
// last so clipping never hides the score.
func (e scoringEvent) feedLine() string {
	var b strings.Builder
	if e.Favorite {
		b.WriteString("★ ")
	}
	when := strings.TrimSpace(periodLabel(e.Period) + " " + e.Clock)
	if when != "" {
		b.WriteString(when + "  ")
	}
	label := fmt.Sprintf("+%d", e.Points)
	if e.PlayType != "" {
		label = e.PlayType
	}
	fmt.Fprintf(&b, "%s %s  %s %d – %d %s", e.Team, label, e.Away, e.AwayScore, e.HomeScore, e.Home)
	if e.PlayText != "" {
		b.WriteString("  " + e.PlayText)
	}
	return plainText(b.String())
}

// enrichEvent fills the play type and text from the game summary's scoring
// play whose resulting score matches the event. It reports false when no
// play matches yet.
func enrichEvent(e scoringEvent, detail gameDetail) (scoringEvent, bool) {
	for _, play := range detail.ScoringPlays {
		if play.AwayScore != e.AwayScore || play.HomeScore != e.HomeScore {
			continue
		}
		e.PlayType = play.ScoringType.Abbreviation
		if e.PlayType == "" {
			e.PlayType = fmt.Sprintf("+%d", e.Points)
		}
		e.PlayText = strings.Join(strings.Fields(play.Text), " ")
		return e, true
	}
	return e, false
}

// pushFeed prepends new scoring events, drops plain finals, and keeps the
// newest feedLimit entries.
func pushFeed(feed []scoringEvent, events []scoringEvent) []scoringEvent {
	for _, e := range events {
		if e.Points == 0 {
			continue
		}
		feed = append([]scoringEvent{e}, feed...)
	}
	if len(feed) > feedLimit {
		feed = feed[:feedLimit]
	}
	return feed
}

func (m model) loadFeedDetail(id string) tea.Cmd {
	return func() tea.Msg {
		detail, err := m.api.summary(context.Background(), id)
		return feedDetailMsg{id, detail, err}
	}
}

// recordScoring adds the events to the feed and returns the summary fetches
// needed to enrich them: one per newly scoring game, plus retries for games
// still waiting on a matching play. A game open in the game center is
// skipped because its own refresh delivers the same summary.
func (m model) recordScoring(events []scoringEvent) (model, []tea.Cmd) {
	before := len(m.feed)
	m.feed = pushFeed(m.feed, events)
	if m.feedPending == nil {
		m.feedPending = map[string]int{}
	}
	for _, e := range m.feed[:len(m.feed)-min(before, len(m.feed))] {
		if _, pending := m.feedPending[e.GameID]; !pending {
			m.feedPending[e.GameID] = -1
		}
	}
	var cmds []tea.Cmd
	for id, retries := range m.feedPending {
		if !m.feedWaiting(id) {
			delete(m.feedPending, id)
			continue
		}
		if retries >= feedRetries {
			continue
		}
		m.feedPending[id] = retries + 1
		if m.showDetail && m.detailID == id {
			continue
		}
		cmds = append(cmds, m.loadFeedDetail(id))
	}
	return m, cmds
}

// feedWaiting reports whether any entry for the game still lacks play text.
func (m model) feedWaiting(id string) bool {
	for _, e := range m.feed {
		if e.GameID == id && e.PlayText == "" {
			return true
		}
	}
	return false
}

// enrichFeed applies a game summary to every waiting entry for that game.
func (m model) enrichFeed(id string, detail gameDetail) model {
	for i, e := range m.feed {
		if e.GameID != id || e.PlayText != "" {
			continue
		}
		if enriched, ok := enrichEvent(e, detail); ok {
			m.feed[i] = enriched
		}
	}
	if !m.feedWaiting(id) {
		delete(m.feedPending, id)
	}
	return m
}
