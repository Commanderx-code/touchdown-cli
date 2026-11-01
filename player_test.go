package main

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
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
