package main

import (
	"context"
	"net/url"
	"strings"
	"time"
)

const athleteBase = "https://site.web.api.espn.com/apis/common/v3/sports/football/nfl/athletes/"

// statSection is one stat category's labels and this season's values.
type statSection struct {
	Title  string // PASSING
	Split  string // "" for the regular season, else the split name (CAREER)
	Labels []string
	Values []string
}

type playerStats struct {
	ID       string
	Sections []statSection
	CachedAt time.Time
}

func (c *client) playerOverview(ctx context.Context, id string) (playerStats, error) {
	var response struct {
		Statistics *struct {
			Labels     []string `json:"labels"`
			Categories []struct {
				DisplayName string `json:"displayName"`
				Count       int    `json:"count"`
			} `json:"categories"`
			Splits []struct {
				DisplayName string   `json:"displayName"`
				Stats       []string `json:"stats"`
			} `json:"splits"`
		} `json:"statistics"`
	}
	cachedAt, err := c.getURL(ctx, athleteBase+url.PathEscape(id)+"/overview", &response)
	if err != nil {
		return playerStats{}, err
	}
	out := playerStats{ID: id, CachedAt: cachedAt}
	stats := response.Statistics
	if stats == nil {
		return out, nil
	}
	splitIndex := -1
	for i, split := range stats.Splits {
		if split.DisplayName == "Regular Season" {
			splitIndex = i
			break
		}
	}
	splitName := ""
	if splitIndex < 0 {
		for i, split := range stats.Splits {
			if split.DisplayName != "Projected" {
				splitIndex, splitName = i, strings.ToUpper(plainText(split.DisplayName))
				break
			}
		}
	}
	if splitIndex < 0 {
		return out, nil
	}
	values := stats.Splits[splitIndex].Stats
	start := 0
	for _, category := range stats.Categories {
		if category.Count <= 0 {
			continue
		}
		end := min(start+category.Count, len(stats.Labels))
		if start >= end {
			break
		}
		section := statSection{Title: strings.ToUpper(plainText(category.DisplayName)), Split: splitName}
		for _, label := range stats.Labels[start:end] {
			section.Labels = append(section.Labels, plainText(label))
		}
		for _, value := range values[min(start, len(values)):min(end, len(values))] {
			section.Values = append(section.Values, plainText(value))
		}
		out.Sections = append(out.Sections, section)
		start = end
	}
	return out, nil
}
