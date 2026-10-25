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

func TestFavoriteConfigRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "touchdown", "config.json")
	initial, err := loadConfig(path)
	if err != nil || initial.FavoriteTeam != "GB" {
		t.Fatalf("default config: %+v %v", initial, err)
	}
	team, err := saveFavorite(path, "Buffalo Bills")
	if err != nil || team.abbreviation != "BUF" {
		t.Fatalf("saving Bills: %+v %v", team, err)
	}
	loaded, err := loadConfig(path)
	if err != nil || loaded.FavoriteTeam != "BUF" {
		t.Fatalf("loading Bills: %+v %v", loaded, err)
	}
	if favoriteLabel(loaded.FavoriteTeam) != "BILLS" {
		t.Fatal("Bills label missing")
	}
	if _, err := saveFavorite(path, "not a team"); err == nil {
		t.Fatal("invalid team accepted")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil || fields["favorite_team"] != "BUF" {
		t.Fatalf("invalid saved config: %s %v", data, err)
	}
}

func TestEveryFavoriteOpensGameAndSeason(t *testing.T) {
	for _, team := range nflTeams {
		t.Run(team.abbreviation, func(t *testing.T) {
			var board scoreboard
			raw := fmt.Sprintf(`{"events":[{"id":"favorite","status":{"type":{"state":"pre"}},"competitions":[{"competitors":[{"homeAway":"away","team":{"abbreviation":%q,"name":%q}},{"homeAway":"home","team":{"abbreviation":"OPP"}}]}]}]}`, team.abbreviation, team.nickname)
			if err := json.Unmarshal([]byte(raw), &board); err != nil {
				t.Fatal(err)
			}
			for _, size := range []struct{ width, height int }{{80, 24}, {150, 36}} {
				m := model{board: board, favorite: team.abbreviation, season: 2026, week: 7, seasonType: 2, showHome: true, width: size.width, height: size.height}
				view := m.View()
				if !strings.Contains(view, strings.ToUpper(team.nickname)+" WATCH") || !strings.Contains(view, strings.ToUpper(team.nickname)+" SEASON") {
					t.Fatalf("favorite menu missing for %s", team.abbreviation)
				}
				for _, line := range strings.Split(view, "\n") {
					if lipgloss.Width(line) > size.width {
						t.Fatalf("home exceeds %d columns", size.width)
					}
				}
				if strings.Count(view, "\n")+1 > size.height {
					t.Fatalf("home exceeds %d rows", size.height)
				}
				m.showHome = false
				view = m.View()
				for _, line := range strings.Split(view, "\n") {
					if lipgloss.Width(line) > size.width {
						t.Fatalf("dashboard exceeds %d columns", size.width)
					}
				}
				if strings.Count(view, "\n")+1 > size.height {
					t.Fatalf("dashboard exceeds %d rows", size.height)
				}
			}
			m := model{board: board, favorite: team.abbreviation, season: 2026, week: 7, showHome: true}
			next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
			if !next.(model).showTeam {
				t.Fatalf("%s season key p failed", team.abbreviation)
			}
			next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
			if !next.(model).showDetail || next.(model).detailID != "favorite" {
				t.Fatalf("%s game key failed", team.abbreviation)
			}
			if conference, _ := m.favoriteConference(); conference != team.conference {
				t.Fatalf("%s conference is %s", team.abbreviation, conference)
			}
		})
	}
}

func TestTeamCatalogHasUniqueResolvableTeams(t *testing.T) {
	if len(nflTeams) != 32 {
		t.Fatalf("expected 32 NFL teams, got %d", len(nflTeams))
	}
	seen := map[string]bool{}
	for _, team := range nflTeams {
		if seen[team.abbreviation] {
			t.Fatalf("duplicate %s", team.abbreviation)
		}
		seen[team.abbreviation] = true
		resolved, err := resolveTeam(team.city + " " + team.nickname)
		if err != nil || resolved.abbreviation != team.abbreviation {
			t.Fatalf("cannot resolve %s %s: %v", team.city, team.nickname, err)
		}
	}
}

func TestBillsFavoriteUpdatesViewsAndControls(t *testing.T) {
	var board scoreboard
	if err := json.Unmarshal([]byte(`{"events":[{"id":"buf","status":{"type":{"state":"pre"}},"competitions":[{"competitors":[{"homeAway":"away","team":{"abbreviation":"BUF","name":"Bills"}},{"homeAway":"home","team":{"abbreviation":"MIA","name":"Dolphins"}}]}]},{"id":"other","status":{"type":{"state":"pre"}},"competitions":[{"competitors":[{"homeAway":"away","team":{"abbreviation":"GB"}},{"homeAway":"home","team":{"abbreviation":"CHI"}}]}]}]}`), &board); err != nil {
		t.Fatal(err)
	}
	m := model{board: board, favorite: "BUF", season: 2026, week: 7, seasonType: 2, width: 80, height: 24, showHome: true}
	if view := m.View(); !strings.Contains(view, "BILLS WATCH") || !strings.Contains(view, "BILLS SEASON") {
		t.Fatalf("Bills home missing:\n%s", view)
	}
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = next.(model)
	if !m.showTeam || !m.teamFromHome {
		t.Fatal("p did not open Bills season")
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = next.(model)
	if !m.showHome {
		t.Fatal("Esc did not return home")
	}
	m.homeSelection = 2
	m, _ = m.openHomeSelection()
	if !m.favoritesOnly || len(m.visibleGames()) != 1 || m.visibleGames()[0].ID != "buf" {
		t.Fatal("Bills watch did not filter games")
	}
	if view := m.View(); !strings.Contains(view, "BILLS WATCH") || !strings.Contains(view, "p season") {
		t.Fatalf("Bills dashboard missing:\n%s", view)
	}
	if conference, _ := m.favoriteConference(); conference != "AFC" {
		t.Fatalf("Bills conference: %s", conference)
	}
	if team, err := resolveTeam("Bills"); err != nil || team.abbreviation != "BUF" {
		t.Fatalf("Bills name lookup: %+v %v", team, err)
	}
}

func TestFavoriteGameShortcutFromHomeAndFilteredBoard(t *testing.T) {
	var board scoreboard
	if err := json.Unmarshal([]byte(`{"events":[{"id":"chiefs","status":{"type":{"state":"pre"}},"competitions":[{"competitors":[{"homeAway":"away","team":{"abbreviation":"KC"}},{"homeAway":"home","team":{"abbreviation":"SEA"}}]}]},{"id":"live","status":{"type":{"state":"in"}},"competitions":[{"competitors":[{"homeAway":"away","team":{"abbreviation":"NYG"}},{"homeAway":"home","team":{"abbreviation":"HOU"}}]}]}]}`), &board); err != nil {
		t.Fatal(err)
	}
	home := model{board: board, favorite: "KC", season: 2026, week: 7, showHome: true}
	next, _ := home.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	opened := next.(model)
	if !opened.showDetail || opened.detailID != "chiefs" || !opened.detailFromHome {
		t.Fatalf("g failed from home: %+v", opened)
	}
	next, _ = opened.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !next.(model).showHome {
		t.Fatal("Esc did not return to home")
	}
	filtered := model{board: board, favorite: "KC", season: 2026, week: 7, liveOnly: true}
	next, _ = filtered.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	opened = next.(model)
	if !opened.showDetail || opened.detailID != "chiefs" {
		t.Fatalf("g failed through live filter: %+v", opened)
	}
	bye := model{board: board, favorite: "BUF", season: 2026, week: 7, showHome: true}
	next, _ = bye.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	fallback := next.(model)
	if !fallback.showTeam || !fallback.teamFromHome || !strings.Contains(fallback.teamNotice, "No BILLS game") {
		t.Fatalf("bye fallback missing: %+v", fallback)
	}
	loading := model{favorite: "KC", showHome: true, loading: true}
	next, _ = loading.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	queued := next.(model)
	if !queued.pendingFavoriteGame || queued.showDetail {
		t.Fatal("g did not wait for first score load")
	}
	next, _ = queued.Update(boardMsg{board: board})
	opened = next.(model)
	if !opened.showDetail || opened.detailID != "chiefs" {
		t.Fatal("queued g did not open Chiefs game")
	}
}
