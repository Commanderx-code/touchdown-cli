# Status Line and Ticker Pane Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `td -status` (one-line prompt segment), `td -status -watch` (in-place ticker) and `td -status -split` (open the ticker in a tmux, Zellij, herdr, Kitty or WezTerm pane).

**Architecture:** Status mode is plain stdout, no Bubble Tea. The prompt path reads the scoreboard cache only and hands network work to a detached `-status -refresh` child throttled by a marker file. Formatting lives in pure functions over the existing `scoreboard` type so every output shape is table-tested without network. Host detection and pane commands go through an exec seam so tests never run real multiplexers.

**Tech Stack:** Go 1.24, standard library, `github.com/charmbracelet/x/term` (already an indirect dependency) for terminal width.

**Spec:** `docs/superpowers/specs/2026-11-01-status-line-design.md`

## Global Constraints

- Go 1.24, module `touchdown`, single `main` package, no new modules beyond promoting `github.com/charmbracelet/x/term` to a direct requirement.
- Touchdown's own stdout carries no colour, icons or Nerd Font glyphs.
- Prompt mode (`-status` without `-watch`/`-split`) never performs a network request.
- Refresh throttle 30 seconds; ` (cached)` suffix when cache is older than 5 minutes; refresh child and watch loop use a 3-second HTTP timeout.
- Exit codes: 0 success (including empty bye output), 1 runtime failure, 2 bad flags or config.
- Existing flags, `-once` and the TUI keep today's behaviour.
- Commits carry no AI attribution lines.

## Review Focus

- A game where ESPN omits `score` on one side mid-game (seen briefly at kickoff) must not print `GB -19 CAR`; treat a missing score as `0`. Test in Task 2.
- A board whose events include the favorite twice (ESPN duplicate on schedule change) must pick the first by kickoff, not panic. Test in Task 2.
- Cache file present but unparseable (truncated write, disk full) must behave as "no cache": empty output, exit 0, refresh spawned. Test in Task 3.
- Marker file in a cache dir that does not exist yet must be created, not error. Test in Task 3.
- `-split` inside tmux nested in Kitty must choose tmux (innermost). Test in Task 5.

---

### Task 1: ESPN fields for clock, period and red zone

**Files:**
- Modify: `espn.go:96-103` (`gameStatus`), `espn.go:68-76` (`gameSituation`)
- Test: `status_test.go` (create)

**Interfaces:**
- Produces: `gameStatus.DisplayClock string` (json `displayClock`), `gameStatus.Period int` (json `period`), `gameSituation.IsRedZone bool` (json `isRedZone`). Both `game.Status` and `competition.Status` share `gameStatus`; status code reads `game.Status`, matching `favoriteMatch` and `statusText`.
- Also names two anonymous structs so tests can build literals: `gameStatus.Type` becomes `statusType` (fields `State`, `ShortDetail`, `Description`, same json tags) and `competitor.Team` becomes `teamRef` (fields `ID`, `DisplayName`, `Abbreviation`, `Name`, same json tags). No behaviour change.

- [ ] **Step 1: Write the failing test**

```go
func TestScoreboardParsesClockPeriodRedZone(t *testing.T) {
	api := jsonClient(t, func(*http.Request) {}, `{"events":[{"id":"1","date":"2026-11-01T18:00Z","status":{"type":{"state":"in","shortDetail":"2:14 - 4th"},"displayClock":"2:14","period":4},"competitions":[{"situation":{"possession":"8","down":3,"distance":7,"isRedZone":true},"competitors":[{"homeAway":"home","score":"27","team":{"id":"8","abbreviation":"GB"}},{"homeAway":"away","score":"19","team":{"id":"29","abbreviation":"CAR"}}]}]}]}`)
	board, err := api.scoreboard(context.Background(), 0, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	g := board.Events[0]
	if g.Status.DisplayClock != "2:14" || g.Status.Period != 4 || !g.Competitions[0].Situation.IsRedZone {
		t.Fatalf("fields not parsed: %+v", g.Status)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run TestScoreboardParsesClockPeriodRedZone ./...`
Expected: compile error, `g.Status.DisplayClock undefined`

- [ ] **Step 3: Add the three fields with the json tags above and name the `statusType` and `teamRef` structs**

- [ ] **Step 4: Run the full suite**

Run: `go test ./...`
Expected: `ok  	touchdown`

- [ ] **Step 5: Commit**

```bash
git add espn.go status_test.go
git commit -m "Parse game clock, period and red zone from the scoreboard"
```

---

### Task 2: One-line status formatter

**Files:**
- Create: `status.go`
- Modify: `dashboard.go:48-56` (`favoriteMatch` delegates to the new free function)
- Test: `status_test.go`

**Interfaces:**
- Produces:
  - `func favoriteGame(board scoreboard, favorite string) (game, bool)`: first event (board is already kickoff-sorted) where either side's abbreviation equals `favorite`.
  - `func statusLine(board scoreboard, favorite string, now time.Time, cachedAt time.Time) string`: the one-line output per the spec table, `""` when no game. `cachedAt` zero means "age unknown, no suffix"; otherwise append ` (cached)` when `now.Sub(cachedAt) > 5*time.Minute`.
  - `func periodLabel(period int) string`: `Q1`..`Q4`, `OT` for 5 and above, `""` for 0.
  - `func scoreOrZero(s string) string`: `"0"` for empty, else `s`.
- `(m model) favoriteMatch()` becomes `return favoriteGame(m.board, m.favorite)`.

Format rules from the spec, restated as code-pinned values:

| State | Output |
| --- | --- |
| `in`, clock and period present | `GB 27-19 CAR · Q4 2:14`, plus ` · BALL` when `Situation.Possession == favorite team ID`, plus ` · RZ` when `IsRedZone` and favorite has possession |
| `in`, clock empty or `Status.Type.Description` contains `Halftime` or `End of` | `GB 17-10 CAR · <Status.Type.ShortDetail>` |
| `post` | `GB W 27-19 CAR · Final`; `L`, `T` by score; `Final/OT` when `Period > 4` |
| `pre`, kickoff same local calendar day as `now` | `GB vs DET · 1:00 PM` (favorite home) / `GB @ DET · 1:00 PM` (away) |
| `pre`, other day | `GB @ DET · Sun 1:00 PM` |
| `pre` and `Status.Type.Description` is `Postponed` or `Canceled` | `GB vs DET · Postponed` using `ShortDetail` |

Time layouts: `3:04 PM` and `Mon 3:04 PM`, in `now.Location()`.

- [ ] **Step 1: Write the failing table test**

```go
func TestStatusLine(t *testing.T) {
	now := time.Date(2026, 11, 1, 14, 30, 0, 0, time.FixedZone("EST", -5*3600))
	live := func(state, clock string, period int, desc, short string, awayScore, homeScore, possession string, rz bool) scoreboard {
		return scoreboard{Events: []game{{ID: "1", Date: "2026-11-01T18:00Z",
			Status: gameStatus{Type: statusType{State: state, ShortDetail: short, Description: desc}, DisplayClock: clock, Period: period},
			Competitions: []competition{{Situation: gameSituation{Possession: possession, IsRedZone: rz},
				Competitors: []competitor{{HomeAway: "away", Score: awayScore, Team: teamRef{ID: "29", Abbreviation: "CAR"}}, {HomeAway: "home", Score: homeScore, Team: teamRef{ID: "8", Abbreviation: "GB"}}}}}}}}
	}
	cases := []struct{ name string; board scoreboard; cachedAt time.Time; want string }{
		{"live ball", live("in", "2:14", 4, "In Progress", "2:14 - 4th", "19", "27", "8", false), time.Time{}, "GB 27-19 CAR · Q4 2:14 · BALL"},
		{"live red zone", live("in", "2:14", 4, "In Progress", "2:14 - 4th", "19", "27", "8", true), time.Time{}, "GB 27-19 CAR · Q4 2:14 · BALL · RZ"},
		{"opponent ball no rz", live("in", "2:14", 4, "In Progress", "2:14 - 4th", "19", "27", "29", true), time.Time{}, "GB 27-19 CAR · Q4 2:14"},
		{"overtime", live("in", "8:00", 5, "In Progress", "8:00 - OT", "27", "27", "", false), time.Time{}, "GB 27-27 CAR · OT 8:00"},
		{"halftime", live("in", "0:00", 2, "Halftime", "Halftime", "10", "17", "", false), time.Time{}, "GB 17-10 CAR · Halftime"},
		{"missing score", live("in", "14:55", 1, "In Progress", "14:55 - 1st", "", "7", "", false), time.Time{}, "GB 7-0 CAR · Q1 14:55"},
		{"final win", live("post", "0:00", 4, "Final", "Final", "19", "27", "", false), time.Time{}, "GB W 27-19 CAR · Final"},
		{"final loss ot", live("post", "0:00", 5, "Final", "Final/OT", "30", "27", "", false), time.Time{}, "GB L 27-30 CAR · Final/OT"},
		{"tie", live("post", "0:00", 5, "Final", "Final/OT", "27", "27", "", false), time.Time{}, "GB T 27-27 CAR · Final/OT"},
		{"stale cache", live("in", "2:14", 4, "In Progress", "2:14 - 4th", "19", "27", "", false), now.Add(-6 * time.Minute), "GB 27-19 CAR · Q4 2:14 (cached)"},
		{"fresh cache", live("in", "2:14", 4, "In Progress", "2:14 - 4th", "19", "27", "", false), now.Add(-20 * time.Second), "GB 27-19 CAR · Q4 2:14"},
	}
	for _, tc := range cases {
		if got := statusLine(tc.board, "GB", now, tc.cachedAt); got != tc.want {
			t.Errorf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
}

func TestStatusLineScheduled(t *testing.T) {
	now := time.Date(2026, 11, 1, 9, 0, 0, 0, time.FixedZone("EST", -5*3600))
	pre := func(date, desc, short string, favHome bool) scoreboard { /* favorite GB vs DET, state "pre" */ }
	cases := []struct{ name string; board scoreboard; want string }{
		{"today home", pre("2026-11-01T18:00Z", "Scheduled", "11/1 - 1:00 PM EST", true), "GB vs DET · 1:00 PM"},
		{"other day away", pre("2026-11-08T18:00Z", "Scheduled", "11/8 - 1:00 PM EST", false), "GB @ DET · Sun 1:00 PM"},
		{"postponed", pre("2026-11-01T18:00Z", "Postponed", "Postponed", true), "GB vs DET · Postponed"},
	}
	// assert as above
}

func TestStatusLineNoGame(t *testing.T) {
	// bye: board with only CAR @ BUF; expect ""
	// duplicate favorite: two GB events, first by kickoff wins; expect line built from the first
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run 'TestStatusLine' ./...`
Expected: compile error, `undefined: statusLine`

- [ ] **Step 3: Implement `favoriteGame`, `statusLine`, `periodLabel`, `scoreOrZero` in `status.go`; point `favoriteMatch` at `favoriteGame`**

- [ ] **Step 4: Run the full suite**

Run: `go test ./...`
Expected: `ok  	touchdown`

- [ ] **Step 5: Commit**

```bash
git add status.go status_test.go dashboard.go
git commit -m "Format a one-line favorite-team status from the scoreboard"
```

---

### Task 3: Prompt mode with cache read and detached refresh

**Files:**
- Create: `refresh.go`, `detach_unix.go` (`//go:build !windows`), `detach_windows.go` (`//go:build windows`)
- Modify: `espn.go:143-161` (extract URL builder), `main.go:756-840` (flags and dispatch)
- Test: `status_test.go`

**Interfaces:**
- Produces:
  - `func scoreboardURL(season, week, seasonType int) string` in `espn.go`; `client.scoreboard` uses it so cache keys match. Current week: `apiBase + "/scoreboard"`; otherwise with the existing `dates`, `seasontype`, `week` query.
  - `func cachedScoreboard(cacheDir string, season, week, seasonType int) (scoreboard, time.Time, bool)` in `refresh.go`: `readCached` on `scoreboardURL`; returns `false` on any error. Sorts events by kickoff like `client.scoreboard`.
  - `func refreshDue(cacheDir string, now time.Time, cachedAt time.Time, hasCache bool) bool`: true when `!hasCache || now.Sub(cachedAt) > 30*time.Second`, and marker `filepath.Join(cacheDir, "status-refresh")` is missing or older than 30 s. When true, creates the dir (`0700`) and touches the marker before returning.
  - `var spawnDetached = func(self string, args []string) error` (package var, swapped in tests): `exec.Command(self, args...)` with stdin/stdout/stderr nil, `cmd.SysProcAttr = detachAttr()`, `cmd.Start()`, then `cmd.Process.Release()`.
  - `func detachAttr() *syscall.SysProcAttr`: unix `&syscall.SysProcAttr{Setsid: true}`; windows `&syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | 0x00000008}` (0x8 is `DETACHED_PROCESS`).
  - `func forwardedFlags() []string`: via `flag.Visit`, re-emits `-favorite`, `-config`, `-season`, `-week`, `-type` as `-name=value` pairs when set.
  - `func runStatusOnce(cacheDir, favorite string, season, week, seasonType int, now time.Time, self string, forwarded []string, out io.Writer) int`: prints `statusLine` (plus newline when non-empty), spawns refresh when due, returns exit code 0. Spawn errors go to stderr, still exit 0.
  - `func runRefresh(api *client, season, week, seasonType int) int`: `api.scoreboard` with `context.WithTimeout(3*time.Second)`; 0 on success, 1 on error (message to stderr).
- `main.go`: new flags `status`, `watch`, `split`, `refresh` (all `bool`). `-refresh` help text: `refresh the status cache and exit (used by -status)`. Flag-combination validation is Task 6. `-theme` is parsed and validated as today, then ignored in status mode. Status mode client: `newClient()` then `api.http.Timeout = 3 * time.Second`.

- [ ] **Step 1: Write failing tests**

```go
func TestScoreboardURLMatchesCacheKey(t *testing.T) {
	// client with a transport that records req.URL.String(); call scoreboard(ctx,2026,7,2) and (ctx,0,0,2)
	// assert recorded URLs equal scoreboardURL(2026,7,2) and scoreboardURL(0,0,2)
}

func TestRunStatusOnceReadsCacheWithoutNetwork(t *testing.T) {
	dir := t.TempDir()
	body := /* live GB 27-19 CAR JSON from Task 1 */
	if err := writeCached(dir, scoreboardURL(0, 0, 2), []byte(body)); err != nil { t.Fatal(err) }
	var spawned [][]string
	spawnDetached = func(self string, args []string) error { spawned = append(spawned, append([]string{self}, args...)); return nil }
	t.Cleanup(func() { spawnDetached = realSpawnDetached })
	var out strings.Builder
	code := runStatusOnce(dir, "GB", 0, 0, 2, time.Now(), "/bin/td", []string{"-favorite=GB"}, &out)
	if code != 0 || out.String() != "GB 27-19 CAR · Q4 2:14 · BALL\n" { t.Fatalf("code %d out %q", code, out.String()) }
	if len(spawned) != 0 { t.Fatalf("fresh cache must not spawn: %v", spawned) }
}

func TestRunStatusOnceSpawnsRefreshWhenStale(t *testing.T) {
	// write cache, os.Chtimes it to 45 s ago; expect one spawn with args
	// []string{"/bin/td", "-status", "-refresh", "-favorite=GB"} and the line still printed
	// call again immediately: marker is fresh, so no second spawn
}

func TestRunStatusOnceNoCache(t *testing.T) {
	// empty dir: output "", code 0, one spawn, marker file exists under dir
	// dir that does not exist yet (filepath.Join(t.TempDir(), "nested")): same, dir created
}

func TestRunStatusOnceCorruptCache(t *testing.T) {
	// write "{not json" at cachePath(dir, scoreboardURL(0,0,2)): output "", code 0, one spawn
}

func TestRunRefreshWritesCache(t *testing.T) {
	// jsonClient with cacheDir = t.TempDir(); runRefresh(api,0,0,2) == 0; cachedScoreboard finds one event
}
```

Keep the real spawn in `var realSpawnDetached = func(...)` and `var spawnDetached = realSpawnDetached` so tests can restore it.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run 'TestScoreboardURL|TestRunStatus|TestRunRefresh' ./...`
Expected: compile errors for the undefined names

- [ ] **Step 3: Implement `scoreboardURL`, `refresh.go`, the two detach files, and the `main.go` flag wiring**

Dispatch order in `main` after theme and week validation: `if *status && *refresh { os.Exit(runRefresh(...)) }`, then `if *status && !*watch && !*split { os.Exit(runStatusOnce(...)) }`. `-watch` and `-split` are wired in Tasks 4 and 5; until then they fall through to the TUI.

- [ ] **Step 4: Run the full suite and a manual check**

Run: `go test ./... && go build -o bin/touchdown . && ./bin/touchdown -status; echo "exit $?"; sleep 2; ./bin/touchdown -status`
Expected: tests `ok`; first call prints nothing (or a line if the TUI cache is warm) with `exit 0`; second call prints the favorite line.

- [ ] **Step 5: Commit**

```bash
git add espn.go refresh.go detach_unix.go detach_windows.go main.go status_test.go
git commit -m "Add td -status with a cache-only prompt path and detached refresh"
```

---

### Task 4: Ticker and watch loop

**Files:**
- Create: `ticker.go`
- Modify: `main.go` (dispatch `-watch`), `go.mod` (`github.com/charmbracelet/x/term` to direct require via `go mod tidy`)
- Test: `status_test.go`

**Interfaces:**
- Consumes: `favoriteGame`, `scoreOrZero`, `periodLabel` from Task 2.
- Produces:
  - `func tickerLines(board scoreboard, favorite string, now time.Time, width int, cached bool) []string`.
    Header `Week %d · updated %s` with `now.Format("15:04:05")`, plus ` · cached` when `cached`. Blank line, favorite game line, blank line, then in-progress games by kickoff, then `pre` by kickoff, then `post`. The favorite is not repeated in the later groups. If no favorite game, no favorite block.
    Game line: `fmt.Sprintf("%-4s %2s  %-4s %2s   %s", away, awayScore, home, homeScore, status)` for non-`pre`; `fmt.Sprintf("%-4s     %-4s      %s", away, home, statusText)` for `pre`. Live suffix: `   BALL GB · 3rd & 7` built with `downLabel` from `live.go` and `· RZ` when `IsRedZone`. Each line clipped to `width` runes.
  - `func terminalWidth(fallback int) int`: `term.GetSize(os.Stdout.Fd())` from `github.com/charmbracelet/x/term`; `fallback` on error. Fallback value 60.
  - `func runWatch(ctx context.Context, api *client, favorite string, season, week, seasonType int, out io.Writer, interval time.Duration) int`: loop until `ctx.Done()`: fetch, on success clear (`\033[H\033[2J`) and print lines; on error print `fetch failed: <err>` to stderr and keep the last screen; sleep `interval` or until cancelled. Returns 0. `main` wires `signal.NotifyContext(context.Background(), os.Interrupt)` and `interval = 30*time.Second`.

- [ ] **Step 1: Write failing tests**

```go
func TestTickerLinesOrderAndShape(t *testing.T) {
	// board: BAL@BUF in (Q3 8:01), GB vs CAR in with GB ball 3rd & 7 RZ, PHI@WSH pre, KC@DEN post
	// want[0] == "Week 9 · updated 14:32:07", want[1] == "", want[2] starts with "CAR  19  GB   27   " and ends with "BALL GB · 3rd & 7 · RZ"
	// want[3] == "", then BAL@BUF, then PHI@WSH pre line, then KC@DEN post line; GB appears once
}

func TestTickerLinesClipToWidth(t *testing.T) {
	// width 20: every line len([]rune(l)) <= 20
}

func TestTickerLinesCachedHeader(t *testing.T) {
	// cached=true: header ends with " · cached"
}

func TestRunWatchStopsOnCancel(t *testing.T) {
	// jsonClient; ctx with cancel; interval 10ms; cancel after 35ms; expect return 0 and at least 2 redraws (count "\033[H\033[2J" in out)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run 'TestTicker|TestRunWatch' ./...`
Expected: `undefined: tickerLines`

- [ ] **Step 3: Implement `ticker.go` and dispatch `-status -watch` in `main`; run `go mod tidy`**

- [ ] **Step 4: Run the full suite and a manual check**

Run: `go test ./... && go vet ./... && go run . -status -watch`
Expected: tests `ok`; ticker redraws; Ctrl-C exits silently with status 0 (`echo $?`).

- [ ] **Step 5: Commit**

```bash
git add ticker.go main.go go.mod go.sum status_test.go
git commit -m "Add td -status -watch, an in-place scoreboard ticker"
```

---

### Task 5: Open the ticker in a pane

**Files:**
- Create: `split.go`
- Modify: `main.go` (dispatch `-split`)
- Test: `status_test.go`

**Interfaces:**
- Consumes: `forwardedFlags` from Task 3.
- Produces:
  - `func detectHost(getenv func(string) string) string`: checks in order `TMUX` → `"tmux"`, `ZELLIJ` → `"zellij"`, `HERDR_ENV` == `"1"` → `"herdr"`, `KITTY_PID` → `"kitty"`, `WEZTERM_PANE` → `"wezterm"`, else `""`.
  - `func shellQuote(args []string) string`: each arg wrapped in single quotes with `'` replaced by `'\''`, joined by spaces.
  - `var runHostCommand = func(name string, args ...string) ([]byte, error)`: `exec.Command(name, args...).Output()`; swapped in tests.
  - `func openSplit(host, self string, forwarded []string, errOut io.Writer) int`: child argv is `append([]string{self, "-status", "-watch"}, forwarded...)`. Per host:
    - tmux: `tmux split-window -h -l 36 -d <child argv...>`
    - zellij: `zellij run -d right -n NFL -- <child argv...>`
    - kitty: `kitten @ launch --location=vsplit --keep-focus <child argv...>`
    - wezterm: `wezterm cli split-pane --right --percent 35 -- <child argv...>`
    - herdr: `herdr pane split --current --direction right --ratio 0.35 --no-focus`; parse stdout JSON `{"result":{"pane":{"pane_id":"..."}}}`; empty id is an error; then `herdr pane run <id> <shellQuote(child argv)>`
    - `""`: print `Run this in a new pane:` and the shell-quoted child argv to `errOut`, return 1.
    Any command error: print `<host> failed: <err>` and the command's stderr (from `*exec.ExitError`), then the manual message, return 1. Success returns 0.

- [ ] **Step 1: Write failing tests**

```go
func TestDetectHostPrefersInnermost(t *testing.T) {
	env := map[string]string{"TMUX": "/tmp/tmux-1000/default,1,0", "KITTY_PID": "4242"}
	if got := detectHost(func(k string) string { return env[k] }); got != "tmux" { t.Fatal(got) }
	// also: only HERDR_ENV=1 → "herdr"; HERDR_ENV="0" → ""; only WEZTERM_PANE → "wezterm"; empty → ""
}

func TestShellQuote(t *testing.T) {
	if got := shellQuote([]string{"/opt/my td/td", "-favorite=Green Bay", "it's"}); got != `'/opt/my td/td' '-favorite=Green Bay' 'it'\''s'` { t.Fatal(got) }
}

func TestOpenSplitCommands(t *testing.T) {
	var calls [][]string
	runHostCommand = func(name string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string{name}, args...))
		if name == "herdr" && args[1] == "split" { return []byte(`{"result":{"pane":{"pane_id":"w1:p3"}}}`), nil }
		return nil, nil
	}
	t.Cleanup(func() { runHostCommand = realRunHostCommand })
	cases := []struct{ host string; want [][]string }{
		{"tmux", {{"tmux", "split-window", "-h", "-l", "36", "-d", "/bin/td", "-status", "-watch", "-favorite=GB"}}},
		{"zellij", {{"zellij", "run", "-d", "right", "-n", "NFL", "--", "/bin/td", "-status", "-watch", "-favorite=GB"}}},
		{"kitty", {{"kitten", "@", "launch", "--location=vsplit", "--keep-focus", "/bin/td", "-status", "-watch", "-favorite=GB"}}},
		{"wezterm", {{"wezterm", "cli", "split-pane", "--right", "--percent", "35", "--", "/bin/td", "-status", "-watch", "-favorite=GB"}}},
		{"herdr", {{"herdr", "pane", "split", "--current", "--direction", "right", "--ratio", "0.35", "--no-focus"}, {"herdr", "pane", "run", "w1:p3", "'/bin/td' '-status' '-watch' '-favorite=GB'"}}},
	}
	// for each: calls = nil; code := openSplit(host, "/bin/td", []string{"-favorite=GB"}, io.Discard); assert code 0 and reflect.DeepEqual(calls, want)
}

func TestOpenSplitFailures(t *testing.T) {
	// unknown host: code 1, errOut contains "Run this in a new pane:" and "'/bin/td' '-status' '-watch'"
	// herdr split returns {"error":"no focused pane"} with nil err: code 1, no "pane run" call, errOut contains "Run this in a new pane:"
	// tmux returns &exec.ExitError{}: code 1, errOut contains "tmux failed"
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -run 'TestDetectHost|TestShellQuote|TestOpenSplit' ./...`
Expected: `undefined: detectHost`

- [ ] **Step 3: Implement `split.go`; in `main`, `-status -split` resolves `os.Executable()` and calls `openSplit(detectHost(os.Getenv), self, forwardedFlags(), os.Stderr)`**

- [ ] **Step 4: Run the full suite and a manual check in Zellij or Kitty (both installed here)**

Run: `go test ./... && go vet ./... && go run . -status -split`
Expected: tests `ok`; inside Zellij a right pane opens with the ticker; in a plain terminal the manual command prints and exit is 1.

- [ ] **Step 5: Commit**

```bash
git add split.go main.go status_test.go
git commit -m "Add td -status -split to open the ticker in a multiplexer pane"
```

---

### Task 6: Flag validation and README

**Files:**
- Modify: `main.go` (combination checks), `README.md` (after "Command-line options")
- Test: `status_test.go`

**Interfaces:**
- Produces: `func validateStatusFlags(status, watch, split, refresh, once, notify, compact bool) error` in `status.go`; `main` prints the error and exits 2. Messages: `-watch, -split and -refresh require -status` when any of the three is set without `status`; `-status cannot be combined with -once, -notify, -notify-all or -compact` when `status` is set with any of those (`notify` covers `-notify-all`, which implies it).

- [ ] **Step 1: Write the failing test**

```go
func TestValidateStatusFlags(t *testing.T) {
	cases := []struct{ status, watch, split, refresh, once, notify, compact bool; wantErr bool }{
		{true, false, false, false, false, false, false, false},
		{true, true, false, false, false, false, false, false},
		{true, true, true, false, false, false, false, false},
		{false, true, false, false, false, false, false, true},
		{false, false, false, true, false, false, false, true},
		{true, false, false, false, true, false, false, true},
		{true, false, false, false, false, true, false, true},
		{true, false, false, false, false, false, true, true},
		{false, false, false, false, true, false, false, false},
	}
	// assert (err != nil) == wantErr
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test -run TestValidateStatusFlags ./...`
Expected: `undefined: validateStatusFlags`

- [ ] **Step 3: Implement and wire it before any `-status` dispatch in `main`**

- [ ] **Step 4: Write the README section**

Insert a `## Status line` section after "Command-line options" with, in order:

1. One paragraph: `td -status` prints one line for a shell prompt, never waits on the network, first run after install prints nothing until the background refresh lands, ` (cached)` means data older than five minutes.
2. The Starship block exactly as in the spec (`[custom.nfl]`, `shell = ["sh", "--norc"]`).
3. `### Ticker pane`: `td -status -watch` and `td -status -split`, then this table:

| Host | What `-split` does |
| --- | --- |
| tmux | `tmux split-window -h -l 36` |
| Zellij | `zellij run -d right` |
| herdr | `herdr pane split --current --direction right`, then `herdr pane run` |
| Kitty | `kitten @ launch --location=vsplit` (needs `allow_remote_control yes`) |
| WezTerm | `wezterm cli split-pane --right --percent 35` |
| Anything else | prints the command to run in a pane you open yourself |

4. The herdr `[[keys.command]]` block from the spec.
5. Add `td -status`, `td -status -watch`, `td -status -split` lines to the "Command-line options" code block.

- [ ] **Step 5: Run the full suite, vet, and the help output**

Run: `go test ./... && go vet ./... && go run . -h 2>&1 | grep -E 'status|watch|split|refresh'`
Expected: tests `ok`; four flags listed.

- [ ] **Step 6: Commit**

```bash
git add main.go status.go status_test.go README.md
git commit -m "Validate status flags and document the status line and ticker pane"
```
