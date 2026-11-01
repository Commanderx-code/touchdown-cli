# Player and injury data

Date: 2026-11-01
Status: approved 2026-11-01 (defaults chosen by the builder at the user's
request; no grilling rounds)

Terms follow `GLOSSARY.md`: season page, game center, spotlight panel,
favorite team, injury report, roster, player card.

## Intent

Touchdown shows games and scores but nothing about the people playing
them. Before a game the question is "who is out", during the season it is
"who is on this team and how are they doing". This slice adds an injury
report and a roster to every team's season page, a player card for any
player on them, an injury report inside the game center, and an injury
count on the favorite's spotlight panel.

## Goals

- Every season page gets three tabs: schedule (today's page), roster and
  injury report.
- Enter on a roster or injury-report row opens that player's card: bio
  and this season's stat lines.
- The game center's CENTER tab lists both teams' Out, Doubtful and
  Questionable players.
- The spotlight panel shows the favorite's injury count.

## Non-goals

- No headshots, fantasy status, contracts, news, or game logs.
- No league-wide injury list. ESPN's all-teams feed is 8.7 MB, over the
  8 MiB response cap, and is not needed.
- No practice-squad injuries; the injury report is the active roster.
- No change to `-status`, `-watch`, `-split`, or the season overview.

## Anti-goals

- Fetching on every screen. Roster, injury report and player data load
  when first shown and stay cached for the session; `r` refetches.
- Blocking the season page. Tabs render a loading line while their data
  arrives; the schedule tab never waits on roster or injury requests.
- Clobbering the favorite's data. Rosters, injury reports and player
  stats live in session stores keyed by team or player, never in the
  dashboard's board or the favorite's schedule fields.

## Constraints

- Go 1.24, Bubble Tea, Lip Gloss, no new dependencies.
- Three new ESPN endpoints, all through the existing `getURL` cache:
  - `GET {apiBase}/injuries?team={abbr}` (lower-case abbreviation):
    entries with `status`, `type.abbreviation` (`O`, `D`, `Q`, `IR`,
    `SUSP`, `A`), `athlete.displayName`, `athlete.position.abbreviation`,
    `athlete.links[].href` (`…/id/{athleteID}/…`), `details.type`
    (body part), `details.returnDate`, `date`, `shortComment`.
    `Active` entries are dropped.
  - `GET {apiBase}/teams/{abbr}/roster`: `athletes[]` groups
    (`offense`, `defense`, `specialTeam`, `injuredReserveOrOut`,
    `practiceSquad`, `suspended`) of `items[]` with `id`, `displayName`,
    `jersey`, `position.abbreviation`, `age`, `displayHeight`,
    `displayWeight`, `experience.years`, `college.shortName`,
    `status.name`, `injuries[0].status`.
  - `GET https://site.web.api.espn.com/apis/common/v3/sports/football/nfl/athletes/{id}/overview`:
    `statistics.displayName`, `statistics.labels[]`,
    `statistics.splits[]` with `displayName` and `stats[]`.
- Every string from ESPN passes through `plainText` before rendering.
- Existing tests keep passing. New screens are tested through
  `model.Update` with key messages and `View()` substring and height
  assertions, as the navigation tests do today.

## Decided behaviour

### Season page tabs

- The season page shows a tab line under its header:
  `1 SCHEDULE     2 ROSTER     3 INJURIES`, the active one in brackets,
  switched with `1`, `2`, `3` and `Tab`. Opening any season page
  (favorite or viewed, from home, `p`, the picker, or an opponent jump)
  starts on the schedule tab with scroll and selection reset.
- Each tab has its own selected row and scroll. `↑`/`↓`, PgUp/PgDn and
  the wheel move within the active tab.
- The schedule tab is today's season page, unchanged.
- Rosters and injury reports live in session stores keyed by team
  abbreviation, shared by the season page, the game center and the
  spotlight panel, each entry with its own `Loading`/`Error` state. A
  message for any team is stored; nothing is stale, so nothing is
  dropped. Player stats live in a store keyed by athlete ID.

### Roster

- Loaded the first time the tab opens for that team. Rows are grouped
  under `OFFENSE`, `DEFENSE`, `SPECIAL TEAMS`, `IR / OUT`,
  `PRACTICE SQUAD`, `SUSPENDED` in that order, empty groups omitted,
  players in ESPN's order within a group. A row is
  `#10  QB   Jordan Love             27  6' 4"  219 lbs  6 yrs  Utah State`.
  Jersey `--` when missing; college blank when missing. When the row's
  `injuries[0].status` is set, it is appended as `  OUT`, `  Q`, `  D`,
  `  IR` or the status text.
- Group headers are not selectable. `Enter` on a row opens the player
  card. `Loading roster…` while loading; `Roster: {error}` on failure.

### Injury report

- Loaded the first time the tab opens for that team. Entries with
  `status` `Active` are dropped. Rows sort by status: Out, Doubtful,
  Questionable, Injured Reserve, then anything else; ties keep ESPN's
  order. A row is
  `OUT   DE   Micah Parsons           Knee - ACL      back Nov 25`
  with a second, dim line wrapping `shortComment` to the panel width.
  Status column uses `OUT`, `D`, `Q`, `IR`, else the type abbreviation.
  Body part and return date are blank when missing.
- A header `INJURY REPORT  ·  3 OUT · 1 Q · 2 IR` counts the statuses.
  `No injuries reported.` when the list is empty.
- `Enter` on a row opens the player card. `Loading injury report…`
  while loading; `Injuries: {error}` on failure.

### Player card

- Full-screen like the game center, titled `◆  PLAYER`. Lines:
  `Jordan Love`, `#10  QB  ·  Green Bay Packers`, a bio line
  `27 yrs  ·  6' 4"  ·  219 lbs  ·  6th season  ·  Utah State` (missing
  parts omitted; `Rookie` for zero years), and when the player appears in
  the team's injury report: `OUT  ·  Knee - ACL  ·  back Nov 25` plus
  the comment, wrapped.
- Below, `SEASON STATS`: the overview's `statistics` has one flat
  `labels[]` and `categories[]` with `displayName` and `count`; the
  labels and each split's values are sliced by those counts. For each
  category: a header line with the category name upper-cased
  (`PASSING`, `RUSHING`), the labels row, then one values row from the
  `Regular Season` split. When there is no `Regular Season` split, the
  first split not named `Projected` is used and its name is shown after
  the category header (`CAREER`). `No season stats yet.` when the
  overview has no statistics or no usable split. `Loading stats…` while loading; `Stats: {error}`
  on failure. Stats load when the card opens and are cached per player
  for the session.
- `↑`/`↓`, PgUp/PgDn and the wheel scroll. `Esc` returns to the tab
  and row the card was opened from. `r` refetches the stats.
- The card is opened only from roster and injury rows, so it always has
  a name, position and team; a card opened from an injury row has no
  age, height, weight or college unless the roster is already loaded.

### Game center injury report

- The CENTER tab (`4`) appends, after the drive timeline, an
  `INJURY REPORT` section with one subsection per team, away then home:
  the team abbreviation as a header, then rows `OUT  DE  Micah Parsons
  (Knee - ACL)` for Out, Doubtful and Questionable players, at most 8
  per team then `+N more`. `No Out, Doubtful or Questionable players.`
  when a team has none. `Loading injury report…` until both loaded.
- The section appears only when the game's kickoff is within 14 days
  of now, either side; ESPN's feed is current state, not game-time, so
  older games omit it entirely.
- Both teams' reports are requested when the game center opens unless
  the store already holds them. They are not refetched on the
  30-second tick; `r` in the game center refetches them with the
  summary.

### Spotlight panel

- When the favorite's injury report has any Out, Doubtful or
  Questionable players, the spotlight panel adds one line:
  `INJURIES  2 OUT · 1 Q` (statuses with zero count omitted, in the
  order OUT, D, Q). Nothing is added while loading or when the count is
  zero.
- The favorite's injury report is requested with the favorite's
  schedule at startup and on the five-minute schedule cadence.

### Footers, help, README, glossary

- Season page footer: `1/2/3 tabs  ·  ↑/↓ select  ·  Enter open  ·  t
  opponent  ·  Esc back  ·  r refresh  ·  q quit`; player card footer:
  `↑/↓ scroll  ·  Esc back  ·  r refresh  ·  q quit`.
- Help NAVIGATION gains `1/2/3 on a season page: schedule, roster,
  injuries`. README Features gains "Rosters and injury reports" and
  "Player cards"; the Controls table row for `1`–`4` covers season page
  tabs; Data and refresh behaviour names the three endpoints.
- Glossary gains: injury report, roster, player card.

## Calls made by the builder

- Roster rows for the `injuredReserveOrOut` group show ESPN's status
  (`Injured Reserve`) rather than the injury report's row, so the two
  tabs can disagree for a day when ESPN updates one before the other.
- The game center fetches both injury reports even for finals; the
  endpoint is small and cached, and the rows still answer "who sat out".
- Athlete IDs for injury rows are parsed from the player-card link
  (`/id/{n}/`); a row with no parsable ID still renders but Enter on it
  does nothing.

- Four builder-check questions settled here: one session store per data
  kind (team-keyed maps, no page slots); the player card renders one
  values row per category from the `Regular Season` split, labels sliced
  by `categories[].count`; the game-center injury section is limited to
  games within 14 days of now.

## Left to the builder

- Exact column widths and clipping at narrow widths; the row formats
  above are the wide layout and may drop college and height below
  80 columns.
- Whether roster and injury loading share one message type or two.
