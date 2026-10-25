package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

const apiBase = "https://site.api.espn.com/apis/site/v2/sports/football/nfl"
const standingsURL = "https://site.api.espn.com/apis/v2/sports/football/nfl/standings"

type client struct {
	http     *http.Client
	cacheDir string
}

func newClient() *client {
	cacheDir := ""
	if base, err := os.UserCacheDir(); err == nil {
		cacheDir = filepath.Join(base, "touchdown")
	}
	return &client{http: &http.Client{Timeout: 10 * time.Second}, cacheDir: cacheDir}
}

func (c *client) get(ctx context.Context, path string, query url.Values, out any) (time.Time, error) {
	u := apiBase + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	return c.getURL(ctx, u, out)
}

type scoreboard struct {
	CachedAt time.Time `json:"-"`
	Leagues  []struct {
		Season struct {
			Year int `json:"year"`
			Type struct {
				ID string `json:"id"`
			} `json:"type"`
		} `json:"season"`
	} `json:"leagues"`
	Week struct {
		Number int `json:"number"`
	} `json:"week"`
	Events []game `json:"events"`
}

type game struct {
	ID           string        `json:"id"`
	Date         string        `json:"date"`
	Status       gameStatus    `json:"status"`
	Competitions []competition `json:"competitions"`
}

type competition struct {
	Competitors []competitor  `json:"competitors"`
	Broadcast   string        `json:"broadcast"`
	Status      gameStatus    `json:"status"`
	Situation   gameSituation `json:"situation"`
	Venue       struct {
		FullName string `json:"fullName"`
	} `json:"venue"`
}

type gameSituation struct {
	Possession string `json:"possession"`
	Down       int    `json:"down"`
	Distance   int    `json:"distance"`
	LastPlay   struct {
		Text string `json:"text"`
	} `json:"lastPlay"`
}

func (g game) dateTime() time.Time {
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04Z07:00"} {
		if t, err := time.Parse(layout, g.Date); err == nil {
			return t
		}
	}
	return time.Time{}
}

type gameStatus struct {
	Type struct {
		State       string `json:"state"`
		ShortDetail string `json:"shortDetail"`
		Description string `json:"description"`
	} `json:"type"`
}

type competitor struct {
	HomeAway string `json:"homeAway"`
	Score    string `json:"score"`
	Team     struct {
		ID           string `json:"id"`
		DisplayName  string `json:"displayName"`
		Abbreviation string `json:"abbreviation"`
		Name         string `json:"name"`
	} `json:"team"`
	Records []struct {
		Name    string `json:"name"`
		Summary string `json:"summary"`
	} `json:"records"`
	Linescores []struct {
		DisplayValue string `json:"displayValue"`
	} `json:"linescores"`
}

func (g game) sides() (away, home competitor) {
	if len(g.Competitions) == 0 {
		return
	}
	for _, c := range g.Competitions[0].Competitors {
		if c.HomeAway == "home" {
			home = c
		} else {
			away = c
		}
	}
	return
}

func (g game) statusText() string {
	if g.Status.Type.State == "pre" {
		if t := g.dateTime(); !t.IsZero() {
			return t.Local().Format("Mon 3:04 PM MST")
		}
		return "Scheduled"
	}
	if g.Status.Type.ShortDetail != "" {
		return g.Status.Type.ShortDetail
	}
	return g.Status.Type.Description
}

func (c *client) scoreboard(ctx context.Context, season, week, seasonType int) (scoreboard, error) {
	var out scoreboard
	q := url.Values{}
	if season != 0 {
		q.Set("dates", strconv.Itoa(season))
		q.Set("seasontype", strconv.Itoa(seasonType))
		q.Set("week", strconv.Itoa(week))
	}
	cachedAt, err := c.get(ctx, "/scoreboard", q, &out)
	if err != nil {
		return out, err
	}
	out.CachedAt = cachedAt
	if season != 0 && (len(out.Events) == 0 || out.Week.Number != week) {
		return out, fmt.Errorf("no games found for %d week %d", season, week)
	}
	sort.SliceStable(out.Events, func(i, j int) bool { return out.Events[i].dateTime().Before(out.Events[j].dateTime()) })
	return out, nil
}

type boxscore struct {
	Teams []struct {
		Team struct {
			Abbreviation string `json:"abbreviation"`
		} `json:"team"`
		Statistics []struct {
			Name         string `json:"name"`
			DisplayValue string `json:"displayValue"`
		} `json:"statistics"`
	} `json:"teams"`
	Players []struct {
		Team struct {
			Abbreviation string `json:"abbreviation"`
		} `json:"team"`
		Statistics []struct {
			Name     string   `json:"name"`
			Labels   []string `json:"labels"`
			Athletes []struct {
				Athlete struct {
					DisplayName string `json:"displayName"`
				} `json:"athlete"`
				Stats []string `json:"stats"`
			} `json:"athletes"`
		} `json:"statistics"`
	} `json:"players"`
}

type play struct {
	ID             string `json:"id"`
	SequenceNumber string `json:"sequenceNumber"`
	Text           string `json:"text"`
	AwayScore      int    `json:"awayScore"`
	HomeScore      int    `json:"homeScore"`
	ScoringPlay    bool   `json:"scoringPlay"`
	IsTurnover     bool   `json:"isTurnover"`
	Team           string `json:"-"`
	ScoreTeam      struct {
		Abbreviation string `json:"abbreviation"`
	} `json:"team"`
	Type struct {
		Text string `json:"text"`
	} `json:"type"`
	Period struct {
		Number int `json:"number"`
	} `json:"period"`
	Clock struct {
		DisplayValue string `json:"displayValue"`
	} `json:"clock"`
}

type drive struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Result      string `json:"result"`
	IsScore     bool   `json:"isScore"`
	Team        struct {
		Abbreviation string `json:"abbreviation"`
	} `json:"team"`
	Start struct {
		Period struct {
			Number int `json:"number"`
		} `json:"period"`
		Clock struct {
			DisplayValue string `json:"displayValue"`
		} `json:"clock"`
	} `json:"start"`
	Plays []play `json:"plays"`
}

type gameDetail struct {
	Boxscore     boxscore
	Plays        []play
	ScoringPlays []play
	Drives       []drive
	Header       []competition
	CachedAt     time.Time
}

func (c *client) summary(ctx context.Context, id string) (gameDetail, error) {
	var response struct {
		Boxscore     boxscore `json:"boxscore"`
		ScoringPlays []play   `json:"scoringPlays"`
		Header       struct {
			Competitions []competition `json:"competitions"`
		} `json:"header"`
		Drives struct {
			Previous []drive `json:"previous"`
			Current  *drive  `json:"current"`
		} `json:"drives"`
	}
	cachedAt, err := c.get(ctx, "/summary", url.Values{"event": {id}}, &response)
	if err != nil {
		return gameDetail{}, err
	}
	result := gameDetail{Boxscore: response.Boxscore, ScoringPlays: response.ScoringPlays, Header: response.Header.Competitions, CachedAt: cachedAt}
	result.Drives = append(result.Drives, response.Drives.Previous...)
	if response.Drives.Current != nil {
		result.Drives = append(result.Drives, *response.Drives.Current)
	}
	seen := map[string]bool{}
	addDrive := func(d drive) {
		for _, p := range d.Plays {
			if p.ID != "" && seen[p.ID] {
				continue
			}
			seen[p.ID] = true
			p.Team = d.Team.Abbreviation
			result.Plays = append(result.Plays, p)
		}
	}
	for _, d := range response.Drives.Previous {
		addDrive(d)
	}
	if response.Drives.Current != nil {
		addDrive(*response.Drives.Current)
	}
	sort.SliceStable(result.Plays, func(i, j int) bool {
		a, _ := strconv.Atoi(result.Plays[i].SequenceNumber)
		b, _ := strconv.Atoi(result.Plays[j].SequenceNumber)
		return a > b
	})
	return result, nil
}

type standings struct {
	CachedAt time.Time       `json:"-"`
	Children []standingGroup `json:"children"`
}

type standingGroup struct {
	Abbreviation string `json:"abbreviation"`
	Standings    struct {
		Entries []standingEntry `json:"entries"`
	} `json:"standings"`
}

type standingEntry struct {
	Team struct {
		Abbreviation string `json:"abbreviation"`
		DisplayName  string `json:"displayName"`
	} `json:"team"`
	Stats []struct {
		Name         string `json:"name"`
		DisplayValue string `json:"displayValue"`
	} `json:"stats"`
}

func (e standingEntry) stat(name string) string {
	for _, s := range e.Stats {
		if s.Name == name {
			return s.DisplayValue
		}
	}
	return "-"
}

func (c *client) standings(ctx context.Context, season int) (standings, error) {
	var out standings
	u := standingsURL + "?season=" + strconv.Itoa(season)
	cachedAt, err := c.getURL(ctx, u, &out)
	if err != nil {
		return out, err
	}
	out.CachedAt = cachedAt
	if len(out.Children) == 0 {
		return out, fmt.Errorf("standings unavailable for %d", season)
	}
	return out, nil
}
