package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// seedsStandings builds standings with the given teams seeded in order. Both
// conferences use the same teams, which is all these tests need.
func seedsStandings(t *testing.T, teams ...string) standings {
	t.Helper()
	var entries []string
	for i, team := range teams {
		entries = append(entries, fmt.Sprintf(`{"team":{"abbreviation":%q,"displayName":"%s Name\u0007"},"stats":[{"name":"playoffSeed","displayValue":"%d"},{"name":"overall","displayValue":"%d-%d"}]}`, team, team, i+1, 9-i, i))
	}
	group := `{"abbreviation":"%s","standings":{"entries":[` + strings.Join(entries, ",") + `]}}`
	body := `{"children":[` + fmt.Sprintf(group, "AFC") + "," + fmt.Sprintf(group, "NFC") + `]}`
	var out standings
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func playoffsModel(t *testing.T, s standings) model {
	t.Helper()
	return model{api: leagueClient(t, nil), season: 2026, week: 9, seasonType: 2, favorite: "GB", standings: s, standingsSeason: 2026, width: 100, height: 40, showPlayoffs: true}
}

const leadersJSON = `{"leaders":{"categories":[
 {"name":"kickoffYards","displayName":"Kickoff Yards","leaders":[{"displayValue":"9","athlete":{"id":"7","displayName":"Skip Me"},"team":{"abbreviation":"KC"}}]},
 {"name":"rushingYards","displayName":"Rushing Yards","leaders":[{"displayValue":"800","athlete":{"id":"3","displayName":"Josh Jacobs","position":{"abbreviation":"RB"}},"team":{"abbreviation":"GB"}}]},
 {"name":"passingYards","displayName":"Passing Yards","leaders":[
  {"displayValue":"2,246","athlete":{"id":"1","displayName":"Jordan Love\u0007","position":{"abbreviation":"QB"}},"team":{"abbreviation":"GB"}},
  {"displayValue":"2,100","athlete":{"id":"2","displayName":"Patrick Mahomes","position":{"abbreviation":"QB"}},"team":{"abbreviation":"KC"}}
 ]}
]}}`

const postseasonJSON = `{"events":[{"id":"9","date":"2027-01-10T18:00Z","status":{"type":{"state":"post","shortDetail":"Final"}},"competitions":[{"competitors":[{"homeAway":"away","score":"27","team":{"abbreviation":"GB"}},{"homeAway":"home","score":"24","team":{"abbreviation":"CHI"}}]}]}]}`

const tbdJSON = `{"events":[{"id":"8","date":"2027-01-09T18:00Z","status":{"type":{"state":"pre"}},"competitions":[{"competitors":[{"homeAway":"away","team":{"abbreviation":"TBD"}},{"homeAway":"home","team":{"abbreviation":"TBD"}}]}]}]}`

// leagueClient serves leaders, and a postseason whose first round is set.
func leagueClient(t *testing.T, seen *[]string) *client {
	t.Helper()
	return leagueClientWith(t, seen, postseasonJSON)
}

func leagueClientWith(t *testing.T, seen *[]string, wildCard string) *client {
	t.Helper()
	return &client{http: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if seen != nil {
			*seen = append(*seen, req.URL.Host+req.URL.Path+"?"+req.URL.RawQuery)
		}
		body := `{}`
		switch {
		case strings.HasSuffix(req.URL.Path, "/leaders"):
			body = leadersJSON
		case strings.HasSuffix(req.URL.Path, "/standings"):
			body = `{"children":[{"abbreviation":"AFC"}]}`
		case strings.HasSuffix(req.URL.Path, "/scoreboard"):
			body = tbdJSON
			if req.URL.Query().Get("week") == "1" {
				body = wildCard
			}
			body = strings.Replace(body, `{"events"`, `{"week":{"number":`+req.URL.Query().Get("week")+`},"events"`, 1)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}}
}

func TestConferenceSeedsSortByESPNSeedAndSkipUnseeded(t *testing.T) {
	var group standingGroup
	body := `{"standings":{"entries":[
	 {"team":{"abbreviation":"B"},"stats":[{"name":"playoffSeed","displayValue":"2"}]},
	 {"team":{"abbreviation":"X"},"stats":[]},
	 {"team":{"abbreviation":"A"},"stats":[{"name":"playoffSeed","displayValue":"1"},{"name":"clincher","displayValue":"z"}]}
	]}}`
	if err := json.Unmarshal([]byte(body), &group); err != nil {
		t.Fatal(err)
	}
	seeds := conferenceSeeds(group)
	if len(seeds) != 2 || seeds[0].Team != "A" || seeds[1].Team != "B" || seeds[0].Clinch != "z" || seeds[1].Clinch != "" {
		t.Fatalf("seeds %+v", seeds)
	}
}

func TestPlayoffPictureGroupsSeedsAndListsMatchups(t *testing.T) {
	m := playoffsModel(t, seedsStandings(t, "KC", "BUF", "CLE", "JAX", "DEN", "PIT", "CIN", "NE", "BAL", "LV", "NYJ", "IND", "GB"))
	text := stripANSI(strings.Join(m.playoffLines(70), "\n"))
	for _, want := range []string{"DIVISION LEADERS", "WILD CARD", "IN THE HUNT", " 1  KC", " 7  CIN", " 8  NE", "11  NYJ", "7 CIN @ 2 BUF", "6 PIT @ 3 CLE", "5 DEN @ 4 JAX", "1 KC has the bye"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "IND") {
		t.Fatalf("seed 12 is out of the hunt:\n%s", text)
	}
	if !strings.Contains(text, "13  GB") {
		t.Fatalf("the favorite must always be listed:\n%s", text)
	}
	if strings.ContainsRune(text, '\a') {
		t.Fatal("team names must be sanitized")
	}
}

func TestPlayoffPictureWaitsForSeeds(t *testing.T) {
	m := playoffsModel(t, standings{Children: []standingGroup{{Abbreviation: "AFC"}}})
	if text := strings.Join(m.playoffLines(70), "\n"); !strings.Contains(text, "No playoff seeds yet") {
		t.Fatalf("lines:\n%s", text)
	}
}

func TestBracketShowsRoundsOnceSet(t *testing.T) {
	var seen []string
	m := playoffsModel(t, standings{})
	m.api = leagueClient(t, &seen)
	m = press(t, m, "2")
	text := stripANSI(m.View())
	for _, want := range []string{"[ 2 BRACKET ]", "WILD CARD", "GB 27 @ CHI 24", "Final", "DIVISIONAL ROUND", "SUPER BOWL", "To be decided."} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q:\n%s", want, text)
		}
	}
	rounds := map[string]bool{}
	for _, u := range seen {
		if strings.Contains(u, "seasontype=3") {
			rounds[u] = true
		}
	}
	if len(rounds) != 4 {
		t.Fatalf("every round must load: %v", seen)
	}
}

func TestBracketWaitsForTheRegularSeasonToEnd(t *testing.T) {
	m := playoffsModel(t, standings{})
	m.api = leagueClientWith(t, nil, tbdJSON)
	m = press(t, m, "2")
	text := stripANSI(m.View())
	if !strings.Contains(text, "Matchups appear when the regular season ends.") || strings.Contains(text, "TBD") {
		t.Fatalf("view:\n%s", text)
	}
}

func TestBracketSaysWhenItCouldNotLoad(t *testing.T) {
	m := playoffsModel(t, standings{})
	m.api = &client{http: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("network down")
	})}}
	m = press(t, m, "2")
	text := stripANSI(m.View())
	if !strings.Contains(text, "Bracket:") || strings.Contains(text, "Matchups appear") {
		t.Fatalf("view:\n%s", text)
	}
}

func TestRoundWithoutGamesIsNotAFailure(t *testing.T) {
	m := playoffsModel(t, standings{})
	m = press(t, m, "2")
	m = m.applyPlayoffRound(playoffRoundMsg{season: 2026, week: 5, err: fmt.Errorf("wrapped: %w", errNoGames)})
	if text := stripANSI(m.View()); strings.Contains(text, "UPDATE FAILED") || !strings.Contains(text, "To be decided.") {
		t.Fatalf("view:\n%s", text)
	}
}

func TestPictureExplainsClinchLetters(t *testing.T) {
	var group standingGroup
	body := `{"standings":{"entries":[{"team":{"abbreviation":"SEA"},"stats":[{"name":"playoffSeed","displayValue":"1"},{"name":"clincher","displayValue":"*"}]}]}}`
	if err := json.Unmarshal([]byte(body), &group); err != nil {
		t.Fatal(err)
	}
	m := playoffsModel(t, standings{Children: []standingGroup{group}})
	if text := stripANSI(strings.Join(m.playoffLines(70), "\n")); !strings.Contains(text, "* top seed") {
		t.Fatalf("lines:\n%s", text)
	}
	m = playoffsModel(t, seedsStandings(t, "KC"))
	if text := stripANSI(strings.Join(m.playoffLines(70), "\n")); strings.Contains(text, "top seed") {
		t.Fatalf("no legend without letters:\n%s", text)
	}
}

func TestPictureDropsHypotheticalGamesInThePostseason(t *testing.T) {
	m := playoffsModel(t, seedsStandings(t, "KC", "BUF", "CLE", "JAX", "DEN", "PIT", "CIN"))
	m.seasonType = 3
	if text := strings.Join(m.playoffLines(70), "\n"); strings.Contains(text, "IF THE SEASON ENDED TODAY") {
		t.Fatalf("lines:\n%s", text)
	}
}

func TestPictureFooterKeepsEveryKeyOnAWidePanel(t *testing.T) {
	m := playoffsModel(t, seedsStandings(t, "KC"))
	if view := stripANSI(m.View()); !strings.Contains(view, "r refresh") {
		t.Fatalf("view:\n%s", view)
	}
}

func TestBracketTabOpensFirstDuringThePostseason(t *testing.T) {
	m := model{api: leagueClient(t, nil), season: 2026, seasonType: 3, week: 1, favorite: "GB", width: 100, height: 40, showHome: true}
	m, _ = m.openPlayoffs()
	if m.playoffTab != playoffBracket {
		t.Fatalf("tab %d", m.playoffTab)
	}
}

func TestPlayoffsConferenceStartsOnTheFavorite(t *testing.T) {
	m := model{api: leagueClient(t, nil), season: 2026, seasonType: 2, week: 9, favorite: "BUF", standingsConference: 1, width: 100, height: 40, showHome: true}
	m, _ = m.openPlayoffs()
	if m.standingsConference != 0 {
		t.Fatalf("conference %d", m.standingsConference)
	}
}

func TestPlayoffsKeysAndBack(t *testing.T) {
	m := model{api: leagueClient(t, nil), season: 2026, week: 9, seasonType: 2, favorite: "GB", standings: seedsStandings(t, "KC"), standingsSeason: 2026, width: 100, height: 40, showHome: true}
	m = press(t, m, "P")
	if !m.showPlayoffs || m.showHome {
		t.Fatalf("P must open playoffs from home: playoffs %v home %v", m.showPlayoffs, m.showHome)
	}
	before := m.standingsConference
	m = press(t, m, "right")
	if m.standingsConference == before {
		t.Fatal("right must switch conference")
	}
	m = press(t, m, "esc")
	if m.showPlayoffs || !m.showHome {
		t.Fatalf("esc must return home: playoffs %v home %v", m.showPlayoffs, m.showHome)
	}
}

func TestLeadersAskESPNAndKeepOurOrder(t *testing.T) {
	var seen []string
	data, err := leagueClient(t, &seen).leaders(context.Background(), 2026, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != 1 || !strings.HasPrefix(seen[0], "site.web.api.espn.com/apis/site/v3/sports/football/nfl/leaders?") || !strings.Contains(seen[0], "season=2026") || !strings.Contains(seen[0], "seasontype=2") {
		t.Fatalf("request %v", seen)
	}
	if len(data.Categories) != 2 || data.Categories[0].Title != "Passing Yards" || data.Categories[1].Title != "Rushing Yards" {
		t.Fatalf("categories %+v", data.Categories)
	}
	love := data.Categories[0].Rows[0]
	if love.Name != "Jordan Love" || love.AthleteID != "1" || love.Position != "QB" || love.Team != "GB" || love.Value != "2,246" {
		t.Fatalf("row %+v", love)
	}
}

func leadersModel(t *testing.T) model {
	t.Helper()
	m := model{api: leagueClient(t, nil), season: 2026, week: 9, seasonType: 2, favorite: "GB", width: 100, height: 24, showHome: true}
	return press(t, m, "L")
}

func TestLeadersScreenRendersAndOpensAPlayerCard(t *testing.T) {
	m := leadersModel(t)
	view := stripANSI(m.View())
	for _, want := range []string{"LEAGUE LEADERS", "PASSING YARDS", "1  Jordan Love", "QB", "2,246", "RUSHING YARDS"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q:\n%s", want, view)
		}
	}
	m = press(t, m, "down")
	if m.leaderSelected != 1 {
		t.Fatalf("selected %d", m.leaderSelected)
	}
	m = press(t, m, "enter")
	if !m.showPlayer || m.player.Name != "Patrick Mahomes" || m.player.Team != "KC" || m.player.TeamName != "Kansas City Chiefs" {
		t.Fatalf("player %+v", m.player)
	}
	m = press(t, m, "esc")
	if m.showPlayer || !m.showLeaders {
		t.Fatalf("esc must return to leaders: player %v leaders %v", m.showPlayer, m.showLeaders)
	}
	m = press(t, m, "esc")
	if m.showLeaders || !m.showHome {
		t.Fatalf("esc must return home: leaders %v home %v", m.showLeaders, m.showHome)
	}
}

func TestLeagueScreensLoadWhenTheSeasonArrives(t *testing.T) {
	m := model{api: leagueClient(t, nil), favorite: "GB", width: 100, height: 24, showHome: true}
	m = press(t, m, "L")
	if !m.showLeaders || !strings.Contains(m.View(), "Loading") {
		t.Fatalf("waiting view:\n%s", m.View())
	}
	var board scoreboard
	board.Week.Number = 9
	board.Leagues = make([]struct {
		Season struct {
			Year int `json:"year"`
			Type struct {
				ID string `json:"id"`
			} `json:"type"`
		} `json:"season"`
	}, 1)
	board.Leagues[0].Season.Year = 2026
	board.Leagues[0].Season.Type.ID = "2"
	next, cmd := m.Update(boardMsg{board: board})
	m = next.(model)
	for _, reply := range runCmds(cmd) {
		next, _ = m.Update(reply)
		m = next.(model)
	}
	if !strings.Contains(stripANSI(m.View()), "PASSING YARDS") {
		t.Fatalf("view:\n%s", m.View())
	}
}

func TestLeagueScreensFitTheTerminal(t *testing.T) {
	picture := seedsStandings(t, "KC", "BUF", "CLE", "JAX", "DEN", "PIT", "CIN", "NE", "BAL", "LV", "NYJ", "IND", "GB")
	for _, size := range [][2]int{{120, 40}, {100, 24}, {80, 24}, {60, 20}, {40, 15}, {30, 10}} {
		for _, name := range []string{"playoffs", "bracket", "leaders"} {
			m := playoffsModel(t, picture)
			switch name {
			case "bracket":
				m = press(t, m, "2")
			case "leaders":
				m.showPlayoffs, m.showHome = false, true
				m = press(t, m, "L")
			}
			m.width, m.height = size[0], size[1]
			view := m.View()
			if lines := strings.Count(view, "\n") + 1; lines > size[1] {
				t.Fatalf("%s at %v: %d lines:\n%s", name, size, lines, view)
			}
			for _, line := range strings.Split(view, "\n") {
				if lipgloss.Width(line) > size[0] {
					t.Fatalf("%s at %v: line too wide: %q", name, size, line)
				}
			}
		}
	}
}

func TestHomeMenuReachesLeagueScreens(t *testing.T) {
	for i, flag := range map[int]func(model) bool{8: func(m model) bool { return m.showPlayoffs }, 9: func(m model) bool { return m.showLeaders }} {
		m := model{api: leagueClient(t, nil), season: 2026, week: 9, seasonType: 2, favorite: "GB", width: 100, height: 24, showHome: true, homeSelection: i}
		m = press(t, m, "enter")
		if !flag(m) {
			t.Fatalf("menu item %d", i)
		}
		m = press(t, m, "esc")
		if !m.showHome {
			t.Fatalf("menu item %d: esc must return home", i)
		}
	}
}

func TestHomeMenuFitsAtEveryHeight(t *testing.T) {
	for height := 5; height <= 30; height++ {
		m := model{favorite: "GB", width: 100, height: height, showHome: true, homeSelection: 9}
		if lines := strings.Count(m.View(), "\n") + 1; lines > height {
			t.Fatalf("height %d: %d lines", height, lines)
		}
		if !strings.Contains(stripANSI(m.View()), "LEAGUE LEADERS") {
			t.Fatalf("height %d: selection must stay visible:\n%s", height, m.View())
		}
	}
}

func TestLeagueScreensMarkStaleData(t *testing.T) {
	m := leadersModel(t)
	m = m.applyLeaders(leadersMsg{season: 2026, err: fmt.Errorf("boom")})
	if view := stripANSI(m.View()); !strings.Contains(view, "UPDATE FAILED") || !strings.Contains(view, "PASSING YARDS") {
		t.Fatalf("view:\n%s", view)
	}
}

var _ tea.Msg = leadersMsg{}
