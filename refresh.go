package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// refreshInterval is how often prompt mode lets a background refresh run.
const refreshInterval = 30 * time.Second

// refreshTimeout bounds the network call made by the refresh child and the
// watch loop.
const refreshTimeout = 3 * time.Second

// cachedScoreboard reads a week's scoreboard from the cache without touching
// the network. It reports false when nothing usable is cached.
func cachedScoreboard(cacheDir string, season, week, seasonType int) (scoreboard, time.Time, bool) {
	var board scoreboard
	cachedAt, err := readCached(cacheDir, scoreboardURL(season, week, seasonType), &board)
	if err != nil {
		return scoreboard{}, time.Time{}, false
	}
	sortByKickoff(board.Events)
	board.CachedAt = cachedAt
	return board, cachedAt, true
}

// refreshDue reports whether prompt mode should start a background refresh:
// the cache is missing or older than refreshInterval, and no attempt has
// been made within refreshInterval. When it returns true it has already
// touched the marker file so concurrent prompts do not also spawn.
func refreshDue(cacheDir string, now time.Time, cachedAt time.Time, hasCache bool) bool {
	if hasCache && now.Sub(cachedAt) <= refreshInterval {
		return false
	}
	if cacheDir == "" {
		return false
	}
	marker := filepath.Join(cacheDir, "status-refresh")
	if info, err := os.Stat(marker); err == nil && now.Sub(info.ModTime()) <= refreshInterval {
		return false
	}
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
		return false
	}
	if err := os.WriteFile(marker, nil, 0600); err != nil {
		return false
	}
	return os.Chtimes(marker, now, now) == nil
}

var realSpawnDetached = func(self string, args []string) error {
	cmd := exec.Command(self, args...)
	cmd.SysProcAttr = detachAttr()
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// spawnDetached starts self with args as a background process that outlives
// this one. Tests replace it.
var spawnDetached = realSpawnDetached

// forwardedFlags re-emits the flags a status child must inherit, in
// -name=value form, for those the user set explicitly. A relative -config
// path is made absolute because a new pane may start in another directory.
func forwardedFlags(fs *flag.FlagSet) []string {
	var args []string
	fs.Visit(func(f *flag.Flag) {
		value := f.Value.String()
		switch f.Name {
		case "config":
			if abs, err := filepath.Abs(value); err == nil {
				value = abs
			}
		case "favorite", "season", "week", "type":
		default:
			return
		}
		args = append(args, "-"+f.Name+"="+value)
	})
	return args
}

// runStatusOnce prints the prompt status line from the cache and, when the
// cache is stale, hands fetching to a detached refresh child.
// liveOnly prints the line only while the favorite's game is in progress,
// so a prompt segment appears during games and disappears otherwise.
func runStatusOnce(cacheDir, favorite string, season, week, seasonType int, now time.Time, self string, forwarded []string, out io.Writer, liveOnly bool) int {
	board, cachedAt, ok := cachedScoreboard(cacheDir, season, week, seasonType)
	if ok {
		g, found := favoriteGame(board, favorite)
		if line := statusLine(board, favorite, now, cachedAt); line != "" && (!liveOnly || found && g.Status.Type.State == "in") {
			fmt.Fprintln(out, line)
		}
	}
	if refreshDue(cacheDir, now, cachedAt, ok) {
		args := append([]string{"-status", "-refresh"}, forwarded...)
		if err := spawnDetached(self, args); err != nil {
			fmt.Fprintln(os.Stderr, "status refresh failed to start:", err)
		}
	}
	return 0
}

// runRefresh fetches one scoreboard so the cache is warm for the next prompt.
func runRefresh(api *client, season, week, seasonType int) int {
	ctx, cancel := context.WithTimeout(context.Background(), refreshTimeout)
	defer cancel()
	if _, err := api.scoreboard(ctx, season, week, seasonType); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
