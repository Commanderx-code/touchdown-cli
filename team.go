package main

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type teamSchedule struct {
	CachedAt time.Time       `json:"-"`
	Team     scheduleTeam    `json:"team"`
	ByeWeek  int             `json:"byeWeek"`
	Events   []scheduledGame `json:"events"`
}

type scheduleTeam struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	DisplayName  string `json:"displayName"`
	Abbreviation string `json:"abbreviation"`
}

type scheduleCompetitor struct {
	HomeAway string       `json:"homeAway"`
	Team     scheduleTeam `json:"team"`
	Score    struct {
		DisplayValue string `json:"displayValue"`
	} `json:"score"`
	Linescores []struct {
		DisplayValue string `json:"displayValue"`
	} `json:"linescores"`
}

type scheduleCompetition struct {
	Status      gameStatus           `json:"status"`
	Competitors []scheduleCompetitor `json:"competitors"`
	Situation   gameSituation        `json:"situation"`
	Venue       struct {
		FullName string `json:"fullName"`
	} `json:"venue"`
}

type scheduledGame struct {
	ID   string `json:"id"`
	Date string `json:"date"`
	Week struct {
		Number int `json:"number"`
	} `json:"week"`
	Competitions []scheduleCompetition `json:"competitions"`
}

func (g scheduledGame) dateTime() time.Time { return game{Date: g.Date}.dateTime() }

func (g scheduledGame) toGame() game {
	out := game{ID: g.ID, Date: g.Date}
	if len(g.Competitions) == 0 {
		return out
	}
	source := g.Competitions[0]
	out.Status = source.Status
	var comp competition
	comp.Status = source.Status
	comp.Situation = source.Situation
	comp.Venue.FullName = source.Venue.FullName
	for _, player := range source.Competitors {
		var side competitor
		side.HomeAway = player.HomeAway
		side.Score = player.Score.DisplayValue
		side.Team.ID = player.Team.ID
		side.Team.Name = player.Team.Name
		side.Team.DisplayName = player.Team.DisplayName
		side.Team.Abbreviation = player.Team.Abbreviation
		for _, line := range player.Linescores {
			side.Linescores = append(side.Linescores, line)
		}
		comp.Competitors = append(comp.Competitors, side)
	}
	out.Competitions = []competition{comp}
	return out
}

func (g scheduledGame) state() string {
	if len(g.Competitions) == 0 {
		return "pre"
	}
	return g.Competitions[0].Status.Type.State
}

func (g scheduledGame) matchup(team string) (opponent, location, score string) {
	if len(g.Competitions) == 0 {
		return "-", "", ""
	}
	var own, other *scheduleCompetitor
	for i := range g.Competitions[0].Competitors {
		c := &g.Competitions[0].Competitors[i]
		if c.Team.Abbreviation == team {
			own = c
		} else {
			other = c
		}
	}
	if own == nil || other == nil {
		return "-", "", ""
	}
	location = "vs"
	if own.HomeAway == "away" {
		location = "@"
	}
	opponent = other.Team.Abbreviation
	if own.Score.DisplayValue != "" && other.Score.DisplayValue != "" {
		score = own.Score.DisplayValue + "-" + other.Score.DisplayValue
		if g.state() == "post" {
			a, _ := strconv.Atoi(own.Score.DisplayValue)
			b, _ := strconv.Atoi(other.Score.DisplayValue)
			if a > b {
				score = "W " + score
			} else if a < b {
				score = "L " + score
			} else {
				score = "T " + score
			}
		}
	}
	return
}

func (c *client) teamSchedule(ctx context.Context, team string, season int) (teamSchedule, error) {
	var out teamSchedule
	path := "/teams/" + url.PathEscape(strings.ToLower(team)) + "/schedule"
	cachedAt, err := c.get(ctx, path, url.Values{"season": {strconv.Itoa(season)}}, &out)
	if err != nil {
		return out, err
	}
	out.CachedAt = cachedAt
	if len(out.Events) == 0 {
		return out, fmt.Errorf("schedule unavailable for %s", team)
	}
	sort.SliceStable(out.Events, func(i, j int) bool { return out.Events[i].dateTime().Before(out.Events[j].dateTime()) })
	return out, nil
}

type seasonLeader struct {
	Category, Player  string
	Yards, Touchdowns int
}

func (c *client) teamLeaders(ctx context.Context, team string, schedule teamSchedule) ([]seasonLeader, error) {
	type totals struct{ yards, touchdowns int }
	byCategory := map[string]map[string]*totals{}
	for _, category := range []string{"passing", "rushing", "receiving"} {
		byCategory[category] = map[string]*totals{}
	}
	successes, failures := 0, 0
	for _, event := range schedule.Events {
		if event.state() != "post" {
			continue
		}
		detail, err := c.summary(ctx, event.ID)
		if err != nil {
			failures++
			continue
		}
		successes++
		for _, players := range detail.Boxscore.Players {
			if players.Team.Abbreviation != team {
				continue
			}
			for _, group := range players.Statistics {
				category := group.Name
				if _, ok := byCategory[category]; !ok {
					continue
				}
				yardsIndex, tdIndex := -1, -1
				for i, label := range group.Labels {
					if label == "YDS" {
						yardsIndex = i
					}
					if label == "TD" {
						tdIndex = i
					}
				}
				if yardsIndex < 0 {
					continue
				}
				for _, athlete := range group.Athletes {
					if yardsIndex >= len(athlete.Stats) {
						continue
					}
					name := athlete.Athlete.DisplayName
					if name == "" {
						continue
					}
					value := byCategory[category][name]
					if value == nil {
						value = &totals{}
						byCategory[category][name] = value
					}
					value.yards += parseStatInt(athlete.Stats[yardsIndex])
					if tdIndex >= 0 && tdIndex < len(athlete.Stats) {
						value.touchdowns += parseStatInt(athlete.Stats[tdIndex])
					}
				}
			}
		}
	}
	var leaders []seasonLeader
	for _, category := range []string{"passing", "rushing", "receiving"} {
		var best seasonLeader
		for player, value := range byCategory[category] {
			if best.Player == "" || value.yards > best.Yards || value.yards == best.Yards && player < best.Player {
				best = seasonLeader{category, player, value.yards, value.touchdowns}
			}
		}
		if best.Player != "" {
			leaders = append(leaders, best)
		}
	}
	if successes == 0 && failures > 0 {
		return leaders, fmt.Errorf("player leaders unavailable")
	}
	if failures > 0 {
		return leaders, fmt.Errorf("%d completed game(s) unavailable for player totals", failures)
	}
	return leaders, nil
}

func parseStatInt(s string) int {
	n, _ := strconv.Atoi(strings.ReplaceAll(s, ",", ""))
	return n
}
