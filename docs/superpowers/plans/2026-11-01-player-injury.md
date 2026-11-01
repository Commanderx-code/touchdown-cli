# Player and Injury Data Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add rosters, injury reports and player cards to every season page, an injury report to the game center, and an injury count to the spotlight panel.

**Architecture:** Three new ESPN fetches (team injuries, team roster, athlete overview) land in session stores on the model keyed by team abbreviation or athlete ID, each entry carrying its own loading/error state. The season page gains a tab line (schedule, roster, injuries) with per-tab selection; Enter on a roster or injury row opens a full-screen player card. The game center's CENTER tab and the spotlight panel read the same injury store.

**Tech Stack:** Go 1.24, Bubble Tea v1, Lip Gloss v1; existing `getURL` cache; table tests with `jsonClient`/`roundTripFunc`, `key(m, name)` and `runCmds`.

**Spec:** `docs/superpowers/specs/2026-11-01-player-injury-design.md`

## Global Constraints

- Go 1.24, Bubble Tea, Lip Gloss, no new dependencies.
- Endpoints: `GET {apiBase}/injuries?team={abbr}` (lower-case), `GET {apiBase}/teams/{abbr}/roster`, `GET https://site.web.api.espn.com/apis/common/v3/sports/football/nfl/athletes/{id}/overview`; all through `client.getURL`.
- Every string from ESPN passes through `plainText` before rendering.
- Rosters, injury reports and player stats live in session stores keyed by team or player, never in the board or the favorite's schedule fields; a message for any team is stored, nothing is dropped.
- Existing tests keep passing; new screens tested through `model.Update` and `View()` substring and height assertions.
- Commits carry no AI attribution lines.

## Review Focus

- An injury entry whose `athlete.links` has no `/id/{n}/` href: the row renders and Enter does nothing. Test in Task 1.
- A roster athlete with missing `jersey`, `experience`, or `college`: `#--`, `Rookie`, blank college, no panic. Test in Task 1.
- Overview `statistics` with `categories` whose counts do not sum to `len(labels)`, or a split with fewer stats than labels: rows clip, never index out of range. Test in Task 1.
- Switching teams (opponent jump, picker) while a roster or injury request is in flight: the reply is stored under its own team and the new team's tab shows its own loading line. Test in Task 3.
- Game center for a game 30 days old: no injury section and no injury requests. Test in Task 5.

---

### Task 1: Data types and fetches

**Files:**
- Create: `injuries.go`, `roster.go`, `player.go`
- Test: `player_test.go` (new; holds compact JSON fixtures as consts)

**Interfaces:**
- Produces (injuries.go):
  - `type injury struct { AthleteID, Name, Position, Status, Code, BodyPart, ReturnDate, Comment string; Date time.Time }` where `Code` is `type.abbreviation` (`O`, `D`, `Q`, `IR`, `SUSP`), `ReturnDate` is `details.returnDate` (`2006-01-02`) kept as given.
  - `type injuryReport struct { Team string; Entries []injury; CachedAt time.Time }`
  - `func (c *client) injuries(ctx context.Context, team string) (injuryReport, error)`: drops `status == "Active"`, parses `AthleteID` from the first `athlete.links[].href` matching `/id/(\d+)/`, sorts by `injuryRank`, runs every string through `plainText`.
  - `func injuryRank(status string) int`: Out 0, Doubtful 1, Questionable 2, Injured Reserve 3, else 4.
  - `func statusLabel(i injury) string`: `OUT`, `D`, `Q`, `IR`, else `strings.ToUpper(i.Code)`.
  - `func (r injuryReport) counts() (out, doubtful, questionable, ir int)`.
  - `func (r injuryReport) summary() string`: `3 OUT · 1 Q · 2 IR` (zero counts omitted, order OUT, D, Q, IR); `""` when all zero.
  - `func (r injuryReport) gameDay() []injury`: Out, Doubtful and Questionable only, in report order.
  - `func (r injuryReport) find(athleteID string) (injury, bool)`.
  - `func returnLabel(date string) string`: `back Nov 25` from `2026-11-25`, `""` when empty or unparsable.
- Produces (roster.go):
  - `type rosterPlayer struct { ID, Name, Jersey, Position, Height, Weight, College, Status, Injury string; Age, Years int; HasYears bool }`
  - `type rosterGroup struct { Key, Title string; Players []rosterPlayer }` with titles `OFFENSE`, `DEFENSE`, `SPECIAL TEAMS`, `IR / OUT`, `PRACTICE SQUAD`, `SUSPENDED` for keys `offense`, `defense`, `specialTeam`, `injuredReserveOrOut`, `practiceSquad`, `suspended`; unknown keys title-cased from the key.
  - `type roster struct { Team string; Groups []rosterGroup; CachedAt time.Time }` with groups in the order above, empty groups omitted.
  - `func (c *client) roster(ctx context.Context, team string) (roster, error)`; `Jersey` `--` when empty, `Injury` = `injuries[0].status` or `""`, `HasYears` true when `experience.years` was present.
  - `func experienceLabel(p rosterPlayer) string`: `Rookie` when `HasYears && Years == 0`, `6 yrs` otherwise, `""` when `!HasYears`.
  - `func seasonLabelFor(p rosterPlayer) string`: `6th season` (ordinal), `Rookie` for 0, `""` when `!HasYears`.
- Produces (player.go):
  - `type statSection struct { Title, Split string; Labels, Values []string }`
  - `type playerStats struct { ID string; Sections []statSection; CachedAt time.Time }`
  - `func (c *client) playerOverview(ctx context.Context, id string) (playerStats, error)`: slices `statistics.labels` and the chosen split's `stats` by `categories[].count` into one section per category titled `strings.ToUpper(categories[].displayName)`; the split is `Regular Season` when present else the first split not named `Projected`, and `Split` is `""` for Regular Season else `strings.ToUpper(displayName)`. Short slices clip: a section never indexes past `len(labels)` or `len(stats)`. No statistics or no usable split → `Sections` empty, no error.
  - `const athleteBase = "https://site.web.api.espn.com/apis/common/v3/sports/football/nfl/athletes/"`.

- [ ] **Step 1: Write failing tests** in `player_test.go`

```go
const injuriesJSON = `{"injuries":[
 {"status":"Active","type":{"abbreviation":"A"},"athlete":{"displayName":"Josh Jacobs","position":{"abbreviation":"RB"},"links":[{"href":"https://www.espn.com/nfl/player/_/id/4047365/josh-jacobs"}]},"shortComment":"fine","date":"2026-10-31T14:09Z"},
 {"status":"Questionable","type":{"abbreviation":"Q"},"athlete":{"displayName":"Christian Watson","position":{"abbreviation":"WR"},"links":[{"href":"https://www.espn.com/nfl/player/_/id/4258173/christian-watson"}]},"details":{"type":"Ribs","returnDate":"2026-11-08"},"shortComment":"Watson said he suffered a broken rib.","date":"2026-10-30T04:06Z"},
 {"status":"Out","type":{"abbreviation":"O"},"athlete":{"displayName":"Micah Parsons","position":{"abbreviation":"DE"},"links":[{"href":"https://www.espn.com/nfl/player/_/id/4361423/micah-parsons"}]},"details":{"type":"Knee - ACL","returnDate":"2026-11-25"},"shortComment":"Targeting Week 12.\u0007","date":"2026-10-31T14:09Z"},
 {"status":"Injured Reserve","type":{"abbreviation":"IR"},"athlete":{"displayName":"No Link","position":{"abbreviation":"G"},"links":[]},"shortComment":"","date":"2026-10-29T22:54Z"}
]}`

func TestInjuriesParsesSortsAndDropsActive(t *testing.T) {
	// client.injuries(ctx, "GB") with jsonClient checking path "/injuries" and query team=gb
	// Entries: Parsons (OUT, 4361423, "Knee - ACL", "2026-11-25", comment without \u0007), Watson (Q), No Link (IR, AthleteID "")
	// counts() == 1,0,1,1; summary() == "1 OUT · 1 Q · 1 IR"; gameDay() has 2; find("4258173") ok; returnLabel("2026-11-25") == "back Nov 25"; returnLabel("") == ""
}

const rosterJSON = `{"athletes":[
 {"position":"offense","items":[{"id":"4036378","displayName":"Jordan Love","jersey":"10","position":{"abbreviation":"QB"},"age":27,"displayHeight":"6' 4\"","displayWeight":"219 lbs","experience":{"years":6},"college":{"shortName":"Utah State"},"status":{"name":"Active"},"injuries":[]},
   {"id":"1","displayName":"No Jersey","position":{"abbreviation":"WR"},"status":{"name":"Active"},"injuries":[{"status":"Questionable"}]}]},
 {"position":"specialTeam","items":[]},
 {"position":"injuredReserveOrOut","items":[{"id":"2","displayName":"Hurt Guy","jersey":"55","position":{"abbreviation":"LB"},"experience":{"years":0},"status":{"name":"Injured Reserve"},"injuries":[{"status":"Injured Reserve"}]}]}
]}`

func TestRosterGroupsAndMissingFields(t *testing.T) {
	// client.roster(ctx, "GB") path "/teams/gb/roster"
	// Groups: OFFENSE (2 players), IR / OUT (1); specialTeam omitted
	// Love: Jersey "10", Age 27, Years 6, HasYears, College "Utah State", experienceLabel "6 yrs", seasonLabelFor "6th season"
	// No Jersey: Jersey "--", !HasYears, experienceLabel "", Injury "Questionable"
	// Hurt Guy: experienceLabel "Rookie", seasonLabelFor "Rookie"
}

const overviewJSON = `{"statistics":{"displayName":"2026 Passing","labels":["CMP","ATT","YDS","CAR","YDS"],"categories":[{"name":"passing","displayName":"Passing","count":3},{"name":"rushing","displayName":"Rushing","count":2}],"splits":[{"displayName":"Regular Season","stats":["114","171","1,504","48"]},{"displayName":"Projected","stats":["323","485","4,261","136","680"]}]}}`

func TestPlayerOverviewSlicesCategories(t *testing.T) {
	// playerOverview(ctx, "3918298") path ".../athletes/3918298/overview"
	// Sections: PASSING labels [CMP ATT YDS] values [114 171 1,504]; RUSHING labels [CAR YDS] values [48] (short split clips)
	// overview with splits only [{"displayName":"Projected"},{"displayName":"Career","stats":[...]}] → Split "CAREER"
	// `{"statistics":null}` → no sections, no error
}
```

- [ ] **Step 2: Run** `go test -run 'TestInjuries|TestRoster|TestPlayerOverview' ./...` — Expected: FAIL, `undefined: injuryReport`
- [ ] **Step 3: Implement** the three files per Interfaces.
- [ ] **Step 4: Run** `go vet ./... && go test -count=1 ./...` — Expected: `ok`
- [ ] **Step 5: Commit** — `git commit -m "Parse ESPN injury reports, rosters and player overviews"`

---

### Task 2: Session stores and messages

**Files:**
- Create: `stores.go`
- Modify: `main.go` (model fields, `Update` cases)
- Test: `player_test.go`

**Interfaces:**
- Consumes: Task 1 types and client methods.
- Produces:
  - `type injuryEntry struct { Report injuryReport; Loading bool; Error string }`, `type rosterEntry struct { Data roster; Loading bool; Error string }`, `type playerEntry struct { Stats playerStats; Loading bool; Error string }`.
  - Model fields: `injuryReports map[string]injuryEntry`, `rosters map[string]rosterEntry`, `players map[string]playerEntry` (lazily made by the helpers below).
  - `type injuriesMsg struct { team string; report injuryReport; err error }`, `type rosterMsg struct { team string; data roster; err error }`, `type playerMsg struct { id string; stats playerStats; err error }`.
  - `func (m model) requestInjuries(team string, force bool) (model, tea.Cmd)`: no-op cmd when the entry is loading, or loaded and `!force`; else marks loading and returns `loadInjuries(team)`. Same shape: `requestRoster(team, force)`, `requestPlayer(id, force)`.
  - `func (m model) loadInjuries(team string) tea.Cmd`, `loadRoster(team string) tea.Cmd`, `loadPlayer(id string) tea.Cmd`.
  - `func (m model) applyInjuries(msg injuriesMsg) model`, `applyRoster(msg rosterMsg) model`, `applyPlayer(msg playerMsg) model`: on error store `Error = err.Error()`, `Loading = false`; else store data.
  - `func (m model) injuriesFor(team string) (injuryEntry, bool)`, `rosterFor`, `playerFor`.
  - `Update` routes the three messages to the apply functions and returns no cmd.

- [ ] **Step 1: Write failing tests**

```go
func TestRequestInjuriesOncePerTeamAndStoresReply(t *testing.T) {
	// m with jsonClient serving injuriesJSON; m, cmd := m.requestInjuries("GB", false) → cmd != nil, entry Loading
	// second requestInjuries("GB", false) → cmd == nil
	// Update(runCmds(cmd)[0]) → injuriesFor("GB") loaded, 3 entries, !Loading
	// requestInjuries("GB", true) → cmd != nil
	// error reply: applyInjuries(injuriesMsg{team:"DET", err: errors.New("boom")}) → Error "boom", !Loading
}

func TestRosterAndPlayerStores(t *testing.T) {
	// same shape for requestRoster("GB") and requestPlayer("4036378")
}
```

- [ ] **Step 2: Run** — Expected: FAIL, `undefined: m.requestInjuries`
- [ ] **Step 3: Implement** `stores.go` and the `Update` cases.
- [ ] **Step 4: Run suite** — Expected: `ok`
- [ ] **Step 5: Commit** — `git commit -m "Store injury reports, rosters and player stats per session"`

---

### Task 3: Season page tabs

**Files:**
- Create: `teamtabs.go` (tab line, roster and injury line builders, selection helpers)
- Modify: `main.go` (fields, keys), `team_ui.go` (`renderTeam`, `teamLines` dispatch, page size), `teampage.go` (`openTeamSeason` resets tab), `openFavoriteSeason` and home case 3 reset tab
- Test: `player_test.go`

**Interfaces:**
- Consumes: Task 2 `requestRoster`, `requestInjuries`, `rosterFor`, `injuriesFor`; Task 1 labels.
- Produces:
  - Model fields: `teamTab int` (0 schedule, 1 roster, 2 injuries), `rosterSelected, rosterScroll, injurySelected, injuryScroll int`.
  - `func (m model) tabLine() string`: `1 SCHEDULE     2 ROSTER     3 INJURIES` with the active tab wrapped `[ ... ]`, rendered like the game-center tab line.
  - `func (m model) setTeamTab(tab int) (model, tea.Cmd)`: sets the tab, and for 1/2 calls `requestRoster`/`requestInjuries` for `activeTeamPage().Team` with `force=false`.
  - `func (m model) rosterLines() []string`: `Loading roster…` / `Roster: {error}` / group headers in `sectionStyle` / rows `#10  QB   Jordan Love             27  6' 4"  219 lbs  6 yrs  Utah State` (`  OUT`/`  Q`/`  D`/`  IR` appended from `Injury`, else the status text when not Active-ish); selected row in `selectedStyle`.
  - `func (m model) rosterRows() []rosterPlayer`: selectable players in render order; `rosterSelectedLine() int` maps selection to line index (headers skipped).
  - `func (m model) injuryLines() []string`: header `INJURY REPORT  ·  {summary}` (`INJURY REPORT` alone when summary empty), `No injuries reported.`, rows `OUT   DE   Micah Parsons           Knee - ACL      back Nov 25` + dim wrapped comment lines (`wrapWords(comment, inner-4)`); `injuryRows() []injury`; `injurySelectedLine() int`.
  - `teamLines()` returns the active tab's lines; `renderTeam` inserts `tabLine()` after the header rule; `teamPageSize` subtracts one more line for the tab line.
  - Keys on the season page: `1`/`2`/`3`/`tab` → `setTeamTab`; `↑`/`↓`/pgup/pgdown/wheel act on the active tab's selection via `scrollBy`; `r` on tabs 1/2 → `requestRoster`/`requestInjuries` with `force=true`; `t` keeps using `teamSelected` (schedule selection) on every tab.
  - `openTeamSeason` and `openFavoriteSeason` and home case 3 set `teamTab = 0` and zero the four new fields.
  - Enter on tabs 1/2 is wired in Task 4; here it is a no-op.

- [ ] **Step 1: Write failing tests**

```go
func TestSeasonPageTabsLoadLazilyAndRender(t *testing.T) {
	// favoriteSeasonModel with jsonClient serving rosterJSON for /roster and injuriesJSON for /injuries; showTeam
	// View contains "[ 1 SCHEDULE ]"; key "2" → cmd fetches roster; after Update(reply) View contains "[ 2 ROSTER ]", "OFFENSE", "#10  QB   Jordan Love", "IR / OUT", "#--"
	// key "3" → View contains "INJURY REPORT  ·  1 OUT · 1 Q · 1 IR", "OUT   DE   Micah Parsons", "back Nov 25", "Targeting Week 12."
	// key "tab" from tab 2 → tab 0
	// height 24 width 100: every tab's View fits
}

func TestSeasonPageTabSelectionAndRefresh(t *testing.T) {
	// on roster tab: down → rosterSelected 1; rosterSelectedLine() skips the group header
	// on injuries tab: "r" → cmd != nil (force refetch); on roster tab "r" refetches roster not the schedule
	// opening another team (openTeamSeason("DET")) resets teamTab to 0 and selections to 0
}

func TestInFlightReplyForAnotherTeamIsStoredNotShown(t *testing.T) {
	// GB roster tab requests; then openTeamSeason("DET") and key "2" (DET loading)
	// Update(rosterMsg{team:"GB", ...}) → rosterFor("GB") loaded; View still shows "Loading roster…" for DET
}
```

- [ ] **Step 2: Run** — Expected: FAIL on `[ 1 SCHEDULE ]`
- [ ] **Step 3: Implement**
- [ ] **Step 4: Run suite** — Expected: `ok`; `TestFavoriteSeasonPageUnchanged` may need its fixture updated for the tab line only.
- [ ] **Step 5: Commit** — `git commit -m "Add roster and injury tabs to the season page"`

---

### Task 4: Player card

**Files:**
- Create: `playercard.go`
- Modify: `main.go` (fields, Enter on tabs, Esc/r/scroll on the card), `ui.go` (`View` order: help → settings → picker → overview → player → home …; footer)
- Test: `player_test.go`

**Interfaces:**
- Consumes: Task 2 `requestPlayer`, `playerFor`, `injuriesFor`, `rosterFor`; Task 1 `statusLabel`, `returnLabel`, `seasonLabelFor`, `find`.
- Produces:
  - `type playerRef struct { ID, Name, Position, Team, TeamName, Jersey, Height, Weight, College string; Age, Years int; HasYears bool }`.
  - Model fields: `showPlayer bool`, `player playerRef`, `playerScroll int`.
  - `func playerFromRoster(p rosterPlayer, team, teamName string) playerRef`; `func playerFromInjury(i injury, team, teamName string) playerRef` (fills bio from `rosterFor(team)` when loaded and the ID matches).
  - `func (m model) openPlayer(ref playerRef) (model, tea.Cmd)`: no-op when `ref.ID == ""`; sets `showPlayer`, `playerScroll = 0`, returns `requestPlayer(ref.ID, false)`.
  - `func (m model) playerLines(inner int) []string`: `Jordan Love` bold, `#10  QB  ·  Green Bay Packers`, bio `27 yrs  ·  6' 4"  ·  219 lbs  ·  6th season  ·  Utah State` (empty parts omitted, line omitted when all empty), injury line `OUT  ·  Knee - ACL  ·  back Nov 25` in `liveStyle` plus wrapped comment when `injuriesFor(team)` has the ID, blank, `SEASON STATS`, then per section `PASSING` (or `PASSING  ·  CAREER`), labels row and values row aligned with `%*s` columns sized to the wider of label and value, `No season stats yet.` / `Loading stats…` / `Stats: {error}`.
  - `func (m model) renderPlayer(width int) string`: framed like the game center with title `◆  PLAYER`, scrolled by `playerScroll`, page size `height-7`; compact variant without the frame.
  - Keys on the card: `up/k`, `down/j`, pgup/pgdown, wheel scroll; `r` → `requestPlayer(id, true)`; `esc`/`backspace` → `showPlayer = false` (season page and tab untouched); `q` quits. Checked before `canGoBack()`.
  - Enter on the roster tab opens `playerFromRoster(rosterRows()[rosterSelected], …)`; on the injuries tab `playerFromInjury(injuryRows()[injurySelected], …)`. `TeamName` is `activeTeamPage().Data.Team.DisplayName` or the team abbreviation.
  - Footer for the card: `↑/↓ scroll  ·  Esc back  ·  r refresh  ·  q quit`.

- [ ] **Step 1: Write failing tests**

```go
func TestEnterOnRosterOpensPlayerCardWithStats(t *testing.T) {
	// season page roster tab loaded; enter on Love → showPlayer; cmd fetches overview; after reply View contains "◆  PLAYER", "Jordan Love", "#10  QB  ·  Green Bay Packers", "27 yrs  ·  6' 4\"  ·  219 lbs  ·  6th season  ·  Utah State", "PASSING", "CMP", "1,504", "RUSHING"
	// esc → !showPlayer, showTeam, teamTab still 1, rosterSelected unchanged
	// fits at 80x24 and 100x30
}

func TestEnterOnInjuryRowShowsInjuryLine(t *testing.T) {
	// injuries tab, enter on Parsons → View contains "OUT  ·  Knee - ACL  ·  back Nov 25", "Targeting Week 12."
	// enter on the row with no athlete ID → showPlayer stays false
}

func TestPlayerCardNoStatsAndError(t *testing.T) {
	// playerMsg with empty sections → "No season stats yet."; playerMsg err → "Stats: boom"; "r" refetches
}
```

- [ ] **Step 2: Run** — Expected: FAIL, `undefined: m.showPlayer`
- [ ] **Step 3: Implement**
- [ ] **Step 4: Run suite** — Expected: `ok`
- [ ] **Step 5: Commit** — `git commit -m "Open a player card from roster and injury rows"`

---

### Task 5: Game center injury report and spotlight count

**Files:**
- Modify: `gamecenter.go` (section), `main.go` (requests on game-center open and `r`, startup and tick for the favorite), `dashboard.go` (spotlight line)
- Test: `player_test.go`

**Interfaces:**
- Consumes: Task 2 stores; Task 1 `gameDay`, `statusLabel`, `summary`.
- Produces:
  - `func injuryWindow(kickoff, now time.Time) bool`: `!kickoff.IsZero() && |now-kickoff| <= 14*24h`.
  - `func (m model) gameInjuryLines() []string`: `INJURY REPORT` header; per team (away, home) header with the abbreviation, rows `OUT  DE  Micah Parsons (Knee - ACL)` from `gameDay()` capped at 8 then `+N more`; `No Out, Doubtful or Questionable players.`; `Loading injury report…` while either entry is loading or absent; `Injuries: {error}` per team on error. Appended by `gameCenterLines()` after the drive timeline only when `injuryWindow(g.dateTime(), time.Now())`.
  - Opening the game center (every path that sets `showDetail = true` and `loadDetail`) also calls `requestInjuries` for both abbreviations when in the window; `r` in the game center forces both.
  - `func (r injuryReport) gameDaySummary() string` (add to `injuries.go`): like `summary()` but OUT, D, Q only.
  - `func (m model) spotlightInjuryLine() string`: `INJURIES  2 OUT · 1 Q` from the favorite's report `gameDaySummary()`; `""` when loading, absent, or empty. `spotlightPanel` appends it after the record/next lines and before live lines, counting toward `maxLines`.
  - `Init()` batches `loadInjuries(favorite)` when `favorite != ""`; the tick path that reloads the favorite's schedule also requests its injuries with `force=true`.

- [ ] **Step 1: Write failing tests**

```go
func TestGameCenterInjurySectionWithinWindow(t *testing.T) {
	// detail for CAR @ GB dated now-1h; open via enter on dashboard → cmds include injuries for CAR and GB
	// after replies (GB injuriesJSON, CAR empty) View on tab 4 contains "INJURY REPORT", "GB", "OUT  DE  Micah Parsons (Knee - ACL)", "CAR", "No Out, Doubtful or Questionable players."
	// game dated now-30d → no "INJURY REPORT" and no injury requests
	// 9 game-day entries → 8 rows then "+1 more"
}

func TestSpotlightShowsInjuryCount(t *testing.T) {
	// dashboardModel with injuryReports["GB"] loaded from injuriesJSON → View contains "INJURIES  1 OUT · 1 Q" (IR omitted)
	// entry loading → no "INJURIES" line; Init() with favorite GB batches an injuries fetch
}
```

- [ ] **Step 2: Run** — Expected: FAIL on `INJURY REPORT`
- [ ] **Step 3: Implement**
- [ ] **Step 4: Run suite** — Expected: `ok`; `TestDashboardFitsTerminalSizes` and the live-feed fit tests still pass.
- [ ] **Step 5: Commit** — `git commit -m "Show injury reports in the game center and on the spotlight panel"`

---

### Task 6: Footers, help, README

**Files:**
- Modify: `ui.go` (season page footer), `help.go`, `README.md`
- Test: `player_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestFootersAndHelpMentionTabs(t *testing.T) {
	// season page View at 120x30 contains "1/2/3 tabs"; help contains "1/2/3 on a season page"
}
```

- [ ] **Step 2: Run** — Expected: FAIL
- [ ] **Step 3: Implement**: season page footer `1/2/3 tabs  ·  ↑/↓ select  ·  Enter open  ·  t opponent  ·  Esc back  ·  r refresh  ·  q quit` (narrow: `1/2/3 · ↑/↓ · Enter · t · Esc · q`); help NAVIGATION gains `1/2/3 on a season page: schedule, roster, injuries`; README Features gains "Rosters and injury reports" and "Player cards", Controls row for `1`–`4` covers season page tabs, Data and refresh behaviour names the injuries, roster and athlete overview endpoints.
- [ ] **Step 4: Run** `go vet ./... && go test -count=1 ./...` — Expected: `ok`
- [ ] **Step 5: Commit** — `git commit -m "Document rosters, injury reports and player cards"`
