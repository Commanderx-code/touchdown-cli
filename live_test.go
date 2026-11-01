package main

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

const liveSituationJSON = `{"week":{"number":9},"events":[{"id":"1","date":"2026-11-01T21:25Z","status":{"type":{"state":"in","shortDetail":"2:14 - 4th","description":"In Progress"},"displayClock":"2:14","period":4},"competitions":[{"venue":{"fullName":"Lambeau Field"},"situation":{"possession":"8","down":3,"distance":7,"isRedZone":true,"downDistanceText":"3rd & 7 at CAR 22","possessionText":"GB 22","homeTimeouts":3,"awayTimeouts":2,"lastPlay":{"text":"J.Love pass short right to C.Watson for 9 yards"}},"competitors":[{"homeAway":"home","score":"27","team":{"id":"8","abbreviation":"GB","name":"Packers"}},{"homeAway":"away","score":"19","team":{"id":"29","abbreviation":"CAR","name":"Panthers"}}]}]}]}`

func TestScoreboardParsesSituationFields(t *testing.T) {
	api := jsonClient(t, func(*http.Request) {}, liveSituationJSON)
	board, err := api.scoreboard(context.Background(), 0, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	s := board.Events[0].Competitions[0].Situation
	if s.DownDistanceText != "3rd & 7 at CAR 22" || s.PossessionText != "GB 22" || s.HomeTimeouts == nil || *s.HomeTimeouts != 3 || s.AwayTimeouts == nil || *s.AwayTimeouts != 2 {
		t.Fatalf("situation not parsed: %+v", s)
	}
}

func TestSummaryParsesScoringType(t *testing.T) {
	api := jsonClient(t, func(*http.Request) {}, `{"scoringPlays":[{"id":"p1","text":"J.Love pass to C.Watson for 12 yards, TOUCHDOWN","awayScore":19,"homeScore":27,"scoringType":{"abbreviation":"TD"},"team":{"abbreviation":"GB"},"period":{"number":4},"clock":{"displayValue":"2:14"}}]}`)
	detail, err := api.summary(context.Background(), "1")
	if err != nil {
		t.Fatal(err)
	}
	if detail.ScoringPlays[0].ScoringType.Abbreviation != "TD" {
		t.Fatalf("%+v", detail.ScoringPlays[0])
	}
}

func liveGame(possession string, down, distance int, ddText string, rz bool, away, home *int, last string) game {
	g := game{ID: "1", Date: "2026-11-01T21:25Z",
		Status: gameStatus{Type: statusType{State: "in", ShortDetail: "2:14 - 4th", Description: "In Progress"}, DisplayClock: "2:14", Period: 4},
		Competitions: []competition{{
			Situation: gameSituation{Possession: possession, Down: down, Distance: distance, DownDistanceText: ddText, IsRedZone: rz, HomeTimeouts: home, AwayTimeouts: away},
			Competitors: []competitor{
				{HomeAway: "away", Score: "19", Team: teamRef{ID: "29", Abbreviation: "CAR", Name: "Panthers", DisplayName: "Carolina Panthers"}},
				{HomeAway: "home", Score: "27", Team: teamRef{ID: "8", Abbreviation: "GB", Name: "Packers", DisplayName: "Green Bay Packers"}},
			}}}}
	g.Competitions[0].Venue.FullName = "Lambeau Field"
	g.Competitions[0].Situation.LastPlay.Text = last
	return g
}

func TestSituationText(t *testing.T) {
	three, two := 3, 2
	cases := []struct {
		name string
		g    game
		want string
	}{
		{"espn text", liveGame("8", 3, 7, "3rd & 7 at CAR 22", false, &two, &three, ""), "GB ball  3rd & 7 at CAR 22"},
		{"built text", liveGame("8", 3, 7, "", false, nil, nil, ""), "GB ball  3rd & 7"},
		{"goal line", liveGame("29", 1, 0, "", true, nil, nil, ""), "CAR ball  1st & GOAL  ·  RED ZONE"},
		{"kickoff no down", liveGame("8", 0, 0, "", false, nil, nil, ""), "GB ball"},
		{"no possession", liveGame("", 0, 0, "", false, nil, nil, ""), ""},
	}
	for _, tc := range cases {
		if got := situationText(tc.g); got != tc.want {
			t.Errorf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
	final := liveGame("8", 3, 7, "3rd & 7", false, nil, nil, "")
	final.Status.Type.State = "post"
	if got := situationText(final); got != "" {
		t.Errorf("final game must have no situation, got %q", got)
	}
}

func TestTimeoutsText(t *testing.T) {
	three, two, zero := 3, 2, 0
	cases := []struct {
		away, home *int
		want       string
	}{
		{&two, &three, "CAR TO ●●○   GB TO ●●●"},
		{&zero, &three, "CAR TO ○○○   GB TO ●●●"},
		{nil, &three, ""},
		{nil, nil, ""},
	}
	for _, tc := range cases {
		if got := timeoutsText(liveGame("8", 1, 10, "", false, tc.away, tc.home, "")); got != tc.want {
			t.Errorf("got %q want %q", got, tc.want)
		}
	}
}

func TestLiveContextLinesOrder(t *testing.T) {
	three, two := 3, 2
	g := liveGame("8", 3, 7, "3rd & 7 at CAR 22", true, &two, &three, "J.Love  pass   to C.Watson")
	m := model{board: scoreboard{Events: []game{g}}, detailID: "1", detailGame: g, showDetail: true}
	want := []string{"GB ball  3rd & 7 at CAR 22  ·  RED ZONE", "CAR TO ●●○   GB TO ●●●", "LATEST  J.Love pass to C.Watson"}
	got := m.liveContextLines()
	if len(got) != len(want) {
		t.Fatalf("got %q want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d: got %q want %q", i, got[i], want[i])
		}
	}
}

func liveBoardModel(width, height int, compact bool) model {
	three, two := 3, 2
	g := liveGame("8", 3, 7, "3rd & 7 at CAR 22", true, &two, &three, "J.Love pass short right to C.Watson for 9 yards")
	return model{board: scoreboard{Events: []game{g}}, season: 2026, week: 9, width: width, height: height, compact: compact}
}

func TestDashboardLiveRowShowsSituationNotVenue(t *testing.T) {
	m := liveBoardModel(100, 30, false)
	view := m.View()
	for _, want := range []string{"GB ball  3rd & 7 at CAR 22", "RED ZONE"} {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "Lambeau Field") || strings.Contains(view, "TO ●") {
		t.Fatalf("live row must drop the venue and never show timeouts:\n%s", view)
	}
	m.board.Events[0].Status.Type.State = "post"
	if view := m.View(); !strings.Contains(view, "Lambeau Field") {
		t.Fatalf("final row lost the venue:\n%s", view)
	}
}

func TestCompactRowSituationNeedsSixtyColumns(t *testing.T) {
	wide := liveBoardModel(60, 20, true).View()
	if !strings.Contains(wide, "GB ball") || strings.Contains(wide, "TO ●") {
		t.Fatalf("compact at 60 columns:\n%s", wide)
	}
	if narrow := liveBoardModel(59, 20, true).View(); strings.Contains(narrow, "GB ball") {
		t.Fatalf("compact at 59 columns must not show the situation:\n%s", narrow)
	}
}

func TestSpotlightShowsLiveSituation(t *testing.T) {
	m := liveBoardModel(120, 40, false)
	m.favorite = "GB"
	view := m.View()
	for _, want := range []string{"GB ball  3rd & 7 at CAR 22", "CAR TO ●●○   GB TO ●●●", "J.Love pass short right"} {
		if strings.Count(view, want) < 1 {
			t.Fatalf("spotlight missing %q:\n%s", want, view)
		}
	}
	if n := strings.Count(view, "\n") + 1; n > 40 {
		t.Fatalf("dashboard overflowed: %d lines", n)
	}
	long := strings.Repeat("word ", 80)
	m.board.Events[0].Competitions[0].Situation.LastPlay.Text = long
	view = m.View()
	playLines := 0
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "word word word") && !strings.Contains(line, "LATEST") {
			playLines++
		}
	}
	if playLines == 0 || playLines > 2 {
		t.Fatalf("latest play should wrap to at most 2 lines, got %d:\n%s", playLines, view)
	}
	if n := strings.Count(view, "\n") + 1; n > 40 {
		t.Fatalf("dashboard overflowed with long play: %d lines", n)
	}
}

func TestLiveTextStripsControlCharacters(t *testing.T) {
	three := 3
	g := liveGame("8", 3, 7, "3rd & 7\x1b[31m at CAR 22", true, &three, &three, "J.Love\x1b]52;c;evil\x07 pass\r\nto C.Watson")
	for _, got := range []string{situationText(g), latestPlayText(g), timeoutsText(g)} {
		if strings.ContainsAny(got, "\x1b\x07\r\n") {
			t.Fatalf("control characters leaked: %q", got)
		}
	}
	if got := latestPlayText(g); got != "J.Love]52;c;evil pass to C.Watson" {
		t.Fatalf("latest play: %q", got)
	}
}

func TestHelpLegendMentionsRedZoneAndTimeouts(t *testing.T) {
	m := model{showHelp: true, width: 100, height: 30}
	view := m.View()
	if !strings.Contains(view, "RED ZONE") || !strings.Contains(view, "TO ●●○") {
		t.Fatalf("legend missing:\n%s", view)
	}
}
