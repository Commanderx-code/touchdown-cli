package main

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"
)

type rosterPlayer struct {
	ID       string
	Name     string
	Jersey   string
	Position string
	Height   string
	Weight   string
	College  string
	Status   string
	Injury   string
	Age      int
	Years    int
	HasYears bool
}

type rosterGroup struct {
	Key     string
	Title   string
	Players []rosterPlayer
}

type roster struct {
	Team     string
	Groups   []rosterGroup
	CachedAt time.Time
}

var rosterGroupOrder = []struct{ key, title string }{
	{"offense", "OFFENSE"},
	{"defense", "DEFENSE"},
	{"specialTeam", "SPECIAL TEAMS"},
	{"injuredReserveOrOut", "IR / OUT"},
	{"practiceSquad", "PRACTICE SQUAD"},
	{"suspended", "SUSPENDED"},
}

func (c *client) roster(ctx context.Context, team string) (roster, error) {
	var response struct {
		Athletes []struct {
			Position string `json:"position"`
			Items    []struct {
				ID            string `json:"id"`
				DisplayName   string `json:"displayName"`
				Jersey        string `json:"jersey"`
				Age           int    `json:"age"`
				DisplayHeight string `json:"displayHeight"`
				DisplayWeight string `json:"displayWeight"`
				Position      struct {
					Abbreviation string `json:"abbreviation"`
				} `json:"position"`
				Experience *struct {
					Years int `json:"years"`
				} `json:"experience"`
				College struct {
					ShortName string `json:"shortName"`
				} `json:"college"`
				Status struct {
					Name string `json:"name"`
				} `json:"status"`
				Injuries []struct {
					Status string `json:"status"`
				} `json:"injuries"`
			} `json:"items"`
		} `json:"athletes"`
	}
	cachedAt, err := c.get(ctx, "/teams/"+url.PathEscape(strings.ToLower(team))+"/roster", nil, &response)
	if err != nil {
		return roster{}, err
	}
	byKey := map[string][]rosterPlayer{}
	var keys []string
	for _, group := range response.Athletes {
		if _, seen := byKey[group.Position]; !seen {
			keys = append(keys, group.Position)
		}
		for _, item := range group.Items {
			p := rosterPlayer{
				ID:       item.ID,
				Name:     plainText(item.DisplayName),
				Jersey:   plainText(item.Jersey),
				Position: plainText(item.Position.Abbreviation),
				Height:   plainText(item.DisplayHeight),
				Weight:   plainText(item.DisplayWeight),
				College:  plainText(item.College.ShortName),
				Status:   plainText(item.Status.Name),
				Age:      item.Age,
			}
			if p.Jersey == "" {
				p.Jersey = "--"
			}
			if item.Experience != nil {
				p.Years, p.HasYears = item.Experience.Years, true
			}
			if len(item.Injuries) > 0 {
				p.Injury = plainText(item.Injuries[0].Status)
			}
			byKey[group.Position] = append(byKey[group.Position], p)
		}
	}
	out := roster{Team: team, CachedAt: cachedAt}
	known := map[string]bool{}
	for _, g := range rosterGroupOrder {
		known[g.key] = true
		if players := byKey[g.key]; len(players) > 0 {
			out.Groups = append(out.Groups, rosterGroup{Key: g.key, Title: g.title, Players: players})
		}
	}
	for _, key := range keys {
		if !known[key] && len(byKey[key]) > 0 {
			out.Groups = append(out.Groups, rosterGroup{Key: key, Title: strings.ToUpper(plainText(key)), Players: byKey[key]})
		}
	}
	return out, nil
}

// experienceLabel is "6 yrs", "Rookie", or "" when ESPN gave no experience.
func experienceLabel(p rosterPlayer) string {
	switch {
	case !p.HasYears:
		return ""
	case p.Years == 0:
		return "Rookie"
	}
	return fmt.Sprintf("%d yrs", p.Years)
}

// seasonLabelFor is "6th season", "Rookie", or "".
func seasonLabelFor(p rosterPlayer) string {
	switch {
	case !p.HasYears:
		return ""
	case p.Years == 0:
		return "Rookie"
	}
	return ordinal(p.Years) + " season"
}

func ordinal(n int) string {
	suffix := "th"
	switch {
	case n%100 >= 11 && n%100 <= 13:
	case n%10 == 1:
		suffix = "st"
	case n%10 == 2:
		suffix = "nd"
	case n%10 == 3:
		suffix = "rd"
	}
	return fmt.Sprintf("%d%s", n, suffix)
}
