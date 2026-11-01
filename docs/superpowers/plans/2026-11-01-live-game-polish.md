# Live-Game Polish Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show each live game's situation, red zone and timeouts on the dashboard, spotlight and game center, and add a session scoring feed with play text.

**Architecture:** Pure formatting functions over the existing `game` type (`situationText`, `timeoutsText`) feed the three views. Score detection generalises `scoreAlerts` into `scoringEvents`; the model keeps a small feed slice, enriches each event with one summary fetch via a new `feedDetailMsg`, and renders it as a panel (wide) or one line (narrow). No new endpoints, no new dependencies.

**Tech Stack:** Go 1.24, Bubble Tea, Lip Gloss.

**Spec:** `docs/superpowers/specs/2026-11-01-live-game-polish-design.md`. Terms per `GLOSSARY.md`.

## Global Constraints

- No new modules. Existing tests keep passing; new rendering tests build a `model`, call `View()`, assert substrings and that line count fits `m.height`.
- Dashboard rows stay two lines each. Feed costs one summary request per scoring event, never per game per poll.
- No change to `-status`, `-watch`, `-split`, alert text, or key bindings.
- Timeouts never appear in dashboard rows or the compact layout.
- Feed holds 20 newest events, clears on week change, empty on launch.
- Commits carry no AI attribution lines.

## Review Focus

- A live row for a game with possession but `down == 0` (kickoff) must read `GB ball`, not `GB ball  0th & 0`. Test in Task 2.
- A game that goes from `in` to `post` with a simultaneous score change (walk-off FG) must yield one feed entry and one alert, not two. Test in Task 5.
- A summary whose scoring plays never match (ESPN relabels a score after review) must stop retrying after two refreshes and keep the diff entry. Test in Task 5.
- ESPN sends `homeTimeouts`/`awayTimeouts` only during live games; a final with the fields absent must render no timeouts line and no glyphs for zero. Test in Task 2.
- At 112x34 (smallest wide layout) the right column with the feed panel added must not overflow the terminal height. Test in Task 6.

---

### Task 1: ESPN situation and scoring-type fields

**Files:**
- Modify: `espn.go` (`gameSituation`, `play`)
- Test: `live_test.go` (create)

**Interfaces:**
- Produces on `gameSituation`: `DownDistanceText string` (json `downDistanceText`), `PossessionText string` (json `possessionText`), `HomeTimeouts *int` (json `homeTimeouts`), `AwayTimeouts *int` (json `awayTimeouts`). Pointers so "absent" differs from 0.
- Produces on `play`: `ScoringType struct { Abbreviation string `json:"abbreviation"` }` (json `scoringType`).

- [ ] **Step 1: Write the failing test**

```go
const liveSituationJSON = `{"week":{"number":9},"events":[{"id":"1","date":"2026-11-01T21:25Z","status":{"type":{"state":"in","shortDetail":"2:14 - 4th","description":"In Progress"},"displayClock":"2:14","period":4},"competitions":[{"venue":{"fullName":"Lambeau Field"},"situation":{"possession":"8","down":3,"distance":7,"isRedZone":true,"downDistanceText":"3rd & 7 at CAR 22","possessionText":"GB 22","homeTimeouts":3,"awayTimeouts":2,"lastPlay":{"text":"J.Love pass short right to C.Watson for 9 yards"}},"competitors":[{"homeAway":"home","score":"27","team":{"id":"8","abbreviation":"GB","name":"Packers"}},{"homeAway":"away","score":"19","team":{"id":"29","abbreviation":"CAR","name":"Panthers"}}]}]}]}`

func TestScoreboardParsesSituationFields(t *testing.T) {
	api := jsonClient(t, func(*http.Request) {}, liveSituationJSON)
	board, err := api.scoreboard(context.Background(), 0, 0, 2)
	if err != nil { t.Fatal(err) }
	s := board.Events[0].Competitions[0].Situation
	if s.DownDistanceText != "3rd & 7 at CAR 22" || s.PossessionText != "GB 22" || s.HomeTimeouts == nil || *s.HomeTimeouts != 3 || s.AwayTimeouts == nil || *s.AwayTimeouts != 2 {
		t.Fatalf("situation not parsed: %+v", s)
	}
}

func TestSummaryParsesScoringType(t *testing.T) {
	api := jsonClient(t, func(*http.Request) {}, `{"scoringPlays":[{"id":"p1","text":"J.Love pass to C.Watson for 12 yards, TOUCHDOWN","awayScore":19,"homeScore":27,"scoringType":{"abbreviation":"TD"},"team":{"abbreviation":"GB"},"period":{"number":4},"clock":{"displayValue":"2:14"}}]}`)
	detail, err := api.summary(context.Background(), "1")
	if err != nil { t.Fatal(err) }
	if detail.ScoringPlays[0].ScoringType.Abbreviation != "TD" { t.Fatalf("%+v", detail.ScoringPlays[0]) }
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test -run 'TestScoreboardParsesSituationFields|TestSummaryParsesScoringType' ./...`
Expected: compile error, `s.DownDistanceText undefined`

- [ ] **Step 3: Add the fields**

- [ ] **Step 4: Run the suite**

Run: `go test ./...` Expected: `ok`

- [ ] **Step 5: Commit**

```bash
git add espn.go live_test.go
git commit -m "Parse down-and-distance text, timeouts and scoring type from ESPN"
```

---

### Task 2: Situation and timeouts text

**Files:**
- Modify: `live.go` (replace `liveContextLines` body), `status.go` (`liveLabel` unchanged)
- Test: `live_test.go`

**Interfaces:**
- Produces:
  - `func situationText(g game) string`: `""` unless state is `in` and `Situation.Possession` matches a competitor's `Team.ID`. Otherwise `<ABBR> ball`, then two spaces and `DownDistanceText` when non-empty, else `3rd & 7` built from `Down`/`Distance` (`& GOAL` when `Distance == 0`) when `Down > 0`. Append `  ·  RED ZONE` when `IsRedZone`.
  - `func timeoutsText(g game) string`: `""` unless both `AwayTimeouts` and `HomeTimeouts` are non-nil; else `CAR TO ●●○   GB TO ●●●` (away first), three glyphs each, `●` × remaining then `○` × (3 − remaining), clamped to 0..3.
  - `func latestPlayText(g game) string`: `Situation.LastPlay.Text` with whitespace collapsed, `""` when absent.
  - `(m model) liveContextLines()` now returns, in order and omitting empties: `situationText`, `timeoutsText`, `"LATEST  " + latestPlayText`. Game center and compact view keep calling it.

- [ ] **Step 1: Write failing tests**

```go
func liveGame(possession string, down, distance int, ddText string, rz bool, away, home *int, last string) game {
	// builds a state "in" game GB (id 8, home) vs CAR (id 29, away) with the given situation
}

func TestSituationText(t *testing.T) {
	three := 3; two := 2
	cases := []struct{ name string; g game; want string }{
		{"espn text", liveGame("8", 3, 7, "3rd & 7 at CAR 22", false, &two, &three, ""), "GB ball  3rd & 7 at CAR 22"},
		{"built text", liveGame("8", 3, 7, "", false, nil, nil, ""), "GB ball  3rd & 7"},
		{"goal line", liveGame("29", 1, 0, "", true, nil, nil, ""), "CAR ball  1st & GOAL  ·  RED ZONE"},
		{"kickoff no down", liveGame("8", 0, 0, "", false, nil, nil, ""), "GB ball"},
		{"no possession", liveGame("", 0, 0, "", false, nil, nil, ""), ""},
	}
	// also: a "post" game with possession set returns ""
}

func TestTimeoutsText(t *testing.T) {
	three := 3; two := 2; zero := 0
	// (&two, &three) → "CAR TO ●●○   GB TO ●●●"; (&zero, &three) → "CAR TO ○○○   GB TO ●●●"; (nil, &three) → ""; both nil → ""
}

func TestLiveContextLinesOrder(t *testing.T) {
	// model with detailGame = liveGame("8",3,7,"3rd & 7 at CAR 22",true,&two,&three,"J.Love  pass   to C.Watson")
	// want []string{"GB ball  3rd & 7 at CAR 22  ·  RED ZONE", "CAR TO ●●○   GB TO ●●●", "LATEST  J.Love pass to C.Watson"}
}
```

- [ ] **Step 2: Run to verify failure** — `go test -run 'TestSituationText|TestTimeoutsText|TestLiveContextLinesOrder' ./...` Expected: `undefined: situationText`

- [ ] **Step 3: Implement in `live.go`; remove the old `BALL`/`&` assembly, keep `downLabel`**

Note: `ticker.go`'s `liveSuffix` keeps its own lowercase `3rd & 7` format and is unchanged.

- [ ] **Step 4: Run the suite** — `go test ./...` Expected: `ok`. `TestDetailFitsTerminalAndScrolls` and friends may need their expected substrings updated from `BALL GB` to `GB ball`; update the expectation, not the code.

- [ ] **Step 5: Commit**

```bash
git add live.go live_test.go main_test.go
git commit -m "Render the live situation and timeouts from the scoreboard"
```

---

### Task 3: Dashboard and compact rows

**Files:**
- Modify: `ui.go` (`scoreboardPanel` badge and style, `compactView` rows)
- Test: `live_test.go`

**Interfaces:**
- Consumes: `situationText`.
- Behaviour: in `scoreboardPanel`, for state `in` the badge is `● LIVE  <STATUS>` + (`  ·  ` + `situationText(g)` when non-empty); the venue is appended only when state is not `in`. When `IsRedZone`, the badge line renders with `liveStyle` even when the row is selected (line one keeps `selectedStyle`). In `compactView`, for state `in` and `width >= 60`, append `  ` + `situationText(g)` to the row.

- [ ] **Step 1: Write failing tests**

```go
func TestDashboardLiveRowShowsSituationNotVenue(t *testing.T) {
	// model{board: board with liveGame(... "3rd & 7 at CAR 22", rz=true ...) and Venue "Lambeau Field", width: 100, height: 30, season: 2026, week: 9}
	// view := m.View(); require Contains "GB ball  3rd & 7 at CAR 22", Contains "RED ZONE", NotContains "Lambeau Field"
	// same board with the game state "post": Contains "Lambeau Field"
}

func TestCompactRowSituationNeedsSixtyColumns(t *testing.T) {
	// compact model width 60: Contains "GB ball"; width 59: NotContains "GB ball"; neither contains "TO ●"
}
```

- [ ] **Step 2: Run to verify failure** — Expected: FAIL on `Contains "GB ball"`

- [ ] **Step 3: Implement**

- [ ] **Step 4: Run the suite** — `go test ./...` Expected: `ok`

- [ ] **Step 5: Commit**

```bash
git add ui.go live_test.go
git commit -m "Show the live situation on dashboard and compact rows"
```

---

### Task 4: Spotlight panel situation, timeouts and latest play

**Files:**
- Modify: `dashboard.go` (`spotlightPanel`)
- Test: `live_test.go`

**Interfaces:**
- Consumes: `situationText`, `timeoutsText`, `latestPlayText`, `wrapWords` (`ui.go`).
- Behaviour: when the favorite's game (`favoriteMatch`) is state `in`, after the existing status line append `situationText` (liveStyle), `timeoutsText` (dimStyle), then `wrapWords(latestPlayText, inner)` capped at 2 lines (dimStyle), each omitted when empty.

- [ ] **Step 1: Write failing test**

```go
func TestSpotlightShowsLiveSituation(t *testing.T) {
	// wide model (width 120, height 40) favorite GB with the live game; view Contains "GB ball  3rd & 7 at CAR 22", "CAR TO ●●○   GB TO ●●●", "J.Love pass"
	// same with a 400-char LastPlay.Text: count lines containing fragments of it ≤ 2
	// height check: strings.Count(view, "\n")+1 <= 40
}
```

- [ ] **Step 2: Run to verify failure** — Expected: FAIL on `Contains "GB ball"`

- [ ] **Step 3: Implement**

- [ ] **Step 4: Run the suite** — Expected: `ok`

- [ ] **Step 5: Commit**

```bash
git add dashboard.go live_test.go
git commit -m "Show situation, timeouts and the latest play in the spotlight panel"
```

---

### Task 5: Scoring events, alerts adapter and feed state

**Files:**
- Create: `feed.go`
- Modify: `alerts.go` (`scoreAlerts` becomes an adapter), `main.go` (model fields, `boardMsg`, week change, new msg)
- Test: `feed_test.go` (create)

**Interfaces:**
- Produces in `feed.go`:
  - `type scoringEvent struct { GameID, Team string; Points int; AwayScore, HomeScore int; Away, Home string; Period int; Clock string; Favorite bool; Final bool; PlayType, PlayText string; Retries int }`
  - `func scoringEvents(previous, current scoreboard, favorite string) []scoringEvent`: for each game present in both boards with state not `pre` and previous state not `post`: a score change yields one event (`Team` = the side whose score rose; if both rose, the side with the larger rise; `Points` = that rise), `Final` true when state is now `post`. A new final with no score change yields one event with `Points == 0` and `Final == true`. `Period`/`Clock` from `current.Status`.
  - `func (e scoringEvent) feedLine() string`: `★ ` prefix when `Favorite`; then `Q4 2:14  GB +7  27-19 CAR` where the scorer is listed first with its score, or with `PlayText` set: `Q4 2:14  GB TD  <PlayText>  27-19`. `periodLabel` from `status.go`.
  - `func enrichEvent(e scoringEvent, detail gameDetail) (scoringEvent, bool)`: finds a `ScoringPlays` entry with `AwayScore == e.AwayScore && HomeScore == e.HomeScore`; sets `PlayType` (`ScoringType.Abbreviation`, else `fmt.Sprintf("+%d", e.Points)`) and `PlayText`; returns false when none match.
  - `func pushFeed(feed []scoringEvent, events []scoringEvent) []scoringEvent`: prepends, keeps 20, skips `Points == 0` events.
- `alerts.go`: `scoreAlerts(previous, current scoreboard, favorite string, all bool) []scoreAlert` now maps `scoringEvents` through the existing filter and text so `features_test.go`/`main_test.go` alert tests still pass unchanged.
- `main.go`:
  - model fields `feed []scoringEvent`, `feedPending map[string]int` (game ID → retries used).
  - `type feedDetailMsg struct { id string; detail gameDetail; err error }`; `func (m model) loadFeedDetail(id string) tea.Cmd` like `loadDetail` but returning `feedDetailMsg`.
  - In `boardMsg`: compute `events := scoringEvents(m.board, msg.board, m.favorite)` under the same guard as alerts today but without `m.notify` (`msg.board.CachedAt.IsZero() && m.board.CachedAt.IsZero() && len(m.board.Events) > 0`); alerts derive from the same `events`. `m.feed = pushFeed(m.feed, events)`; for each pushed event's game ID issue `loadFeedDetail` once; for every game ID in `feedPending` with retries < 2 whose newest entry still has empty `PlayText`, issue it again and increment.
  - In `feedDetailMsg`: for each feed entry with that `GameID` and empty `PlayText`, apply `enrichEvent`; on success delete the pending counter.
  - Where `m.week += delta` runs: `m.feed = nil; m.feedPending = nil`.

- [ ] **Step 1: Write failing tests**

```go
func TestScoringEventsDetectsChangesAndFinals(t *testing.T) {
	// prev: GB 20 CAR 19 in; cur: GB 27 CAR 19 in → one event Team GB Points 7 Final false
	// prev: GB 27 CAR 19 in; cur: GB 27 CAR 22 post → one event Team CAR Points 3 Final true (walk-off: one event, not two)
	// prev: GB 27 CAR 19 in; cur same scores post → one event Points 0 Final true
	// prev: pre; cur: in 7-0 → no event
	// favorite "GB" sets Favorite true on GB games only
}

func TestScoreAlertsUnchangedThroughEvents(t *testing.T) {
	// same inputs as the existing alert tests: body "GB 27 — 19 CAR", title "Touchdown · Score update" / "Touchdown · Final"
}

func TestFeedLineFormats(t *testing.T) {
	// diff: "Q4 2:14  GB +7  27-19 CAR"; favorite: "★ Q4 2:14  GB +7  27-19 CAR"; enriched: "Q4 2:14  GB TD  J.Love pass to C.Watson for 12 yards, TOUCHDOWN  27-19"
}

func TestEnrichEventMatchesScores(t *testing.T) {
	// detail with plays (19,20) and (19,27): event (19,27) → PlayType "TD", ok; event (19,24) → !ok
	// play without scoringType → PlayType "+7"
}

func TestPushFeedCapsAndSkipsZero(t *testing.T) {
	// 25 events → len 20, newest first; Points 0 skipped
}

func TestFeedRetriesStopAfterTwo(t *testing.T) {
	// drive model.Update with boardMsg (score change) then feedDetailMsg with no matching play, then two more boardMsg with no changes:
	// count loadFeedDetail cmds issued == 3 (initial + 2 retries); entry PlayText still ""
	// (collect cmds by executing the returned tea.Cmd against a jsonClient that records summary requests)
}

func TestWeekChangeClearsFeed(t *testing.T) {
	// model with feed of 1, send tea.KeyMsg{Type: tea.KeyRight} on the dashboard → m.feed == nil
}
```

- [ ] **Step 2: Run to verify failure** — Expected: `undefined: scoringEvents`

- [ ] **Step 3: Implement `feed.go`, adapt `alerts.go`, wire `main.go`**

- [ ] **Step 4: Run the suite** — Expected: `ok`, including the pre-existing alert tests untouched.

- [ ] **Step 5: Commit**

```bash
git add feed.go alerts.go main.go feed_test.go
git commit -m "Detect scoring events once for alerts and a session scoring feed"
```

---

### Task 6: Feed rendering

**Files:**
- Modify: `dashboard.go` (`feedPanel`, `wideDashboard`, `narrowDashboard`; delete `shortcutsPanel`)
- Test: `feed_test.go`

**Interfaces:**
- Consumes: `m.feed`, `scoringEvent.feedLine`.
- Produces: `func (m model) feedPanel(inner int) string`: header `SCORING FEED` (sectionStyle) + rule, then up to 6 `feedLine()`s clipped to `inner`, favorite entries in accentStyle, others textStyle; empty state `No scores yet this session` (dimStyle). `wideDashboard` right column becomes spotlight, feed, snapshot. `narrowDashboard` inserts `⚡ ` + newest `feedLine()` (clipped, liveStyle) between the summary and the spotlight when the feed is non-empty, and reduces `rows` by one in that case.

- [ ] **Step 1: Write failing tests**

```go
func TestWideDashboardFeedPanel(t *testing.T) {
	// width 112, height 34, feed of 8 entries → view Contains "SCORING FEED", contains entries 1..6 clocks, NotContains entry 7's clock, NotContains "QUICK KEYS"; strings.Count(view,"\n")+1 <= 34
	// empty feed → Contains "No scores yet this session"
}

func TestNarrowDashboardFeedLine(t *testing.T) {
	// width 90, height 24, feed of 2 → Contains "⚡ " + newest.feedLine() prefix, NotContains the older clock; height fits
	// empty feed → NotContains "⚡"
}
```

- [ ] **Step 2: Run to verify failure** — Expected: FAIL on `Contains "SCORING FEED"`

- [ ] **Step 3: Implement**

- [ ] **Step 4: Run the suite** — Expected: `ok`. `TestDashboardFitsTerminalSizes` must still pass at every size it checks.

- [ ] **Step 5: Commit**

```bash
git add dashboard.go feed_test.go
git commit -m "Render the scoring feed on the wide and narrow dashboards"
```

---

### Task 7: Help legend and README

**Files:**
- Modify: `help.go`, `README.md`
- Test: `live_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestHelpLegendMentionsRedZoneAndTimeouts(t *testing.T) {
	m := model{showHelp: true, width: 100, height: 30}
	view := m.View()
	if !strings.Contains(view, "RED ZONE") || !strings.Contains(view, "TO ●●○") { t.Fatalf("legend missing:\n%s", view) }
}
```

- [ ] **Step 2: Run to verify failure** — Expected: FAIL `legend missing`

- [ ] **Step 3: Add two lines under READING SCORES in the full help**: `RED ZONE  offense inside the 20  ·  TO ●●○  timeouts remaining`, split into two `textStyle` lines. Short help unchanged.

- [ ] **Step 4: README**: in Features, extend the "Scores dashboard" bullet with `live rows show possession, down and distance, and red zone`; add a bullet `**Scoring feed:** Every score across live games appears on the dashboard with the play that produced it.`; in "Game center" mention timeouts. In "Data and refresh behavior" add one sentence: `Each new score triggers one request for that game's summary to fetch the scoring play.`

- [ ] **Step 5: Run suite and vet** — `go test ./... && go vet ./...` Expected: `ok`

- [ ] **Step 6: Commit**

```bash
git add help.go README.md live_test.go
git commit -m "Document red zone, timeouts and the scoring feed"
```
