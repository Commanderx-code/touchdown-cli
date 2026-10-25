# Touchdown

A keyboard-driven NFL dashboard for the terminal, inspired by [kickoff-cli](https://github.com/e12jassir/kickoff-cli). It is an original Go implementation using Bubble Tea and Lip Gloss.

## Run

Requires Go 1.24 or later. An internet connection is needed on first launch; saved responses remain available offline.

```sh
make build
./bin/touchdown
```

Install the `touchdown` and `td` commands for your user:

```sh
make install
td
```

`make install` places the binary and a `td` shortcut in `~/.local/bin` by default. Use `make install PREFIX=/some/path` to choose another prefix; ensure the resulting `bin` directory is on your `PATH`.

`go run .` also works. Touchdown opens on a pixel-style home menu. Choose all games, live games, your favorite team's watch and season pages, or AFC/NFC standings with the arrow keys and Enter. The scores dashboard shows live/final/upcoming totals, a favorite team spotlight, weekly scores, and a conference standings snapshot on wide terminals. The spotlight shows the next matchup and local kickoff during bye weeks and after the displayed game finishes, plus the latest result. It stacks the cards on narrower terminals.

Green Bay (`GB`) is the default favorite. To save a different team:

```sh
td -set-favorite BUF
td -show-config
td
```

You can also edit `~/.config/touchdown/config.json` and set `"favorite_team"` to an NFL abbreviation or team name, such as `"BUF"` or `"Bills"`. Set `"theme"` to `"mint"`, `"amber"`, or `"mono"`. Press `c` inside Touchdown to choose a team with ↑/↓ and preview a theme with ←/→; Enter saves both, while Esc cancels. The saved favorite controls the watch card, starred games, favorite filter, season schedule and leaders, standings focus, and favorite score alerts. `td -favorite DET` or `td -theme amber` overrides the saved setting for one launch. Team names and abbreviations are matched case-insensitively.

The keys are the same for every favorite team: `g` opens its game for the displayed week, and `p` opens its season page. `g` works from the home menu or scores dashboard, even when the Live Now filter is on. During a bye week, `g` opens the season page with an explanation. Before kickoff, game stats will appear as a placeholder until ESPN provides them.

Use `j`/`k` or arrow keys to move through the home menu and games, and Enter to open a selection. On the scores dashboard, `h`/`l` or left/right arrows change weeks; `Esc` returns to the home menu. Press `s` for AFC/NFC standings, then left/right to switch conferences. On the season page, ↑/↓ selects a game and Enter opens its stats and game center; Esc returns to the same place in the schedule. In game detail, `1`–`4` (or Tab) switch between team stats, game player stats, play-by-play, and the game center with quarter scores, scoring plays, and a drive timeline. Live games also show possession, down and distance, and the latest play when available. The play list starts with the newest event. `j`/`k`, the mouse wheel, or Page Up/Page Down scroll lists. Press `?` for a help and score legend. Standings show the visible row range; when all 16 teams fit, the list is already fully visible. `r` refreshes and `q` quits. Scores refresh every 30 seconds while the app is open; the season page refreshes every five minutes.

The favorite team's schedule loads in the background with the dashboard and refreshes every five minutes. The season page shows the record, next game, bye week, full schedule, and passing/rushing/receiving yard leaders. Select any game to inspect its full box score. Season leader totals are calculated when you open the season page from completed game box scores and may be partial when a game summary is unavailable.

```sh
go run . -once
go run . -season 2026 -week 7
go run . -season 2026 -week 1 -type 3
go run . -favorite DET
go run . -theme amber
go run . -theme mono -compact
go run . -notify
go run . -notify-all
```

Themes are `mint` (default), `amber`, and `mono`. The settings screen saves the chosen theme; `-theme` overrides it for one launch. `-compact` uses a shorter layout. Desktop alerts are off by default. `-notify` alerts on favorite team score changes and final scores; `-notify-all` does so for every game. Alerts require `notify-send` and a running desktop notification service. Touchdown never alerts for an initial load or a cached response.

Successful responses are saved under the operating system's user cache directory (usually `~/.cache/touchdown`). If a request fails, Touchdown displays the last saved response and marks it `CACHED`. Use `r` to retry. Cached data can be old, so check the displayed timestamp when offline.

The app reads ESPN's publicly accessible but undocumented NFL scoreboard, summary, team schedule, and standings endpoints. ESPN can change or restrict them, so this is a hobby prototype rather than a guaranteed data service. Times appear in the terminal's local timezone. Play-by-play and stats may be absent before kickoff or while ESPN is updating a game. Standings update when ESPN records results, which may lag live scores.
