package main

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const leadersURL = "https://site.web.api.espn.com/apis/site/v3/sports/football/nfl/leaders"

// leaderOrder is every category shown, in order. ESPN also sends kicking and
// punting categories, which are left out.
var leaderOrder = []string{
	"passingYards", "passingTouchdowns", "quarterbackRating",
	"rushingYards", "rushingTouchdowns",
	"receivingYards", "receptions", "receivingTouchdowns",
	"totalTackles", "sacks", "interceptions", "totalPoints",
}

type leaderRow struct {
	AthleteID string
	Name      string
	Position  string
	Team      string
	Value     string
}

type leaderCategory struct {
	Title string
	Rows  []leaderRow
}

type leagueLeaders struct {
	Season     int
	Categories []leaderCategory
	CachedAt   time.Time
}

type leadersMsg struct {
	season int
	data   leagueLeaders
	err    error
}

func (c *client) leaders(ctx context.Context, season, seasonType int) (leagueLeaders, error) {
	var response struct {
		Leaders struct {
			Categories []struct {
				Name        string `json:"name"`
				DisplayName string `json:"displayName"`
				Leaders     []struct {
					DisplayValue string `json:"displayValue"`
					Athlete      struct {
						ID          string `json:"id"`
						DisplayName string `json:"displayName"`
						Position    struct {
							Abbreviation string `json:"abbreviation"`
						} `json:"position"`
					} `json:"athlete"`
					Team struct {
						Abbreviation string `json:"abbreviation"`
					} `json:"team"`
				} `json:"leaders"`
			} `json:"categories"`
		} `json:"leaders"`
	}
	query := url.Values{"season": {strconv.Itoa(season)}, "seasontype": {strconv.Itoa(seasonType)}, "limit": {"5"}}
	cachedAt, err := c.getURL(ctx, leadersURL+"?"+query.Encode(), &response)
	if err != nil {
		return leagueLeaders{}, err
	}
	out := leagueLeaders{Season: season, CachedAt: cachedAt}
	for _, name := range leaderOrder {
		for _, category := range response.Leaders.Categories {
			if category.Name != name || len(category.Leaders) == 0 {
				continue
			}
			parsed := leaderCategory{Title: plainText(category.DisplayName)}
			for _, leader := range category.Leaders {
				parsed.Rows = append(parsed.Rows, leaderRow{
					AthleteID: leader.Athlete.ID,
					Name:      plainText(leader.Athlete.DisplayName),
					Position:  plainText(leader.Athlete.Position.Abbreviation),
					Team:      plainText(leader.Team.Abbreviation),
					Value:     plainText(leader.DisplayValue),
				})
			}
			out.Categories = append(out.Categories, parsed)
		}
	}
	return out, nil
}

func (m model) openLeaders() (model, tea.Cmd) {
	origin := "dashboard"
	if m.showHome {
		origin = "home"
	}
	m = m.consumeOverview().push(backTarget{Screen: origin})
	m.showHome = false
	m.showLeaders = true
	return m.requestLeaders(false)
}

func (m model) requestLeaders(force bool) (model, tea.Cmd) {
	if m.season == 0 || m.leadersLoading || (!force && m.leadersError == "" && m.leaderData.Season == m.season) {
		return m, nil
	}
	m.leadersLoading = true
	season, seasonType := m.season, m.seasonType
	if seasonType != 3 {
		seasonType = 2
	}
	return m, func() tea.Msg {
		data, err := m.api.leaders(context.Background(), season, seasonType)
		return leadersMsg{season: season, data: data, err: err}
	}
}

func (m model) applyLeaders(msg leadersMsg) model {
	m.leadersLoading = false
	if msg.err != nil {
		m.leadersError = plainText(msg.err.Error())
		return m
	}
	m.leaderData, m.leadersError = msg.data, ""
	m.leaderSelected = max(0, min(m.leaderSelected, len(m.leaderRows())-1))
	return m
}

// leaderRows is every selectable player in render order.
func (m model) leaderRows() []leaderRow {
	var rows []leaderRow
	for _, category := range m.leaderData.Categories {
		rows = append(rows, category.Rows...)
	}
	return rows
}

// leaderRender returns the screen's lines and the line of each selectable row.
func (m model) leaderRender(width int) ([]string, []int) {
	if len(m.leaderData.Categories) == 0 {
		switch {
		case m.leadersError != "":
			return []string{liveStyle.Render(clipText("Leaders: "+m.leadersError, width))}, nil
		case m.season > 0 && !m.leadersLoading:
			return []string{dimStyle.Render("No leaders yet this season.")}, nil
		}
		return []string{dimStyle.Render("Loading league leaders…")}, nil
	}
	var lines []string
	if notice := staleLine(m.leaderData.CachedAt, m.leadersError); notice != "" {
		lines = append(lines, liveStyle.Render(clipText(notice, width)))
	}
	var rowLines []int
	index := 0
	for n, category := range m.leaderData.Categories {
		if n > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, sectionStyle.Render(strings.ToUpper(category.Title)))
		for rank, row := range category.Rows {
			text := clipText(leaderText(rank+1, row), width)
			switch {
			case index == m.leaderSelected:
				text = selectedStyle.Render(padLine(text, width))
			case row.Team == m.favorite:
				text = accentStyle.Render(text)
			default:
				text = textStyle.Render(text)
			}
			rowLines = append(rowLines, len(lines))
			lines = append(lines, text)
			index++
		}
	}
	return lines, rowLines
}

func leaderText(rank int, row leaderRow) string {
	return fmt.Sprintf("%d  %-24s %-4s %-4s %8s", rank, clipText(row.Name, 24), row.Position, row.Team, row.Value)
}

func (m model) moveLeaderSelection(delta int) model {
	rows := len(m.leaderRows())
	if rows == 0 {
		return m
	}
	m.leaderSelected = max(0, min(m.leaderSelected+delta, rows-1))
	lines, rowLines := m.leaderRender(m.leagueWidth())
	maxScroll := max(0, len(lines)-m.leaguePageSize())
	line := rowLines[m.leaderSelected]
	// The section title sits above the first row of each category.
	if line <= 1 {
		line = 0
	}
	m.leaderScroll = keepVisible(line, m.leaderScroll, m.leaguePageSize(), maxScroll)
	return m
}

func (m model) updateLeadersKey(key string) (model, tea.Cmd) {
	switch key {
	case "up", "k":
		return m.moveLeaderSelection(-1), nil
	case "down", "j":
		return m.moveLeaderSelection(1), nil
	case "pgup":
		return m.moveLeaderSelection(-m.leaguePageSize()), nil
	case "pgdown":
		return m.moveLeaderSelection(m.leaguePageSize()), nil
	case "enter":
		rows := m.leaderRows()
		if m.leaderSelected >= len(rows) {
			return m, nil
		}
		row := rows[m.leaderSelected]
		ref := playerRef{ID: row.AthleteID, Name: row.Name, Position: row.Position, Team: row.Team, TeamName: row.Team, Jersey: "--"}
		if team, ok := findTeam(row.Team); ok {
			ref.TeamName = team.city + " " + team.nickname
		}
		return m.openPlayer(ref)
	case "r":
		return m.requestLeaders(true)
	case "esc", "backspace":
		return m.back()
	}
	return m, nil
}

func (m model) renderLeaders(width int) string {
	title := "◆  LEAGUE LEADERS"
	if m.season > 0 {
		title = fmt.Sprintf("◆  LEAGUE LEADERS  ·  %d", m.season)
	}
	lines, _ := m.leaderRender(m.leagueWidth())
	footer := "↑/↓ select  ·  Enter player  ·  r refresh  ·  Esc back  ·  q quit"
	return m.renderLeaguePage(title, "", lines, m.leaderScroll, footer, "↑/↓ · Enter · Esc · q", width)
}
