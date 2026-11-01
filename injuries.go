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
	report := injuryReport{Team: team, CachedAt: cachedAt}
	for _, entry := range response.Injuries {
		if entry.Status == "Active" {
			continue
		}
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
				break
			}
		}
		report.Entries = append(report.Entries, row)
	}
	sort.SliceStable(report.Entries, func(i, j int) bool {
		return injuryRank(report.Entries[i].Status) < injuryRank(report.Entries[j].Status)
	})
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
