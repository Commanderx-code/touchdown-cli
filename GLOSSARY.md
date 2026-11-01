# Touchdown

A terminal dashboard for following NFL games, built around one favorite team. This glossary fixes the words used in the interface, the docs, and the code.

## Language

**Favorite team**:
The one NFL team the user follows. It drives the spotlight panel, filtering, alerts, and the status line.
_Avoid_: My team, home team, selected team

**Scoreboard**:
ESPN's list of one week's games with scores and status. The source for the dashboard, the status line, and the ticker.
_Avoid_: Board, feed

**Game center**:
The full-screen view of one game: stats, players, plays, quarter scores, scoring plays, and drives.
_Avoid_: Detail view, game view

**Spotlight panel**:
The dashboard card for the favorite team: next game, live score, last result, record.
_Avoid_: Watch card, favorite card

**Situation**:
The live state of a game in progress: which team has the ball, down and distance, field position, red zone, timeouts left, and the last play.
_Avoid_: Context, live context, game state

**Red zone**:
The offense is inside the opponent's 20-yard line, as reported by ESPN. Not computed locally.

**Scoring event**:
One change to a game's score noticed between two scoreboard refreshes, enriched when possible with the scoring play's text.
_Avoid_: Score update, score alert

**Scoring feed**:
The dashboard list of recent scoring events across all live games, newest first, kept for the current session and week only.
_Avoid_: Ticker, score ticker, scoring ticker

**Alert**:
A desktop notification sent for a scoring event or a final, enabled with `-notify`.
_Avoid_: Notification

**Status line**:
The one-line favorite-team summary printed by `td -status` for a shell prompt.
_Avoid_: Prompt segment, status counter

**Ticker**:
The in-place scoreboard redrawn in a side pane by `td -status -watch`.
_Avoid_: Watch mode, feed
