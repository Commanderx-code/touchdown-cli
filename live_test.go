package main

import (
	"context"
	"net/http"
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
