package main

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const overviewWeeks = 18

// overviewWeek is one regular-season week's row in the season overview.
type overviewWeek struct {
	Week     int
	Loaded   bool
	Loading  bool
	Err      string
	Games    int
	Favorite string
	Byes     []string
}

// overviewMsg carries one week's scoreboard for the overview only. It is a
// separate message type so overview fetches never touch the dashboard board,
// alerts or the scoring feed.
type overviewMsg struct {
	season, week int
	board        scoreboard
	err          error
}

// summarizeWeek reduces a week's scoreboard to its overview row.
func summarizeWeek(board scoreboard, favorite string, week int, now time.Time) overviewWeek {
	w := overviewWeek{Week: week, Loaded: true, Games: len(board.Events)}
	playing := map[string]bool{}
	for _, g := range board.Events {
		away, home := g.sides()
		playing[away.Team.Abbreviation] = true
		playing[home.Team.Abbreviation] = true
	}
	for _, team := range nflTeams {
		if !playing[team.abbreviation] {
			w.Byes = append(w.Byes, team.abbreviation)
		}
	}
	sort.Strings(w.Byes)
	if favorite == "" {
		return w
	}
	w.Favorite = favorite + " BYE"
	for _, g := range board.Events {
		away, home := g.sides()
		if away.Team.Abbreviation != favorite && home.Team.Abbreviation != favorite {
			continue
		}
		own, other, location := away, home, "@"
		if home.Team.Abbreviation == favorite {
			own, other, location = home, away, "vs"
		}
		w.Favorite = fmt.Sprintf("%s %s %s", favorite, location, other.Team.Abbreviation)
		if g.Status.Type.State == "post" {
			if result := scoreResult(own.Score, other.Score); result != "" {
				w.Favorite = fmt.Sprintf("%s %s %s-%s %s", favorite, result, own.Score, other.Score, other.Team.Abbreviation)
			}
		} else if g.Status.Type.State == "pre" {
			if kickoff := g.dateTime(); !kickoff.IsZero() {
				w.Favorite += "  " + kickoff.In(now.Location()).Format("Mon 3:04 PM")
			}
		} else if g.Status.Type.State == "in" {
			w.Favorite += "  LIVE"
		}
		break
	}
	return w
}

func scoreResult(own, other string) string {
	a, errA := strconv.Atoi(own)
	b, errB := strconv.Atoi(other)
	if errA != nil || errB != nil {
		return ""
	}
	switch {
	case a > b:
		return "W"
	case a < b:
		return "L"
	}
	return "T"
}

// line renders the row: "W12  14 GAMES  ·  GB @ DET  Thu 12:30 PM  ·  BYE: KC, NE".
func (w overviewWeek) line(favorite string) string {
	label := fmt.Sprintf("W%-2d  ", w.Week)
	switch {
	case w.Err != "":
		return label + "unavailable · r to retry"
	case !w.Loaded:
		return label + "…"
	}
	parts := []string{fmt.Sprintf("%d GAMES", w.Games)}
	if w.Favorite != "" {
		if strings.HasSuffix(w.Favorite, " BYE") {
			parts = append([]string{w.Favorite}, parts...)
		} else {
			parts = append(parts, w.Favorite)
		}
	}
	if len(w.Byes) > 0 {
		byes := w.Byes
		extra := ""
		if len(byes) > 6 {
			extra = fmt.Sprintf(" +%d", len(byes)-6)
			byes = byes[:6]
		}
		parts = append(parts, "BYE: "+strings.Join(byes, ", ")+extra)
	}
	return plainText(label + strings.Join(parts, "  ·  "))
}

// openOverview shows the season overview with the cursor on the dashboard's
// week and starts loading the weeks around it.
func (m model) openOverview() (model, tea.Cmd) {
	origin := "dashboard"
	if m.showHome {
		origin = "home"
	}
	m = m.consumeOverview().push(backTarget{Screen: origin})
	m.showHome = false
	m.showOverview = true
	if m.season == 0 {
		return m, nil
	}
	m = m.ensureOverview()
	m.overviewCursor = max(1, min(overviewWeeks, m.week))
	m.overviewScroll = m.overviewScrollFor(m.overviewCursor)
	return m.overviewFetches(2)
}

// ensureOverview resets the week cache when the season or the favorite it
// was summarized for changed, and places the cursor when it has none yet.
func (m model) ensureOverview() model {
	if m.overviewSeason != m.season || m.overviewFavorite != m.favorite || m.overview == nil {
		m.overviewSeason = m.season
		m.overviewFavorite = m.favorite
		m.overview = map[int]overviewWeek{}
	}
	if m.overviewCursor == 0 {
		m.overviewCursor = max(1, min(overviewWeeks, m.week))
		m.overviewScroll = m.overviewScrollFor(m.overviewCursor)
	}
	return m
}

// seasonKnown runs when the first scoreboard arrives while the overview is
// open, so its rows start loading instead of staying blank.
func (m model) seasonKnown() (model, tea.Cmd) {
	if !m.showOverview || m.season == 0 {
		return m, nil
	}
	m = m.ensureOverview()
	m.overviewCursor = max(1, min(overviewWeeks, m.week))
	m.overviewScroll = m.overviewScrollFor(m.overviewCursor)
	return m.overviewFetches(2)
}

// overviewFetches requests the weeks within reach of the cursor that are
// neither loaded nor in flight.
func (m model) overviewFetches(reach int) (model, tea.Cmd) {
	if m.season == 0 {
		return m, nil
	}
	m = m.ensureOverview()
	var cmds []tea.Cmd
	for week := m.overviewCursor - reach; week <= m.overviewCursor+reach; week++ {
		if week < 1 || week > overviewWeeks {
			continue
		}
		w := m.overview[week]
		if w.Loaded || w.Loading || w.Err != "" {
			continue
		}
		w.Week, w.Loading = week, true
		m.overview[week] = w
		cmds = append(cmds, m.loadOverviewWeek(week))
	}
	if len(cmds) == 0 {
		return m, nil
	}
	return m, tea.Batch(cmds...)
}

func (m model) loadOverviewWeek(week int) tea.Cmd {
	season := m.season
	return func() tea.Msg {
		board, err := m.api.scoreboard(context.Background(), season, week, 2)
		return overviewMsg{season: season, week: week, board: board, err: err}
	}
}

func (m model) applyOverview(msg overviewMsg) model {
	if msg.season != m.overviewSeason || m.overview == nil {
		return m
	}
	if msg.err != nil {
		m.overview[msg.week] = overviewWeek{Week: msg.week, Err: msg.err.Error()}
		return m
	}
	m.overview[msg.week] = summarizeWeek(msg.board, m.favorite, msg.week, time.Now())
	return m
}

func (m model) overviewPageSize() int {
	height := m.height
	if height <= 0 {
		height = 24
	}
	return max(1, min(overviewWeeks, height-9))
}

func (m model) overviewScrollFor(cursor int) int {
	return max(0, min(cursor-1-m.overviewPageSize()/2, overviewWeeks-m.overviewPageSize()))
}

// moveOverviewCursor moves by delta weeks, keeps the cursor visible and
// fetches the weeks beside it.
func (m model) moveOverviewCursor(delta int) (model, tea.Cmd) {
	if m.season == 0 {
		return m, nil
	}
	m = m.ensureOverview()
	m.overviewCursor = max(1, min(overviewWeeks, m.overviewCursor+delta))
	page := m.overviewPageSize()
	if m.overviewCursor-1 < m.overviewScroll {
		m.overviewScroll = m.overviewCursor - 1
	}
	if m.overviewCursor-1 >= m.overviewScroll+page {
		m.overviewScroll = m.overviewCursor - page
	}
	return m.overviewFetches(1)
}

func (m model) updateOverviewKey(key string) (model, tea.Cmd) {
	if m.season == 0 {
		if key == "esc" || key == "backspace" {
			return m.back()
		}
		return m, nil
	}
	m = m.ensureOverview()
	switch key {
	case "up", "k":
		return m.moveOverviewCursor(-1)
	case "down", "j":
		return m.moveOverviewCursor(1)
	case "pgup":
		return m.moveOverviewCursor(-m.overviewPageSize())
	case "pgdown":
		return m.moveOverviewCursor(m.overviewPageSize())
	case "home":
		return m.moveOverviewCursor(-overviewWeeks)
	case "end":
		return m.moveOverviewCursor(overviewWeeks)
	case "r":
		for week := m.overviewCursor - 1; week <= m.overviewCursor+1; week++ {
			if week >= 1 && week <= overviewWeeks {
				m.overview[week] = overviewWeek{Week: week}
			}
		}
		return m.overviewFetches(1)
	case "enter":
		m = m.push(backTarget{Screen: "overview"})
		m.showOverview = false
		m.seasonType = 2
		m.week = m.overviewCursor
		m.feed, m.feedPending = nil, nil
		m.selected = 0
		m.loading = true
		m.errorText = ""
		return m, m.loadBoard()
	case "esc", "backspace":
		return m.back()
	}
	return m, nil
}

func (m model) renderOverview(width int) string {
	height := m.height
	if height <= 0 {
		height = 24
	}
	title := accentStyle.Render("◆  SEASON OVERVIEW")
	if m.season > 0 {
		title = accentStyle.Render(fmt.Sprintf("◆  SEASON OVERVIEW  ·  %d", m.season))
	}
	if m.season == 0 {
		return clipStyled(title, width) + "\n" + dimStyle.Render("Loading season…") + "\n" + dimStyle.Render("Esc back  ·  q quit")
	}
	page := m.overviewPageSize()
	start := max(0, min(m.overviewScroll, overviewWeeks-page))
	end := min(overviewWeeks, start+page)
	compact := m.compact || height < 14 || width < 40
	inner := width
	if !compact {
		inner = min(90, width-4)
	}
	var rows []string
	for week := start + 1; week <= end; week++ {
		w := m.overview[week]
		if w.Week == 0 {
			w.Week = week
		}
		row := clipText(w.line(m.favorite), inner)
		if week == m.overviewCursor {
			row = "› " + row
		} else {
			row = "  " + row
		}
		row = clipText(row, inner)
		if week == m.overviewCursor {
			rows = append(rows, selectedStyle.Render(padLine(row, inner)))
		} else {
			rows = append(rows, textStyle.Render(row))
		}
	}
	footer := "↑/↓ week  ·  Enter open week  ·  r refresh  ·  Esc back  ·  q quit"
	if compact {
		lines := append([]string{clipStyled(title, width)}, rows...)
		lines = append(lines, dimStyle.Render(clipText(footer, width)))
		return strings.Join(lines, "\n")
	}
	lines := []string{title, dimStyle.Render(fmt.Sprintf("REGULAR SEASON  ·  W%d–W%d OF %d", start+1, end, overviewWeeks)), borderStyle.Render(strings.Repeat("─", inner))}
	lines = append(lines, rows...)
	lines = append(lines, borderStyle.Render(strings.Repeat("─", inner)), dimStyle.Render(footer))
	for i := range lines {
		lines[i] = clipStyled(lines[i], inner)
	}
	return centerBlock(framedLines(lines, inner), width)
}
