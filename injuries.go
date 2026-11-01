package main

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
)

// injury is one row of a team's injury report.
type injury struct {
	AthleteID  string
	Name       string
	Position   string
	Status     string // Out, Doubtful, Questionable, Injured Reserve, …
	Code       string // ESPN type abbreviation: O, D, Q, IR, SUSP
	BodyPart   string
	ReturnDate string // 2006-01-02 as ESPN gives it
	Comment    string
	Date       time.Time
}

// injuryReport is one team's players who are anything other than active.
type injuryReport struct {
	Team     string
	Entries  []injury
	CachedAt time.Time
	listed   map[string]bool // every athlete ID and name the feed mentioned
}

var athleteIDPattern = regexp.MustCompile(`/id/(\d+)/`)

func (c *client) injuries(ctx context.Context, team string) (injuryReport, error) {
	var response struct {
		Injuries []struct {
			Status string `json:"status"`
			Date   string `json:"date"`
			Type   struct {
				Abbreviation string `json:"abbreviation"`
			} `json:"type"`
			ShortComment string `json:"shortComment"`
			Details      struct {
				Type       string `json:"type"`
				ReturnDate string `json:"returnDate"`
			} `json:"details"`
			Athlete struct {
				DisplayName string `json:"displayName"`
				Position    struct {
					Abbreviation string `json:"abbreviation"`
				} `json:"position"`
				Links []struct {
					Href string `json:"href"`
				} `json:"links"`
			} `json:"athlete"`
		} `json:"injuries"`
	}
	cachedAt, err := c.get(ctx, "/injuries", url.Values{"team": {strings.ToLower(team)}}, &response)
	if err != nil {
		return injuryReport{}, err
	}
	report := injuryReport{Team: team, CachedAt: cachedAt, listed: map[string]bool{}}
	for _, entry := range response.Injuries {
		row := injury{
			Name:       plainText(entry.Athlete.DisplayName),
			Position:   plainText(entry.Athlete.Position.Abbreviation),
			Status:     plainText(entry.Status),
			Code:       plainText(entry.Type.Abbreviation),
			BodyPart:   plainText(entry.Details.Type),
			ReturnDate: plainText(entry.Details.ReturnDate),
			Comment:    plainText(strings.Join(strings.Fields(entry.ShortComment), " ")),
			Date:       game{Date: entry.Date}.dateTime(),
		}
		for _, link := range entry.Athlete.Links {
			if match := athleteIDPattern.FindStringSubmatch(link.Href); match != nil {
				row.AthleteID = match[1]
				report.listed[row.AthleteID] = true
				break
			}
		}
		report.listed[row.Name] = true
		if entry.Status != "Active" {
			report.Entries = append(report.Entries, row)
		}
	}
	sortInjuries(report.Entries)
	return report, nil
}

func injuryRank(status string) int {
	switch status {
	case "Out":
		return 0
	case "Doubtful":
		return 1
	case "Questionable":
		return 2
	case "Injured Reserve":
		return 3
	}
	return 4
}

// statusLabel is the short column form of an injury status.
func statusLabel(i injury) string {
	switch i.Status {
	case "Out":
		return "OUT"
	case "Doubtful":
		return "D"
	case "Questionable":
		return "Q"
	case "Injured Reserve":
		return "IR"
	}
	return strings.ToUpper(i.Code)
}

func (r injuryReport) counts() (out, doubtful, questionable, ir int) {
	for _, i := range r.Entries {
		switch i.Status {
		case "Out":
			out++
		case "Doubtful":
			doubtful++
		case "Questionable":
			questionable++
		case "Injured Reserve":
			ir++
		}
	}
	return
}

// summary is "3 OUT · 1 Q · 2 IR" with zero counts omitted, "" when empty.
func (r injuryReport) summary() string {
	out, d, q, ir := r.counts()
	return joinCounts(out, d, q, ir)
}

// gameDaySummary counts only the players whose status matters for the next
// game: Out, Doubtful and Questionable.
func (r injuryReport) gameDaySummary() string {
	out, d, q, _ := r.counts()
	return joinCounts(out, d, q, 0)
}

func joinCounts(out, d, q, ir int) string {
	var parts []string
	for _, c := range []struct {
		n     int
		label string
	}{{out, "OUT"}, {d, "D"}, {q, "Q"}, {ir, "IR"}} {
		if c.n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", c.n, c.label))
		}
	}
	return strings.Join(parts, " · ")
}

// gameDay is the Out, Doubtful and Questionable rows in report order.
func (r injuryReport) gameDay() []injury {
	var rows []injury
	for _, i := range r.Entries {
		if injuryRank(i.Status) <= 2 {
			rows = append(rows, i)
		}
	}
	return rows
}

func (r injuryReport) find(athleteID string) (injury, bool) {
	if athleteID == "" {
		return injury{}, false
	}
	for _, i := range r.Entries {
		if i.AthleteID == athleteID {
			return i, true
		}
	}
	return injury{}, false
}

// returnLabel turns ESPN's 2006-01-02 return date into "back Nov 25".
func returnLabel(date string) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return ""
	}
	return "back " + t.Format("Jan 2")
}

// fullInjuries is the feed completed from the roster. The feed holds only the
// 25 newest updates, most of them players already cleared, so anyone hurt for
// weeks falls off it; injured reserve above all. The roster lists every
// status but no body part or note, so the feed's row wins where both have
// one. The report still loads when the roster does not.
func (c *client) fullInjuries(ctx context.Context, team string) (injuryReport, error) {
	report, err := c.injuries(ctx, team)
	if err != nil {
		return report, err
	}
	if data, err := c.roster(ctx, team); err == nil {
		report = report.withRoster(data)
	}
	return report, nil
}

// withRoster adds every injured roster player the feed never mentioned. A
// player the feed lists as Active stays off: its update is the newer one.
func (r injuryReport) withRoster(data roster) injuryReport {
	if r.CachedAt.IsZero() {
		r.CachedAt = data.CachedAt
	}
	for _, group := range data.Groups {
		for _, p := range group.Players {
			if p.Injury == "" || p.Injury == "Active" || r.listed[p.ID] || r.listed[p.Name] {
				continue
			}
			code := []rune(strings.ToUpper(p.Injury))
			r.Entries = append(r.Entries, injury{AthleteID: p.ID, Name: p.Name, Position: p.Position, Status: p.Injury, Code: string(code[:min(4, len(code))])})
		}
	}
	sortInjuries(r.Entries)
	return r
}

func sortInjuries(rows []injury) {
	sort.SliceStable(rows, func(i, j int) bool {
		return injuryRank(rows[i].Status) < injuryRank(rows[j].Status)
	})
}
