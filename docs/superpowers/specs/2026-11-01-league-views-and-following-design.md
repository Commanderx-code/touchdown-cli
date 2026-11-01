# League views, followed teams and background alerts

## Intent

Touchdown answers "how is my team doing" well. On a Sunday in November the next
questions are league-wide: who is in the playoffs if the season ended today, who
leads the league, and how the other teams I care about are doing.

## Parts

### Playoff picture and bracket

A screen with two tabs, opened with `P` or from the home menu.

- **Picture:** one conference at a time, ←/→ to switch, starting on the
  favorite's. Seeds 1 to 4 under division leaders, 5 to 7 under wild card, the
  next four under in the hunt, then the wild-card matchups if the season ended
  today. The favorite is highlighted and always listed, even when out of the
  hunt.
- **Bracket:** the four postseason rounds with scores or kickoff times.
  Rounds refresh with the scores while the tab is open. It opens first during
  the postseason.

### League leaders

A screen opened with `L` or from the home menu. Top five in twelve categories.
Enter on a player opens the player card, and Esc comes back.

### Followed teams

The favorite stays the single home team. Other teams can be followed.

- `td -follow KC,DET` saves the list, and `td -follow ""` clears it. Space in
  settings toggles the highlighted team.
- The status line, `-live`, the ticker and score alerts include followed teams.
- The dashboard's favorite filter shows them too.

### Background alerts

`td -status -watch -notify` and `-notify-all` send the same desktop alerts as
the app.

### Releases

Pushing a `v*` tag builds Linux, macOS and Windows binaries and publishes them
on a GitHub release.

## Calls made by the builder

The person asked for all of these with no further questions, so these are mine.

- Keys are `P` and `L`, because `p`, `l`, `t` and `s` are taken.
- Clinch markers show only when ESPN sends them.
- Seeds come from ESPN's `playoffSeed`, not a local tiebreaker calculation.
- Bracket games do not open the game center. It needs the game on the current
  scoreboard, and a postseason scoreboard is a separate page of data.
- Leaders use ESPN's own categories and values, for the current season type.
- Followed teams live in the config as `followed_teams`. The favorite drives the
  theme of the home screen, spotlight and season keys as before.
- The status line joins several games with ` | `, favorite first.
- `tdprompt` and the Starship segment need no change.
- Alerts use `notify-send`, so they work on Linux only, like the app's alerts.
