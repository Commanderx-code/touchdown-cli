# Schedule browsing

Date: 2026-11-01
Status: approved 2026-11-01 (grilling rounds 1 and 2, all answers yes)

Terms follow `GLOSSARY.md`: season page, team picker, season overview,
kickoff slot, favorite team.

## Intent

Touchdown answers "how is my team doing" well and "what is everyone else
doing" poorly. The season page exists only for the favorite team, and the
only way to see another week is stepping the dashboard one week at a time.
This slice opens the season page to every team, groups the dashboard by
kickoff slot, and adds a season overview that shows each week at a glance
and jumps to it.

## Goals

- Any team's season page, reached from a team picker or by jumping to an
  opponent from a matchup.
- Dashboard rows grouped under kickoff-slot headers.
- A season overview of weeks 1 to 18 with game counts, the favorite's
  matchup and bye teams, with Enter jumping the dashboard to that week.
- Esc always returns to the screen you came from.

## Non-goals

- No postseason in the overview or picker; the dashboard's `-type 3`
  browsing is unchanged.
- No new ESPN endpoints. No change to `-status`, `-watch`, `-split`.
- No injuries or player pages.

## Anti-goals

- Bulk fetching. Opening the overview must not fire 18 requests; it loads
  the visible weeks plus one ahead.
- Clobbering the favorite's data. Viewing another team must never change
  what the dashboard spotlight shows.

## Constraints

- Go 1.24, Bubble Tea, Lip Gloss, no new dependencies.
- Data: `client.teamSchedule(ctx, abbreviation, season)` and
  `client.teamLeaders` already work for any team; `client.scoreboard`
  for any week. All are cached through the existing cache.
- Existing tests keep passing. New screens are tested through `model.Update`
  with key messages and `View()` substring and height assertions, as the
  navigation tests do today.
- Key `t` is unused today and becomes the team-schedule key.

## Decided behaviour

### Team picker and any team's season page

- Home menu gains two items after the standings items: `Team schedules`
  and `Season`.
- `t` on the dashboard, or `Team schedules` on the home menu, opens the
  team picker: the same 32-team list and order as the `c` settings screen,
  favorite marked `★`, current selection highlighted. Typing letters jumps
  to the first abbreviation starting with them (buffer resets after one
  second or on a non-letter key). Enter opens that team's season page. Esc
  returns to where the picker was opened from.
- The season page renders for a "viewed team". For `p` the viewed team is
  the favorite, and the page reads the favorite's schedule and leaders
  exactly as today. For any other team the page reads a separate viewed
  slot (`viewTeam`, its schedule, leaders, loading and error state, scroll
  and selection). The favorite's slot and the dashboard spotlight are never
  touched by viewing another team.
- The viewed team's leaders load in the background the same way the
  favorite's do: one summary request per completed game, cached, with the
  same `Loading player totals…` placeholder.
- Header line shows the viewed team the way the favorite page does, for
  example `DET  6-2-0  ·  17 GAMES`.
- On any season page, with a matchup selected, `t` opens the opponent's
  season page. Enter still opens the game as today.

### Back navigation

- A history stack on the model records, for the new screens only, the
  screen to return to: picker → (dashboard | home), season page → (picker
  | season page | dashboard | home), overview → home, dashboard-from-
  overview → overview.
- Esc pops the stack. Existing `teamFromHome`, `standingsFromHome`,
  `detailFromTeam` flags keep working for the screens they cover; the
  stack only adds the new transitions. Nested opponent jumps walk back one
  page at a time.
- `q` quits from anywhere, as today.

### Kickoff slots on the dashboard

- When the favorite filter is off and the terminal has 24 or more rows,
  the dashboard list groups games by kickoff slot in kickoff order, each
  group preceded by one dim header row: the kickoff formatted in local
  time as `THU 8:15 PM`, `SUN 1:00 PM`, `SUN 4:25 PM`, `MON 8:15 PM`.
  Two games kick off in the same slot when their start times are equal to
  the minute.
- Games with no parseable date go last under `TBD`.
- Header rows are not selectable; `↑`/`↓` skip them. Row counting for the
  page size treats a header as one line, a game as two.
- Below 24 rows, with the favorite filter on, or in `-compact`, the list
  is unchanged.

### Season overview

- Home item `Season` opens the overview: one row per regular-season week
  1 to 18, in order, the current dashboard week highlighted first:

  ```
  W12  14 GAMES  ·  GB @ DET  Thu 12:30 PM  ·  BYE: KC, NE
  W11  GB BYE  ·  13 GAMES  ·  BYE: GB, LV
  W15  …
  ```

  The `…` row is a week not yet loaded.
- Loading: on open and on every scroll, request the scoreboard for each
  visible week plus one past the bottom that is not loaded and not in
  flight. Cached weeks render instantly. A failed week shows
  `W15  unavailable · r to retry`.
- Game count is the number of events. Bye teams are the abbreviations of
  the 32 teams absent from that week's events, up to 6 shown then `+N`.
  The favorite's matchup is `GB @ DET` or `GB vs DET` with the local
  kickoff `Thu 12:30 PM`, or the result `GB W 27-19 DET` once final, or
  `GB BYE`.
- `↑`/`↓`, PgUp/PgDn and wheel scroll. Enter sets the dashboard to that
  week (same code path as `←`/`→`) and shows it; Esc from that dashboard
  returns to the overview. `r` refetches the failed and visible weeks.
- Compact layout renders the same rows as a plain list.

### Footers and help

- Dashboard footer gains `t teams`; the season page footer gains
  `t opponent`; home shows the two new items; `?` lists `t` and the
  `Season` item under QUICK KEYS.

## Calls made after the builder check

- The overview is regular season only: it fetches `scoreboard(season, week, 2)`
  whatever the dashboard's `-type`, and Enter sets the dashboard to season
  type 2 and that week. Before the first scoreboard arrives (`season == 0`)
  the overview and picker show `Loading season…` and accept only Esc.
- Overview rows are W1 to W18 in order with the cursor on the current
  dashboard week and the viewport scrolled to show it. Loading is
  cursor-driven, not viewport-driven: on open fetch the current week and
  two either side (at most 5 requests), then fetch a week when the cursor
  lands on it or beside it. Unloaded rows show `…`.
- The "return to overview" entry survives `←`/`→` and `f` on the
  dashboard and is consumed by opening any other screen (game, standings,
  settings, picker, season page). Changing the favorite in settings never
  touches the viewed team or the stack.
- Nested opponent jumps restore the previous team by reloading it from
  cache at the top of the page; scroll and selection reset. The stack
  entry holds only the team abbreviation.
- The overview is a session snapshot: it refetches on open, cursor moves
  and `r`, never on the 30-second tick. Its fetches use their own message
  type and never write the dashboard board, so they cannot trigger alerts
  or the scoring feed.

## Left to the builder

- Shape of the history stack entry and where the new screen flags live
  on `model`.
- Whether the picker reuses `settings_ui.go` rendering directly or a
  shared list helper.
- Exact column alignment in the overview rows.
- Test fixtures for multi-week scoreboards.
