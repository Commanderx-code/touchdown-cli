package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func key(m model, name string) model {
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(name)}
	if name == "enter" {
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	}
	if name == "esc" {
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	}
	next, _ := m.Update(msg)
	return next.(model)
}

func assertFits(t *testing.T, view string, width, height int) {
	t.Helper()
	lines := strings.Split(view, "\n")
	if len(lines) > height {
		t.Fatalf("view has %d rows, height %d", len(lines), height)
	}
	for _, line := range lines {
		if lipgloss.Width(line) > width {
			t.Fatalf("view exceeds %d columns: %q", width, line)
		}
	}
}

func TestSeasonScheduleOpensAnyGameAndReturns(t *testing.T) {
	var schedule teamSchedule
	if err := json.Unmarshal([]byte(`{"team":{"displayName":"Buffalo Bills"},"events":[{"id":"old","date":"2026-09-13T17:00Z","week":{"number":1},"competitions":[{"status":{"type":{"state":"post","shortDetail":"Final"}},"competitors":[{"homeAway":"away","score":{"displayValue":"21"},"team":{"abbreviation":"BUF","displayName":"Buffalo Bills"}},{"homeAway":"home","score":{"displayValue":"17"},"team":{"abbreviation":"NYJ","displayName":"New York Jets"}}]}]},{"id":"next","date":"2026-09-20T17:00Z","week":{"number":2},"competitions":[{"status":{"type":{"state":"pre"}},"competitors":[{"homeAway":"home","team":{"abbreviation":"BUF"}},{"homeAway":"away","team":{"abbreviation":"MIA"}}]}]}]}`), &schedule); err != nil {
		t.Fatal(err)
	}
	m := model{favorite: "BUF", season: 2026, teamSeason: 2026, teamData: schedule, showTeam: true, width: 80, height: 24}
	if !strings.Contains(m.View(), "› W01") {
		t.Fatal("first matchup is not selected")
	}
	m = key(m, "enter")
	if !m.showDetail || !m.detailFromTeam || m.detailID != "old" {
		t.Fatal("Enter did not open historical game")
	}
	if view := m.View(); !strings.Contains(view, "BUF") || !strings.Contains(view, "NYJ") {
		t.Fatalf("historical game header missing: %s", view)
	}
	// The summary parser supplies full quarter lines for historical games.
	var header competition
	header.Competitors = []competitor{{HomeAway: "away", Score: "21"}, {HomeAway: "home", Score: "17"}}
	header.Competitors[0].Team.Abbreviation = "BUF"
	header.Competitors[1].Team.Abbreviation = "NYJ"
	header.Competitors[0].Linescores = append(header.Competitors[0].Linescores, struct {
		DisplayValue string `json:"displayValue"`
	}{"7"})
	header.Competitors[1].Linescores = append(header.Competitors[1].Linescores, struct {
		DisplayValue string `json:"displayValue"`
	}{"3"})
	next, _ := m.Update(detailMsg{id: "old", detail: gameDetail{Header: []competition{header}}})
	m = next.(model)
	m.detailTab = 3
	if !strings.Contains(m.View(), "QUARTER SCORES") || !strings.Contains(m.View(), "BUF") {
		t.Fatal("historical quarter scores unavailable")
	}
	m = key(m, "esc")
	if !m.showTeam || m.showDetail {
		t.Fatal("Esc did not return to season schedule")
	}
	m = m.scrollBy(1)
	if m.teamSelected != 1 || !strings.Contains(m.View(), "› W02") {
		t.Fatal("second matchup cannot be selected")
	}
	long := model{favorite: "BUF", season: 2026, teamSeason: 2026, showTeam: true, width: 80, height: 24}
	for i := 1; i <= 17; i++ {
		game := scheduledGame{ID: fmt.Sprintf("game-%d", i)}
		game.Week.Number = i
		long.teamData.Events = append(long.teamData.Events, game)
	}
	long = long.scrollBy(16)
	if long.teamSelected != 16 || long.teamScroll == 0 || !strings.Contains(long.View(), "› W17") {
		t.Fatal("last season game is not reachable")
	}
}

func TestHelpFitsAndReturns(t *testing.T) {
	for _, size := range []struct{ w, h int }{{80, 24}, {60, 16}, {40, 10}} {
		m := model{showHome: true, favorite: "GB", width: size.w, height: size.h}
		m = key(m, "?")
		if !m.showHelp || !strings.Contains(m.View(), "TOUCHDOWN HELP") && !strings.Contains(m.View(), "TOUCHDOWN GUIDE") {
			t.Fatal("help did not open")
		}
		assertFits(t, m.View(), size.w, size.h)
		m = key(m, "esc")
		if m.showHelp || !m.showHome {
			t.Fatal("help did not return to home")
		}
	}
}

func TestSettingsPreviewCancelAndSave(t *testing.T) {
	defer applyTheme("mint")
	path := filepath.Join(t.TempDir(), "config.json")
	if err := saveSettings(path, "GB", "mint"); err != nil {
		t.Fatal(err)
	}
	m := model{showHome: true, favorite: "GB", theme: "mint", configPath: path, width: 80, height: 24}
	m = key(m, "c")
	if !m.showSettings {
		t.Fatal("settings did not open")
	}
	assertFits(t, m.View(), 80, 24)
	m.width, m.height = 150, 36
	assertFits(t, m.View(), 150, 36)
	m.width, m.height = 80, 24
	for i := 0; i < 8; i++ {
		m = key(m, "k")
	}
	if nflTeams[m.settingsTeam].abbreviation != "BUF" {
		t.Fatal("team picker did not reach Bills")
	}
	m = key(m, "l")
	if themeNames[m.settingsTheme] != "amber" {
		t.Fatal("theme did not advance")
	}
	m = key(m, "esc")
	config, err := loadConfig(path)
	if err != nil || config.FavoriteTeam != "GB" || config.Theme != "mint" || m.favorite != "GB" {
		t.Fatalf("cancel changed settings: %+v %+v %v", config, m, err)
	}
	m = key(m, "c")
	for i := 0; i < 8; i++ {
		m = key(m, "k")
	}
	m = key(m, "l")
	m = key(m, "enter")
	config, err = loadConfig(path)
	if err != nil || config.FavoriteTeam != "BUF" || config.Theme != "amber" || m.favorite != "BUF" || m.theme != "amber" || m.showSettings {
		t.Fatalf("save failed: %+v %+v %v", config, m, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), `"theme": "amber"`) {
		t.Fatalf("theme not persisted: %s %v", data, err)
	}
	if !strings.Contains(m.View(), "BILLS WATCH") {
		t.Fatal("favorite UI did not update")
	}
	m.showHome = false
	m.showDetail = true
	m.detailID = "old"
	m = key(m, "c")
	m = key(m, "esc")
	if !m.showDetail || m.showSettings {
		t.Fatal("cancel did not return to game detail")
	}
	m = key(m, "c")
	m = key(m, "enter")
	if !m.showHome || m.showDetail || m.showSettings {
		t.Fatal("saving from detail did not return to home")
	}
}

func TestLiveGameHeaderShowsPossessionAndLatestPlay(t *testing.T) {
	var board scoreboard
	if err := json.Unmarshal([]byte(`{"events":[{"id":"live","status":{"type":{"state":"in","shortDetail":"8:44 - 3rd"}},"competitions":[{"situation":{"possession":"9","down":3,"distance":7,"lastPlay":{"text":"Jordan Love pass complete for 14 yards"}},"competitors":[{"homeAway":"away","score":"17","team":{"id":"9","abbreviation":"GB"}},{"homeAway":"home","score":"10","team":{"id":"16","abbreviation":"MIN"}}]}]}]}`), &board); err != nil {
		t.Fatal(err)
	}
	m := model{board: board, detailID: "live", detailGame: board.Events[0], showDetail: true, width: 80, height: 24}
	view := m.View()
	for _, part := range []string{"BALL GB", "3RD & 7", "LATEST", "Jordan Love"} {
		if !strings.Contains(view, part) {
			t.Fatalf("missing %q from live header: %s", part, view)
		}
	}
	assertFits(t, view, 80, 24)
	var schedule teamSchedule
	if err := json.Unmarshal([]byte(`{"events":[{"id":"live","week":{"number":7},"competitions":[{"status":{"type":{"state":"in"}},"competitors":[{"homeAway":"away","team":{"abbreviation":"GB"}},{"homeAway":"home","team":{"abbreviation":"MIN"}}]}]}]}`), &schedule); err != nil {
		t.Fatal(err)
	}
	fromSeason := model{board: board, teamData: schedule, teamSeason: 2026, season: 2026, favorite: "GB", showTeam: true, width: 80, height: 24}
	fromSeason = key(fromSeason, "enter")
	if !strings.Contains(fromSeason.View(), "BALL GB") || !fromSeason.detailFromTeam {
		t.Fatal("season selection lost live scoreboard situation")
	}
}
