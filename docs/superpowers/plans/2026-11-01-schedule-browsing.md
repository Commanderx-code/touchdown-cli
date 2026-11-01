# Schedule Browsing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Open the season page to every team via a picker and opponent jumps, group dashboard rows by kickoff slot, and add a season overview that jumps the dashboard to any week, with Esc always returning to the previous screen.

**Architecture:** The season page is parameterised through a `teamPage` value assembled from either the favorite's existing fields or a new viewed-team slot, so `team_ui.go` renders both without duplication. A `backTarget` stack covers only the new transitions. The overview keeps its own `map[int]overviewWeek` fed by an `overviewMsg` that never touches the dashboard board. Kickoff slots are a rendering concern inside `scoreboardPanel`.

**Tech Stack:** Go 1.24, Bubble Tea, Lip Gloss.

**Spec:** `docs/superpowers/specs/2026-11-01-schedule-browsing-design.md`. Terms per `GLOSSARY.md`.

## Global Constraints

- No new dependencies or ESPN endpoints. Existing tests keep passing.
- Viewing another team never writes `m.teamData`, `m.teamLeaders` or any favorite field; the dashboard spotlight is unchanged by it.
- Overview fetches use `seasonType` 2 and `overviewMsg`; they never write `m.board`, so alerts and the scoring feed cannot fire from them. Opening the overview fires at most 5 requests.
- Key `t`: dashboard → picker; season page → opponent. `q` quits everywhere.
- Rendering tests assert substrings and that `View()` fits `m.height`.
- Commits carry no AI attribution lines.

## Review Focus

- `t` on the favorite's season page with the bye row or no matchup selected must do nothing, not open a page for an empty abbreviation. Test in Task 3.
- A `teamScheduleMsg` for team B arriving after the user has already jumped to team C must be dropped, not rendered. Test in Task 1.
- Two games kicking off one minute apart (ESPN sometimes lists 1:00 and 1:01) form two slots; equal-to-the-minute times form one. Test in Task 4.
- Overview Enter from `-type 3` must switch the dashboard to season type 2 and reload; Esc then returns to the overview. Test in Task 5.
- Opening the overview before the first scoreboard arrives (`season == 0`) shows `Loading season…`, fires no requests, and Esc returns home. Test in Task 5.

---

### Task 1: Viewed-team slot behind the season page

**Files:**
- Modify: `main.go` (model fields, `loadTeam`/`loadTeamLeaders` take a team, `teamScheduleMsg`/`teamLeadersMsg` routing), `team_ui.go` (`teamLines`, `renderTeam` read a `teamPage`), `dashboard.go` only if it calls a changed signature
- Test: `schedule_test.go` (create)

**Interfaces:**
- Produces:
  - `type teamPage struct { Team string; Season int; Data teamSchedule; Leaders []seasonLeader; LeaderError, Error string; Loading, LeadersLoading, LeadersReady bool }`
  - Model fields: `viewTeam string` and `view teamPage`. `viewTeam == ""` or `== m.favorite` means the page shows the favorite.
  - `func (m model) activeTeamPage() teamPage`: favorite fields assembled (`Team: m.favorite, Season: m.teamSeason, Data: m.teamData, Leaders: m.teamLeaders, LeaderError: m.teamLeaderError, Error: m.teamError, Loading: m.teamLoading, LeadersLoading: m.teamLeadersLoading, LeadersReady: m.teamLeadersReady`) when showing the favorite, else `m.view`.
  - `func (m model) loadTeamFor(team string) tea.Cmd` and `func (m model) loadTeamLeadersFor(team string, data teamSchedule) tea.Cmd`; the existing `loadTeam`/`loadTeamLeaders` become one-line wrappers passing `m.favorite`.
  - `func (m model) openTeamSeason(team string) (model, tea.Cmd)`: sets `showTeam`, resets `teamScroll`/`teamSelected`/`teamNotice`; for the favorite behaves like `openFavoriteSeason("")`; otherwise sets `viewTeam = team`, and when `m.view.Team != team || m.view.Season != m.season` resets `m.view = teamPage{Team: team, Season: m.season, Loading: true}` and returns `loadTeamFor(team)`; when data exists but leaders are not ready and not loading, returns `loadTeamLeadersFor`.
  - Message routing: `teamScheduleMsg`/`teamLeadersMsg` with `msg.team == m.favorite` take today's path. With `msg.team == m.viewTeam && msg.team != m.favorite` they fill `m.view` (and on schedule success issue `loadTeamLeadersFor`). Any other team is dropped.
- `teamLines()` and `renderTeam()` read `page := m.activeTeamPage()` instead of `m.teamData`, `m.favorite`, etc. Output for the favorite is byte-identical to today.

- [ ] **Step 1: Write failing tests**

```go
func TestOpenTeamSeasonLoadsViewedTeamWithoutTouchingFavorite(t *testing.T) {
	// model favorite GB with teamData for GB (teamSeason 2026); m, cmd := m.openTeamSeason("DET")
	// cmd != nil; run it against a jsonClient serving /teams/det/schedule → teamScheduleMsg{team:"DET"}; Update with it
	// m.view.Data.Team.Abbreviation == "DET"; m.teamData unchanged (still GB events); View() Contains "DET" header and NotContains GB's next-game line
}

func TestStaleTeamScheduleMsgDropped(t *testing.T) {
	// viewTeam "DAL"; Update(teamScheduleMsg{team:"DET", data: det}) → m.view.Data has no events
}

func TestFavoriteSeasonPageUnchanged(t *testing.T) {
	// existing TestScheduleAndSeasonLeaders-style model; assert View() identical before/after setting viewTeam = "" vs favorite
}
```

- [ ] **Step 2: Run to verify failure** — `go test -run 'TestOpenTeamSeason|TestStaleTeamSchedule|TestFavoriteSeasonPageUnchanged' ./...` Expected: `undefined: teamPage`
- [ ] **Step 3: Implement**
- [ ] **Step 4: Run the suite** — `go test ./...` Expected: `ok`
- [ ] **Step 5: Commit** — `git commit -m "Render the season page for a viewed team without touching the favorite"`

---

### Task 2: Team picker screen

**Files:**
- Create: `picker.go`
- Modify: `main.go` (fields, `t` key on the dashboard, home item, Esc), `home.go` (`homeOptions` adds `TEAM SCHEDULES` and `SEASON` after the standings items), `settings_ui.go` only to share the row format if convenient
- Test: `schedule_test.go`

**Interfaces:**
- Produces:
  - Model fields: `showPicker bool`, `pickerIndex, pickerScroll int`, `pickerBuffer string`, `pickerTyped time.Time`.
  - `func (m model) openPicker() model`: `showPicker = true`, `pickerIndex` = favorite's index in `nflTeams` (0 if none), buffer cleared.
  - `func (m model) updatePickerKey(key string, now time.Time) (model, tea.Cmd)`: `up/k`, `down/j`, `pgup/pgdown`, `home/end`; letters append to `pickerBuffer` (reset when `now.Sub(pickerTyped) > time.Second` or on a non-letter) and move `pickerIndex` to the first `nflTeams` abbreviation with that prefix (case-insensitive); `enter` → `openTeamSeason(nflTeams[pickerIndex].abbreviation)` with `showPicker = false`; `esc` → `m.back()` (Task 3; until then: `showPicker = false`, return to home or dashboard via a `pickerFromHome bool`).
  - `func (m model) renderPicker(width int) string`: title `◆  TEAM SCHEDULES`, `dimStyle` range line `1–20 OF 32`, rows `› GB   Green Bay Packers` with `★` after the favorite's row, same page-size logic as `settingsPageSize`; compact variant for `height < 14 || width < 30` mirrors `renderSettings`.
- `View()` renders the picker before the home check. Home item index 6 (`TEAM SCHEDULES`) → `openPicker` with `pickerFromHome = true`; index 7 (`SEASON`) is wired in Task 5.

- [ ] **Step 1: Write failing tests**

```go
func TestPickerOpensFromDashboardAndHome(t *testing.T) {
	// dashboard model, key "t" → showPicker; View Contains "TEAM SCHEDULES" and "Green Bay"; height fits at 80x24
	// home model homeSelection 6, "enter" → showPicker
}

func TestPickerTypeToJumpAndEnter(t *testing.T) {
	// keys "d","e" within 1s → nflTeams[pickerIndex].abbreviation == "DEN"; then "t" → "DET"
	// after 2s, "k" → "KC"
	// "enter" → showTeam && viewTeam == "KC" && !showPicker
}
```

- [ ] **Step 2: Run to verify failure** — Expected: `undefined: openPicker`
- [ ] **Step 3: Implement**
- [ ] **Step 4: Run the suite** — Expected: `ok`; `TestHomeMenuFitsAndOpensViews` still passes with 8 items.
- [ ] **Step 5: Commit** — `git commit -m "Add a team picker that opens any team's season page"`

---

### Task 3: Back stack and opponent jump

**Files:**
- Create: `history.go`
- Modify: `main.go` (Esc handling, `t` on season page, pushes at each new transition)
- Test: `schedule_test.go`

**Interfaces:**
- Produces:
  - `type backTarget struct { Screen string; Team string }` with `Screen` one of `home`, `dashboard`, `picker`, `season`, `overview`.
  - Model field `history []backTarget`.
  - `func (m model) push(t backTarget) model`, `func (m model) back() (model, tea.Cmd)`: pops and shows the target: `home` → `showHome`; `dashboard` → all flags off; `picker` → `showPicker`; `season` → `openTeamSeason(Team)`; `overview` → `showOverview` (Task 5). Empty stack falls through to today's Esc behaviour.
  - Pushes: opening the picker from the dashboard pushes `dashboard`, from home pushes `home`; picker → season pushes `picker`; opponent jump pushes `season{Team: current}`; overview → dashboard pushes `overview` (Task 5).
  - `t` on a season page: `page := m.activeTeamPage()`; if `teamSelected` indexes a game whose `matchup(page.Team)` returns a non-empty opponent, push `season{Team: page.Team}` and `openTeamSeason(opponent)`; otherwise no-op.
  - Consuming the overview entry: opening game detail, standings, settings, picker or a season page from the dashboard drops a top-of-stack `overview` entry.
- Esc handling order: if `len(history) > 0` and the current screen is picker, viewed-team season page, overview, or dashboard-with-overview-on-top → `back()`; else today's code.

- [ ] **Step 1: Write failing tests**

```go
func TestOpponentJumpAndEscWalksBack(t *testing.T) {
	// favorite GB season page with teamData containing W1 @ MIN; teamSelected on it; key "t" → showTeam && viewTeam == "MIN"
	// key "t" again on MIN's page after feeding its schedule (MIN vs DET selected) → viewTeam == "DET"
	// "esc" → viewTeam == "MIN"; "esc" → favorite page (viewTeam "" or "GB"); "esc" → home or dashboard as before
}

func TestOpponentJumpIgnoresEmptySelection(t *testing.T) {
	// season page with no events: "t" leaves showTeam true and viewTeam unchanged
}

func TestPickerEscReturnsToOrigin(t *testing.T) {
	// dashboard → t → esc → dashboard (not home); home → enter(6) → esc → home
}
```

- [ ] **Step 2: Run to verify failure** — Expected: FAIL on `viewTeam == "MIN"`
- [ ] **Step 3: Implement**
- [ ] **Step 4: Run the suite** — Expected: `ok`; all `navigation_test.go` Esc cases still pass.
- [ ] **Step 5: Commit** — `git commit -m "Jump to an opponent's season page and walk back with Esc"`

---

### Task 4: Kickoff-slot headers on the dashboard

**Files:**
- Modify: `ui.go` (`scoreboardPanel`), `dashboard.go` (row budget accounts for header lines)
- Test: `schedule_test.go`

**Interfaces:**
- Produces:
  - `func kickoffSlot(g game) string`: `""` when `dateTime()` is zero, else `strings.ToUpper(t.Local().Format("Mon 3:04 PM"))` (`SUN 1:00 PM`).
  - `func (m model) slotHeadersEnabled() bool`: `!m.favoritesOnly && !m.compact && m.height >= 24`.
  - In `scoreboardPanel`, when enabled, before each game whose `kickoffSlot` differs from the previous rendered game's, emit `dimStyle.Render(slot)` or `TBD` for `""`. Headers are not selectable: `m.selected` still indexes `visibleGames()`.
  - Paging: `maxRows` stays a game count; the panel renders games `start..end` as today plus the headers between them, and the callers in `dashboard.go` subtract an estimated header count (`len(distinct slots in the window)`) via a new `func (m model) slotHeaderCount(start, end int) int` so the dashboard still fits.

- [ ] **Step 1: Write failing tests**

```go
func TestDashboardGroupsBySlot(t *testing.T) {
	// board: THU 2026-11-05T01:15Z, two SUN 18:00Z, one SUN 18:01Z, one MON 01:15Z, one with Date ""; width 100 height 30 favoritesOnly false
	// view Contains headers in order: "THU", "SUN 1:00 PM", "SUN 1:01 PM", "MON", "TBD" (local tz fixed via time.Local = America/New_York in the test, restore after)
	// Count of "SUN 1:00 PM" header == 1
	// favoritesOnly true or height 23 → no header rows
	// height fits at 100x30 and 80x24
}
```

- [ ] **Step 2: Run to verify failure** — Expected: FAIL missing `THU`
- [ ] **Step 3: Implement**
- [ ] **Step 4: Run the suite** — Expected: `ok`; `TestDashboardFitsTerminalSizes` and `TestDashboardFitsWithLiveFavoriteAndFeed` still pass.
- [ ] **Step 5: Commit** — `git commit -m "Group dashboard games under kickoff-slot headers"`

---

### Task 5: Season overview

**Files:**
- Create: `overview.go`
- Modify: `main.go` (fields, msg, keys, home item 7, Esc via stack, `r`), `history.go` (`overview` target)
- Test: `schedule_test.go`

**Interfaces:**
- Produces:
  - `type overviewWeek struct { Week int; Loaded bool; Loading bool; Err string; Games int; Favorite string; Byes []string }`
  - `type overviewMsg struct { season, week int; board scoreboard; err error }`
  - Model fields: `showOverview bool`, `overview map[int]overviewWeek`, `overviewCursor, overviewScroll int`, `overviewSeason int`.
  - `func summarizeWeek(board scoreboard, favorite string, week int, now time.Time) overviewWeek`: `Games = len(Events)`; `Byes` = `nflTeams` abbreviations absent from all events, sorted; `Favorite` = `GB @ DET  Thu 12:30 PM` (kickoff via `kickoffLabel`-style local format `Mon 3:04 PM`), or `GB W 27-19 DET` from `favoriteGameResult`-style logic once `post`, or `GB BYE` when absent, or `""` with no favorite.
  - `func (w overviewWeek) line(favorite string) string`: `W12  14 GAMES  ·  GB @ DET  Thu 12:30 PM  ·  BYE: KC, NE`; `W11  GB BYE  ·  13 GAMES  ·  BYE: GB, LV` (favorite part first when it is a bye); more than 6 byes → first 6 then ` +N`; unloaded → `W15  …`; error → `W15  unavailable · r to retry`.
  - `func (m model) openOverview() (model, tea.Cmd)`: `showOverview = true`; if `m.season == 0` return with no cmd; reset map when `overviewSeason != m.season`; cursor = current week (clamped 1..18); return `m.overviewFetches()`.
  - `func (m model) overviewFetches() (model, tea.Cmd)`: for weeks `cursor-2..cursor+2` on open, and `cursor-1..cursor+1` on cursor moves, not loaded and not loading → mark loading, batch `loadOverviewWeek(week)`.
  - `func (m model) loadOverviewWeek(week int) tea.Cmd`: `api.scoreboard(ctx, m.season, week, 2)` → `overviewMsg`.
  - `overviewMsg` handling: ignore when `msg.season != m.overviewSeason`; on error set `Err`; else `summarizeWeek`. Never touches `m.board`.
  - Keys on the overview: `up/k`, `down/j`, `pgup/pgdown`, wheel → move cursor then `overviewFetches()`; `r` → clear `Err` and `Loaded` on the three weeks around the cursor and refetch; `enter` → push `overview`, set `m.seasonType = 2`, `m.week = cursor`, `showOverview = false`, `selected = 0`, `loading = true`, return `loadBoard()`; `esc` → `back()`.
  - `func (m model) renderOverview(width int) string`: title `◆  SEASON OVERVIEW  ·  2026`, rows for W1..W18 with the cursor row in `selectedStyle`, scrolled so the cursor is visible, `Loading season…` when `m.season == 0`; compact variant is the same rows without the frame.
- `View()` renders the overview before the home check. Home item 7 → `openOverview` with `history` push `home`. Footer for the overview: `↑/↓ week  ·  Enter open week  ·  r refresh  ·  Esc back  ·  q quit`.

- [ ] **Step 1: Write failing tests**

```go
func TestSummarizeWeekLineFormats(t *testing.T) {
	// 14-game board missing KC and NE, favorite GB @ DET pre Thu → line "W12  14 GAMES  ·  GB @ DET  Thu 12:30 PM  ·  BYE: KC, NE"
	// board where GB absent → line starts "W11  GB BYE  ·  13 GAMES"
	// final GB 27-19 DET → contains "GB W 27-19 DET"
	// 8 byes → "BYE: A, B, C, D, E, F +2"
	// unloaded → "W15  …"; Err → "W15  unavailable · r to retry"
}

func TestOverviewOpenFetchesAtMostFiveWeeks(t *testing.T) {
	// model season 2026 week 12, home selection 7, enter → showOverview; run cmds against a recording client: exactly 5 scoreboard requests, weeks 10..14, all with seasontype=2
	// moving cursor down twice → requests for 15 (and nothing already loaded/loading repeated)
}

func TestOverviewEnterJumpsWeekAndEscReturns(t *testing.T) {
	// model seasonType 3; open overview, cursor to 12, enter → showOverview false, week 12, seasonType 2, cmd produces boardMsg request with week=12
	// esc on dashboard → showOverview true
	// then "t" from dashboard consumes the overview entry: picker esc → dashboard, dashboard esc → home
}

func TestOverviewBeforeSeasonKnown(t *testing.T) {
	// season 0: open → View Contains "Loading season…", no requests; esc → home
}

func TestOverviewMsgNeverTouchesBoard(t *testing.T) {
	// m.board has GB 20; overviewMsg for the same week with GB 27 → m.board unchanged, m.feed empty
}
```

- [ ] **Step 2: Run to verify failure** — Expected: `undefined: summarizeWeek`
- [ ] **Step 3: Implement**
- [ ] **Step 4: Run the suite** — Expected: `ok`
- [ ] **Step 5: Commit** — `git commit -m "Add a season overview that jumps the dashboard to any week"`

---

### Task 6: Footers, help, README

**Files:**
- Modify: `ui.go` (footers), `help.go`, `README.md`
- Test: `schedule_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestFootersAndHelpMentionTeamsKey(t *testing.T) {
	// dashboard View at 120x30 Contains "t teams"; season page footer Contains "t opponent"; help Contains "t  team schedules"
}
```

- [ ] **Step 2: Run to verify failure** — Expected: FAIL
- [ ] **Step 3: Implement**: dashboard footers (both widths) gain `t teams`; season page footer gains `t opponent`; help QUICK KEYS gains `t  team schedules` and the NAVIGATION section mentions `Season` on the home menu.
- [ ] **Step 4: README**: Controls table adds `t` (`Open the team picker on the dashboard; open the selected opponent's season on a season page`). Features: extend "Favorite-team view" to "Team season pages" covering any team; add "Season overview" bullet; "Scores dashboard" bullet mentions kickoff-slot grouping. Quick start paragraph lists the two new home items.
- [ ] **Step 5: Run suite and vet** — `go test ./... && go vet ./...` Expected: `ok`
- [ ] **Step 6: Commit** — `git commit -m "Document team schedules, the season overview and the t key"`
