# Live-game polish

Date: 2026-11-01
Status: approved 2026-11-01 (grilling rounds 1 and 2, all answers yes)

Terms follow `GLOSSARY.md`: situation, red zone, scoring event, scoring
feed, spotlight panel, game center.

## Intent

On a Sunday with several games live, the dashboard today says only
`● LIVE  2:14 - 4TH  ·  Lambeau Field` per game. Who has the ball, where,
and what just happened live only inside a game center. This slice brings
the situation onto the dashboard and adds a scoring feed so the user sees
every score as it lands without opening each game.

## Goals

- Every live dashboard row shows the situation in place of the venue.
- Red zone is visible at a glance.
- Timeouts remaining show in the game center and the spotlight panel.
- A scoring feed lists recent scoring events across all live games with
  the scoring play's text.
- The spotlight panel shows the favorite team's latest play.

## Non-goals

- No injuries, inactives, win probability, or new key bindings.
- No change to `td -status`, `-watch`, `-split`, or the alert text.
- No persistence of the feed across launches.

## Anti-goals

- Extra network traffic that scales with the number of live games every
  poll. The feed costs one summary request per scoring event, never one
  per game per poll.
- A dashboard that grows a line per game. Rows stay two lines each.

## Constraints

- Go 1.24, no new dependencies. Bubble Tea and Lip Gloss as today.
- Data comes from fields already in the scoreboard payload's `situation`
  (`possession`, `down`, `distance`, `isRedZone`, `downDistanceText`,
  `possessionText`, `homeTimeouts`, `awayTimeouts`, `lastPlay.text`) and the
  per-game summary's `scoringPlays`. The timeout and text fields are not
  parsed today and must be added to `gameSituation`. They are only present
  during live games, so they cannot be verified against ESPN until a game
  is live; the builder tests against recorded JSON shaped as above.
- Existing tests in `*_test.go` keep passing. New rendering is tested the
  same way: build a `model`, call `View()`, assert on substrings and
  height.

## Decided behaviour

### Situation line (dashboard rows, game center, spotlight)

- `func situationText(g game) string` renders, for a live game:
  `GB ball  3rd & 7 at CAR 22`, using `downDistanceText` verbatim when
  present, else `3rd & 7` built from `down` and `distance` (`& GOAL` when
  distance is 0), else nothing after the possession. With possession but
  no down (kickoff, conversion): `GB ball`. With no possession: `""`.
- Red zone appends `  ·  RED ZONE`.
- Dashboard live rows: second line becomes
  `● LIVE  Q4 2:14  ·  GB ball  3rd & 7 at CAR 22  ·  RED ZONE`, clipped to
  the panel. The venue is dropped for live rows only; upcoming and final
  rows are unchanged. Clock and period come from `statusText()` as today.
- Red zone rows render the second line in `liveStyle` whether or not the
  row is selected; the selected row keeps `selectedStyle` for line one.
  In `mono` the live style is bold, which is the whole signal.
- Game center: the existing `liveContextLines` output is replaced by one
  situation line plus a timeouts line (below). The LATEST play line stays.
- Spotlight panel: when the favorite's game is live, add the situation
  line, then the timeouts line, then the latest play wrapped to the panel
  width, at most 2 lines.

### Timeouts

- `func timeoutsText(g game) string` renders `GB TO ●●●   CAR TO ●●○`
  (away team first, matching the score order), three glyphs per team,
  `●` for a remaining timeout, `○` for a used one. Empty when ESPN sends
  no timeout fields.
- Shown in the game center header and the spotlight panel only. Never in
  dashboard rows or the compact layout.

### Scoring events and the feed

- `scoreAlerts` in `alerts.go` is generalised into
  `func scoringEvents(previous, current scoreboard) []scoringEvent`, which
  returns every score change or new final between two boards, favorite or
  not. `scoringEvent` carries the game ID, team abbreviation that scored,
  points added, both scores after the change, period, clock, whether the
  favorite is in the game, and an optional `playText`.
- Alerts filter that list exactly as `scoreAlerts` does today (favorite
  only unless `-notify-all`, finals included) and keep the same text.
- Detection happens where alerts are detected today, on each board
  refresh, and is skipped on first load and on a cached board.
- Feed entries show at once with the diff: `Q4 2:14  GB +7  CAR 19 – 27 GB`,
  `★` prefix when the favorite is in the game. Scores always read away –
  home with the abbreviation beside each score, the same rule as every
  other score in the app. The model then issues one summary fetch for that
  game; when a scoring play's `awayScore`/`homeScore` equal the event's
  scores, the entry becomes
  `Q4 2:14  GB TD  CAR 19 – 27 GB  Love 12-yd pass to Watson (Crosby kick)`,
  play text last so clipping trims the play, never the score. The
  play type label is the summary's `scoringType.abbreviation` (`TD`, `FG`,
  `SF`, `2PT`) or the diff points when absent. If no play matches, retry on
  the next two refreshes, then keep the diff entry.
- Finals with no score change produce an alert as today but no feed entry.
- When the scoring game is open in the game center, its regular summary
  refresh serves the enrichment too and no extra request is made. Any
  summary response, whichever view asked for it, enriches matching feed
  entries.
- The feed holds the 20 newest events, newest first, in memory; it clears
  when the week changes and starts empty on launch.

### Feed placement and height budget

- Wide dashboard (`width >= 112 && height >= 34`): a `SCORING FEED` panel
  in the right column between the spotlight panel and the conference
  snapshot, showing at most the newest 6 entries, each one line clipped to
  the panel. Empty state: `No scores yet this session`. The Quick Keys
  panel is removed; `?` still lists the keys.
- Narrow dashboard: one line directly under the summary line,
  `⚡ Q4 2:14  GB TD  CAR 19 – 27 GB  Love 12-yd pass to Watson`, newest
  entry only, clipped; no row when the feed is empty.
- The dashboard never exceeds the terminal height. Both layouts measure
  the rendered spotlight (and feed line) instead of assuming a fixed
  height, and size the game list from what remains. On the wide layout the
  feed shrinks from 6 entries toward 1 before the game list drops below 4
  rows; if even 1 entry does not fit, the feed panel is omitted. The
  spotlight's live lines are dropped in this order when the terminal is
  short: play text first, then timeouts, then the situation line, so the
  game list always keeps at least 1 row at 80x24.
- Compact layout (`-compact`): no feed. Rows append the situation only
  when the terminal is 60 columns or wider.

### Help screen

`?` legend gains `RED ZONE  offense inside the 20` and
`TO ●●○  timeouts remaining`.

## Calls made after the builder check

- Scores in feed entries read away – home with both abbreviations, play
  text last.
- Layout math measures rendered panels; feed entries and spotlight live
  lines yield before the game list does.
- One summary response serves both the game center and the feed when the
  IDs match.

## Left to the builder

- Where the feed slice and retry counters live on `model`.
- Whether `situationText` reuses or replaces `liveContextLines`.
- Exact column alignment inside the feed panel.
- Test JSON fixtures for live situations and summary scoring plays.
