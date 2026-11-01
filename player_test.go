package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

const injuriesJSON = `{"injuries":[
 {"status":"Active","type":{"abbreviation":"A"},"athlete":{"displayName":"Josh Jacobs","position":{"abbreviation":"RB"},"links":[{"href":"https://www.espn.com/nfl/player/_/id/4047365/josh-jacobs"}]},"shortComment":"fine","date":"2026-10-31T14:09Z"},
 {"status":"Questionable","type":{"abbreviation":"Q"},"athlete":{"displayName":"Christian Watson","position":{"abbreviation":"WR"},"links":[{"href":"https://www.espn.com/nfl/player/_/id/4258173/christian-watson"}]},"details":{"type":"Ribs","returnDate":"2026-11-08"},"shortComment":"Watson said he suffered a broken rib.","date":"2026-10-30T04:06Z"},
 {"status":"Out","type":{"abbreviation":"O"},"athlete":{"displayName":"Micah Parsons","position":{"abbreviation":"DE"},"links":[{"href":"https://www.espn.com/nfl/player/_/id/4361423/micah-parsons"}]},"details":{"type":"Knee - ACL","returnDate":"2026-11-25"},"shortComment":"Targeting Week 12.\u0007","date":"2026-10-31T14:09Z"},
 {"status":"Injured Reserve","type":{"abbreviation":"IR"},"athlete":{"displayName":"No Link","position":{"abbreviation":"G"},"links":[]},"shortComment":"","date":"2026-10-29T22:54Z"}
]}`

func TestInjuriesParsesSortsAndDropsActive(t *testing.T) {
	var path, query string
	api := jsonClient(t, func(req *http.Request) { path, query = req.URL.Path, req.URL.RawQuery }, injuriesJSON)
	report, err := api.injuries(context.Background(), "GB")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, "/injuries") || query != "team=gb" {
		t.Fatalf("request %s?%s", path, query)
	}
	if len(report.Entries) != 3 || report.Team != "GB" {
		t.Fatalf("entries %+v", report.Entries)
	}
	parsons := report.Entries[0]
	if parsons.Name != "Micah Parsons" || parsons.AthleteID != "4361423" || parsons.Status != "Out" || parsons.Code != "O" || parsons.BodyPart != "Knee - ACL" || parsons.ReturnDate != "2026-11-25" || parsons.Comment != "Targeting Week 12." || parsons.Position != "DE" {
		t.Fatalf("parsons %+v", parsons)
	}
	if report.Entries[1].Name != "Christian Watson" || report.Entries[2].Name != "No Link" || report.Entries[2].AthleteID != "" {
		t.Fatalf("order or id: %+v", report.Entries)
	}
	if out, d, q, ir := report.counts(); out != 1 || d != 0 || q != 1 || ir != 1 {
		t.Fatalf("counts %d %d %d %d", out, d, q, ir)
	}
	if report.summary() != "1 OUT · 1 Q · 1 IR" || report.gameDaySummary() != "1 OUT · 1 Q" {
		t.Fatalf("summary %q gameDay %q", report.summary(), report.gameDaySummary())
	}
	if len(report.gameDay()) != 2 {
		t.Fatalf("gameDay %+v", report.gameDay())
	}
	if _, ok := report.find("4258173"); !ok {
		t.Fatal("find Watson")
	}
	if statusLabel(parsons) != "OUT" || statusLabel(report.Entries[1]) != "Q" || statusLabel(report.Entries[2]) != "IR" || statusLabel(injury{Code: "susp"}) != "SUSP" {
		t.Fatal("status labels")
	}
	if returnLabel("2026-11-25") != "back Nov 25" || returnLabel("") != "" || returnLabel("soon") != "" {
		t.Fatalf("returnLabel %q", returnLabel("2026-11-25"))
	}
	if (injuryReport{}).summary() != "" {
		t.Fatal("empty summary")
	}
}

const rosterJSON = `{"athletes":[
 {"position":"offense","items":[{"id":"4036378","displayName":"Jordan Love","jersey":"10","position":{"abbreviation":"QB"},"age":27,"displayHeight":"6' 4\"","displayWeight":"219 lbs","experience":{"years":6},"college":{"shortName":"Utah State"},"status":{"name":"Active"},"injuries":[]},
   {"id":"1","displayName":"No Jersey","position":{"abbreviation":"WR"},"status":{"name":"Active"},"injuries":[{"status":"Questionable"}]}]},
 {"position":"specialTeam","items":[]},
 {"position":"injuredReserveOrOut","items":[{"id":"2","displayName":"Hurt Guy","jersey":"55","position":{"abbreviation":"LB"},"experience":{"years":0},"status":{"name":"Injured Reserve"},"injuries":[{"status":"Injured Reserve"}]}]}
]}`

func TestRosterGroupsAndMissingFields(t *testing.T) {
	var path string
	api := jsonClient(t, func(req *http.Request) { path = req.URL.Path }, rosterJSON)
	r, err := api.roster(context.Background(), "GB")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, "/teams/gb/roster") || r.Team != "GB" {
		t.Fatalf("request %s team %q", path, r.Team)
	}
	if len(r.Groups) != 2 || r.Groups[0].Title != "OFFENSE" || len(r.Groups[0].Players) != 2 || r.Groups[1].Title != "IR / OUT" || len(r.Groups[1].Players) != 1 {
		t.Fatalf("groups %+v", r.Groups)
	}
	love := r.Groups[0].Players[0]
	if love.ID != "4036378" || love.Jersey != "10" || love.Position != "QB" || love.Age != 27 || love.Years != 6 || !love.HasYears || love.College != "Utah State" || love.Height != `6' 4"` || love.Weight != "219 lbs" || love.Injury != "" {
		t.Fatalf("love %+v", love)
	}
	if experienceLabel(love) != "6 yrs" || seasonLabelFor(love) != "6th season" {
		t.Fatalf("love labels %q %q", experienceLabel(love), seasonLabelFor(love))
	}
	nj := r.Groups[0].Players[1]
	if nj.Jersey != "--" || nj.HasYears || experienceLabel(nj) != "" || seasonLabelFor(nj) != "" || nj.Injury != "Questionable" || nj.College != "" {
		t.Fatalf("no jersey %+v", nj)
	}
	hurt := r.Groups[1].Players[0]
	if experienceLabel(hurt) != "Rookie" || seasonLabelFor(hurt) != "Rookie" || hurt.Status != "Injured Reserve" {
		t.Fatalf("hurt %+v", hurt)
	}
}

const overviewJSON = `{"statistics":{"displayName":"2026 Passing","labels":["CMP","ATT","YDS","CAR","YDS"],"categories":[{"name":"passing","displayName":"Passing","count":3},{"name":"rushing","displayName":"Rushing","count":2}],"splits":[{"displayName":"Regular Season","stats":["114","171","1,504","48"]},{"displayName":"Projected","stats":["323","485","4,261","136","680"]}]}}`

func TestPlayerOverviewSlicesCategories(t *testing.T) {
	var path string
	api := jsonClient(t, func(req *http.Request) { path = req.URL.Path }, overviewJSON)
	stats, err := api.playerOverview(context.Background(), "3918298")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, "/athletes/3918298/overview") || stats.ID != "3918298" {
		t.Fatalf("request %s id %q", path, stats.ID)
	}
	if len(stats.Sections) != 2 {
		t.Fatalf("sections %+v", stats.Sections)
	}
	p := stats.Sections[0]
	if p.Title != "PASSING" || p.Split != "" || strings.Join(p.Labels, ",") != "CMP,ATT,YDS" || strings.Join(p.Values, ",") != "114,171,1,504" {
		t.Fatalf("passing %+v", p)
	}
	r := stats.Sections[1]
	if r.Title != "RUSHING" || strings.Join(r.Labels, ",") != "CAR,YDS" || strings.Join(r.Values, ",") != "48" {
		t.Fatalf("rushing %+v", r)
	}
	career := `{"statistics":{"labels":["G","TD"],"categories":[{"displayName":"General","count":5}],"splits":[{"displayName":"Projected","stats":["1","2"]},{"displayName":"Career","stats":["17","9"]}]}}`
	stats, err = jsonClient(t, func(*http.Request) {}, career).playerOverview(context.Background(), "1")
	if err != nil || len(stats.Sections) != 1 || stats.Sections[0].Split != "CAREER" || strings.Join(stats.Sections[0].Values, ",") != "17,9" || len(stats.Sections[0].Labels) != 2 {
		t.Fatalf("career %+v err %v", stats.Sections, err)
	}
	stats, err = jsonClient(t, func(*http.Request) {}, `{"statistics":null}`).playerOverview(context.Background(), "1")
	if err != nil || len(stats.Sections) != 0 {
		t.Fatalf("no stats: %+v err %v", stats.Sections, err)
	}
}

func TestRequestInjuriesOncePerTeamAndStoresReply(t *testing.T) {
	m := model{api: jsonClient(t, func(*http.Request) {}, injuriesJSON)}
	m, cmd := m.requestInjuries("GB", false)
	if cmd == nil {
		t.Fatal("first request must fetch")
	}
	if entry, ok := m.injuriesFor("GB"); !ok || !entry.Loading {
		t.Fatalf("entry after request: %+v ok %v", entry, ok)
	}
	if _, again := m.requestInjuries("GB", false); again != nil {
		t.Fatal("in-flight request must not refetch")
	}
	next, _ := m.Update(runCmds(cmd)[0])
	m = next.(model)
	entry, ok := m.injuriesFor("GB")
	if !ok || entry.Loading || entry.Error != "" || len(entry.Report.Entries) != 3 {
		t.Fatalf("stored entry: %+v", entry)
	}
	if _, again := m.requestInjuries("GB", false); again != nil {
		t.Fatal("loaded entry must not refetch without force")
	}
	if _, forced := m.requestInjuries("GB", true); forced == nil {
		t.Fatal("force must refetch")
	}
	m = m.applyInjuries(injuriesMsg{team: "DET", err: errors.New("boom")})
	if entry, _ := m.injuriesFor("DET"); entry.Error != "boom" || entry.Loading {
		t.Fatalf("error entry: %+v", entry)
	}
}

func TestRosterAndPlayerStores(t *testing.T) {
	m := model{api: jsonClient(t, func(*http.Request) {}, rosterJSON)}
	m, cmd := m.requestRoster("GB", false)
	next, _ := m.Update(runCmds(cmd)[0])
	m = next.(model)
	if entry, ok := m.rosterFor("GB"); !ok || entry.Loading || len(entry.Data.Groups) != 2 {
		t.Fatalf("roster entry: %+v", entry)
	}
	if _, again := m.requestRoster("GB", false); again != nil {
		t.Fatal("loaded roster must not refetch")
	}
	m.api = jsonClient(t, func(*http.Request) {}, overviewJSON)
	m, cmd = m.requestPlayer("3918298", false)
	next, _ = m.Update(runCmds(cmd)[0])
	m = next.(model)
	if entry, ok := m.playerFor("3918298"); !ok || entry.Loading || len(entry.Stats.Sections) != 2 {
		t.Fatalf("player entry: %+v", entry)
	}
	m = m.applyPlayer(playerMsg{id: "x", err: errors.New("bad")})
	if entry, _ := m.playerFor("x"); entry.Error != "bad" {
		t.Fatalf("player error: %+v", entry)
	}
}

// teamDataClient serves schedules, rosters and injury reports for any team.
func teamDataClient(t *testing.T, seen *[]string) *client {
	t.Helper()
	return &client{http: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if seen != nil {
			*seen = append(*seen, req.URL.Path+"?"+req.URL.RawQuery)
		}
		body := `{}`
		switch {
		case strings.HasSuffix(req.URL.Path, "/roster"):
			body = rosterJSON
		case strings.HasSuffix(req.URL.Path, "/injuries"):
			body = injuriesJSON
		case strings.HasSuffix(req.URL.Path, "/overview"):
			body = overviewJSON
		case strings.HasSuffix(req.URL.Path, "/schedule"):
			parts := strings.Split(req.URL.Path, "/")
			body = scheduleJSON(strings.ToUpper(parts[len(parts)-2]))
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})}}
}

func seasonTabsModel(t *testing.T, seen *[]string) model {
	t.Helper()
	m := favoriteSeasonModel(t)
	m.api = teamDataClient(t, seen)
	m.showTeam = true
	m.width, m.height = 100, 24
	return m
}

// press sends a key and runs its command tree back through Update.
func press(t *testing.T, m model, name string) model {
	t.Helper()
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(name)}
	switch name {
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		msg = tea.KeyMsg{Type: tea.KeyTab}
	case "down":
		msg = tea.KeyMsg{Type: tea.KeyDown}
	case "up":
		msg = tea.KeyMsg{Type: tea.KeyUp}
	}
	next, cmd := m.Update(msg)
	m = next.(model)
	for _, reply := range runCmds(cmd) {
		next, _ = m.Update(reply)
		m = next.(model)
	}
	return m
}

func TestSeasonPageTabsLoadLazilyAndRender(t *testing.T) {
	var seen []string
	m := seasonTabsModel(t, &seen)
	if view := m.View(); !strings.Contains(view, "[ 1 SCHEDULE ]") || !strings.Contains(view, "2 ROSTER") {
		t.Fatalf("tab line missing:\n%s", view)
	}
	m = press(t, m, "2")
	rosterRequests := 0
	for _, u := range seen {
		if strings.Contains(u, "/roster") {
			rosterRequests++
		}
	}
	if rosterRequests != 1 || m.teamTab != 1 {
		t.Fatalf("roster tab: requests %d tab %d", rosterRequests, m.teamTab)
	}
	view := m.View()
	for _, want := range []string{"[ 2 ROSTER ]", "OFFENSE", "#10  QB   Jordan Love", "IR / OUT", "#--", "Utah State", "6 yrs"} {
		if !strings.Contains(view, want) {
			t.Fatalf("roster view missing %q:\n%s", want, view)
		}
	}
	m = press(t, m, "3")
	view = m.View()
	for _, want := range []string{"[ 3 INJURIES ]", "INJURY REPORT  ·  1 OUT · 1 Q · 1 IR", "OUT   DE   Micah Parsons", "Knee - ACL", "back Nov 25", "Targeting Week 12."} {
		if !strings.Contains(view, want) {
			t.Fatalf("injury view missing %q:\n%s", want, view)
		}
	}
	m = press(t, m, "tab")
	if m.teamTab != 0 {
		t.Fatalf("tab wraps to schedule, got %d", m.teamTab)
	}
	for tab := 0; tab < 3; tab++ {
		m.teamTab = tab
		if lines := strings.Count(m.View(), "\n") + 1; lines > m.height {
			t.Errorf("tab %d has %d lines at height %d", tab, lines, m.height)
		}
	}
}

func TestSeasonPageTabSelectionAndRefresh(t *testing.T) {
	var seen []string
	m := seasonTabsModel(t, &seen)
	m = press(t, press(t, m, "2"), "down")
	if m.rosterSelected != 1 || m.rosterSelectedLine() != 2 {
		t.Fatalf("roster selection %d line %d", m.rosterSelected, m.rosterSelectedLine())
	}
	seen = nil
	m = press(t, m, "r")
	if len(seen) != 1 || !strings.Contains(seen[0], "/roster") {
		t.Fatalf("r on the roster tab must refetch the roster only: %v", seen)
	}
	m = press(t, m, "3")
	seen = nil
	m = press(t, m, "r")
	if len(seen) != 1 || !strings.Contains(seen[0], "/injuries") {
		t.Fatalf("r on the injuries tab must refetch injuries only: %v", seen)
	}
	m, _ = m.openTeamSeason("DET")
	if m.teamTab != 0 || m.rosterSelected != 0 || m.injurySelected != 0 {
		t.Fatalf("opening another team must reset tabs: tab %d roster %d injury %d", m.teamTab, m.rosterSelected, m.injurySelected)
	}
}

func TestInFlightReplyForAnotherTeamIsStoredNotShown(t *testing.T) {
	m := seasonTabsModel(t, nil)
	m, _ = m.setTeamTab(1) // GB roster in flight
	m, _ = m.openTeamSeason("DET")
	m, _ = m.setTeamTab(1) // DET roster in flight
	next, _ := m.Update(rosterMsg{team: "GB", data: roster{Team: "GB", Groups: []rosterGroup{{Title: "OFFENSE", Players: []rosterPlayer{{Name: "Jordan Love"}}}}}})
	m = next.(model)
	if entry, ok := m.rosterFor("GB"); !ok || entry.Loading || len(entry.Data.Groups) != 1 {
		t.Fatalf("GB reply must be stored: %+v", entry)
	}
	if view := m.View(); !strings.Contains(view, "Loading roster…") || strings.Contains(view, "Jordan Love") {
		t.Fatalf("DET page must still be loading:\n%s", view)
	}
}
