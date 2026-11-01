# Status line and ticker pane

Date: 2026-11-01
Status: approved 2026-11-01

## Intent

Touchdown is a full-screen dashboard. While working, the owner wants the
favorite team's score within eye-shot without switching to it: a short
segment in the shell prompt, and a narrow side pane that keeps a live
ticker running next to the editor. This design adds a `-status` mode that
serves both, plus a `-split` helper that opens the ticker pane in the
multiplexers and terminals the owner uses.

## Goals

- `td -status` prints one short, plain-text line and exits quickly enough
  to sit in a shell prompt.
- `td -status -watch` redraws a compact multi-game ticker in place every
  30 seconds, suited to a narrow vertical pane.
- `td -status -split` opens that ticker in a new pane beside the current
  one in tmux, Zellij, Kitty, WezTerm and herdr, and tells the user what to
  run by hand anywhere else.
- README documents a Starship `[custom.nfl]` block and a one-line recipe
  per supported host, including herdr's keybinding config.

## Non-goals

- No colour, icons or Nerd Font glyphs in Touchdown's own output. Starship
  and the host add those. The README snippet may include a football emoji
  in the Starship `format`, not in Touchdown's stdout.
- No new data. Everything comes from the scoreboard endpoint Touchdown
  already fetches. No play-by-play, no injuries.
- No daemon. The only background work is a short-lived detached refresh
  child that fetches once and exits (see "Prompt-speed cache"). The watch
  loop is a plain foreground loop.
- Opening a pane in Ghostty, Konsole, Alacritty or any terminal without a
  split CLI is out of scope; those get the manual-command fallback.

## Anti-goals

- A prompt segment that stalls the shell. If the status line regularly adds
  more than about 100 ms to a prompt, the feature has failed even if the
  output is right.
- Hammering ESPN from a prompt that redraws on every keystroke in some
  shells. Prompt mode must never fetch more often than the TUI does.
- Breaking `-once`, the TUI, or existing flags. `-status` is additive.

## Constraints

- Go 1.24, module `touchdown`, single `main` package, no new dependencies.
  Bubble Tea is not used for status mode; it is plain `fmt` to stdout.
- Reuse `client.scoreboard`, `game.sides`, `game.statusText`, the cache
  under `~/.cache/touchdown`, and the favorite-team resolution in `main`.
- Timezone, favorite team, `-favorite`, `-config`, `-season`, `-week`,
  `-type` behave exactly as they do for `-once`.
- `-status` with `-notify`, `-compact`, `-theme`, `-once` is an error
  (exit 2) with a one-line message, except `-theme`, which is ignored.
- Tests use the existing table-driven style in `*_test.go` and never hit
  the network.

## Decided behaviour

### `td -status` (one line)

Output rules, in priority order, for the favorite team's game in the
displayed week (current week unless `-season`/`-week` given):

| Situation | Output | Example |
| --- | --- | --- |
| In progress | `FAV score-opp score OPP · period clock` with favorite first regardless of home/away. Append ` · BALL` when the favorite has possession, ` · RZ` when ESPN's situation has `isRedZone` true (add that field to `gameSituation`). | `GB 27-19 CAR · Q4 2:14 · BALL` |
| Halftime / end of period | Same, with ESPN's short detail in place of period and clock. | `GB 17-10 CAR · Halftime` |
| Final | `FAV W 27-19 CAR · Final` or `L`. Overtime shows `Final/OT`. | `GB W 27-19 CAR · Final` |
| Scheduled | `FAV vs OPP · Sun 1:00 PM` (home) or `FAV @ OPP · Sun 1:00 PM` (away). Day and time in local zone, no zone suffix. If kickoff is today, drop the day: `GB vs DET · 1:00 PM`. | `GB @ DET · Sun 1:00 PM` |
| Bye week or no game found | Empty stdout, exit 0. Starship hides an empty segment. | |
| Postponed / cancelled | `FAV vs OPP · Postponed` using ESPN's short detail. | |
| Cache older than 5 minutes | Normal line with ` (cached)` suffix, exit 0. | `GB 27-19 CAR · Q4 2:14 (cached)` |
| No cache yet | Empty stdout, exit 0, refresh child spawned. | |

Score always reads favorite first: `GB 27-19 CAR` means Green Bay 27,
Carolina 19. Period uses `Q1`-`Q4` and `OT`; clock is ESPN's `displayClock`
(add `displayClock` and `period` to `gameStatus`).

### Prompt-speed cache

Starship runs the command on every prompt and kills it at
`command_timeout` (500 ms default; the owner's config uses 1200 ms). A
prompt invocation therefore never waits on the network.

- `td -status` reads the scoreboard cache and prints from it immediately.
  It never performs a fetch itself.
- If the cache is missing or older than 30 seconds, it spawns a detached
  child, `<self> -status -refresh` (plus any forwarded flags), and exits
  without waiting. The child fetches the scoreboard with a 3-second
  timeout, writes the cache through the existing `writeCached`, and exits.
  Its stdout and stderr go to `/dev/null`.
- A refresh marker file `<cache dir>/status-refresh` is touched by the
  parent before spawning. No new child is spawned while the marker is
  younger than 30 seconds, so a slow or failing network costs at most one
  attempt per 30 seconds regardless of how often the prompt redraws.
- First prompt with no cache: empty stdout, exit 0; the next prompt has
  data. The README says so.
- Age for the ` (cached)` suffix: cache older than 5 minutes. Fresh or
  slightly stale data prints no marker. This replaces the earlier meaning
  of "fetch failed".
- `-refresh` is a hidden maintenance flag: present in `-h` output but
  documented only in this spec. It is an error without `-status`.
- `-watch` and `-split` do not use the detached child. The watch loop
  fetches in-process with the 3-second timeout and falls back to cache as
  today, since nothing is waiting on it.
- The 30-second and 5-minute figures are not configurable in this release.

### `td -status -watch` (ticker)

- Clears the screen and redraws every 30 seconds until Ctrl-C. Uses
  ANSI `\033[H\033[2J`; no alt screen, no Bubble Tea.
- Line order: favorite team's game first, then other in-progress games by
  kickoff time, then upcoming games by kickoff, then finals. A blank line
  separates the favorite from the rest.
- Each line: `AWAY  sc  HOME  sc   status`, same shape as `-once`, plus
  the live suffixes (`BALL GB`, `3rd & 7`, `RZ`) when present. Lines wrap
  at the terminal width when it can be read, otherwise at 60 columns.
- Header line: `Week 9 · updated 14:32:07` (local time), suffixed
  `· cached` when the board came from fallback cache.
- Ctrl-C exits 0 with no error text. Fetch errors print on stderr and the
  last good board stays on screen.

### `td -status -split` (open the pane)

Detection order and command, favoring the innermost host (a tmux session
inside Kitty is tmux):

| Host | Detect | Command run |
| --- | --- | --- |
| tmux | `TMUX` set | `tmux split-window -h -l 36 -d <self> -status -watch` |
| Zellij | `ZELLIJ` set | `zellij run -d right -n "NFL" -- <self> -status -watch` |
| herdr | `HERDR_ENV=1` | `herdr pane split --current --direction right --ratio 0.35 --no-focus`, parse `.result.pane.pane_id` from stdout JSON, then `herdr pane run <id> "<self> -status -watch"` |
| Kitty | `KITTY_PID` set | `kitten @ launch --location=vsplit --keep-focus <self> -status -watch` |
| WezTerm | `WEZTERM_PANE` set | `wezterm cli split-pane --right --percent 35 -- <self> -status -watch` |
| anything else | | print `Run this in a new pane:` and the manual command on stderr, exit 1 |

`<self>` is `os.Executable()` so the split works whether the user runs
`td`, `touchdown` or `./bin/touchdown`. `-favorite`, `-config`, `-season`,
`-week` and `-type` are forwarded to the child when explicitly set
(`flag.Visit`).

herdr's `pane run` types the string into the new pane's shell and presses
Enter, so the command is one POSIX-shell-quoted string (single-quote each
argument, escape embedded single quotes). tmux, Zellij, Kitty and WezTerm
take an argv list and need no quoting. For herdr, a `pane split` that
exits 0 but whose JSON lacks `.result.pane.pane_id` is a failure and takes
the same path as a non-zero exit.

Kitty's remote control must be enabled (`allow_remote_control yes`); if the
`kitten` call fails, fall through to the manual-command message. Any host
command that exits non-zero prints its stderr and the manual command, exit 1.

`-split` without `-status` is an error (exit 2). `-split` with `-watch`
is accepted and means the same as `-split` alone.

### README

- New "Status line" section after "Command-line options" with the
  Starship block:

  ```toml
  [custom.nfl]
  command = "td -status"
  when = "command -v td"
  format = "🏈 [$output]($style) "
  style = "bold green"
  shell = ["sh", "--norc"]
  ```

  Noting `shell` keeps the prompt from paying Fish start-up cost.
- Ticker subsection: `td -status -watch`, then `td -status -split`, then a
  compact table of the five hosts and the manual command for the rest.
- herdr recipe:

  ```toml
  [[keys.command]]
  key = "prefix+alt+g"
  type = "pane"
  command = "td -status -watch"
  description = "NFL ticker"
  ```

- Controls table unchanged. Help screen (`?`) unchanged.

### Exit codes

0 success (including empty bye output and cached fallback), 1 runtime
failure (no data, host command failed), 2 bad flags or config, matching
the existing convention in `main`.

## Left to the builder

- File layout. One new `status.go` and `status_test.go` is the obvious
  split; the builder may fold the host table into `split.go`.
- How the fresh-cache option threads through `client` (field, option
  func, or a second constructor).
- Exact column alignment in the ticker beyond "same shape as `-once`".
- How the detached child is spawned (`exec.Command` with `Setsid` on
  Linux and macOS; Windows uses `DETACHED_PROCESS`), and how tests fake it.
- Whether `isRedZone`, `displayClock`, `period` are read from
  `game.Status` or `competition.Status`; ESPN populates both, use whichever
  the existing code trusts.
- How tests fake the host commands for `-split` (an exec seam or a
  `PATH` shim both fine).

## Calls made after the builder check

- Prompt mode never fetches; a detached refresh child does, throttled by a
  marker file to one attempt per 30 seconds. Chosen over raising
  Starship's `command_timeout` because the prompt must stay under 100 ms
  on any network.
- ` (cached)` now means "older than 5 minutes", not "fetch failed".
- herdr `pane run` receives a shell-quoted string; a 0-exit split with no
  `pane_id` in the JSON is a failure.
