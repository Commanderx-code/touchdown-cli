package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m model) renderBoard(inner int) string {
	if m.width >= 112 && m.height >= 34 {
		return m.wideDashboard()
	}
	return m.narrowDashboard(inner)
}

func (m model) dashboardSummary(width int) string {
	live, final, upcoming := 0, 0, 0
	for _, g := range m.board.Events {
		switch g.Status.Type.State {
		case "in":
			live++
		case "post":
			final++
		default:
			upcoming++
		}
	}
	parts := []string{
		liveStyle.Render(fmt.Sprintf("● %d LIVE", live)),
		accentStyle.Render(fmt.Sprintf("✓ %d FINAL", final)),
		dimStyle.Render(fmt.Sprintf("◷ %d UPCOMING", upcoming)),
	}
	if m.favorite != "" {
		favorite := "★ " + favoriteLabel(m.favorite)
		if record := m.favoriteRecord(); record != "" {
			favorite += " " + record
		}
		parts = append(parts, sectionStyle.Render(favorite))
	}
	if !m.board.CachedAt.IsZero() {
		parts = append(parts, liveStyle.Render("CACHED "+m.board.CachedAt.Local().Format("3:04 PM")))
	}
	return centerLine(clipStyled(strings.Join(parts, "    "), width), width)
}

func (m model) favoriteMatch() (game, bool) { return favoriteGame(m.board, m.favorite) }

func (m model) favoriteRecord() string {
	if m.favorite == "" {
		return ""
	}
	if m.standingsSeason == m.season {
		for _, group := range m.standings.Children {
			for _, e := range group.Standings.Entries {
				if e.Team.Abbreviation != m.favorite {
					continue
				}
				record := e.stat("wins") + "-" + e.stat("losses")
				if ties := e.stat("ties"); ties != "0" && ties != "-" {
					record += "-" + ties
				}
				return record
			}
		}
	}
	if g, ok := m.favoriteMatch(); ok {
		away, home := g.sides()
		for _, team := range []competitor{away, home} {
			if team.Team.Abbreviation != m.favorite {
				continue
			}
			for _, record := range team.Records {
				if record.Name == "overall" {
					return record.Summary
				}
			}
		}
	}
	return ""
}

// spotlightPanel renders the favorite-team card. maxLines bounds its inner
// line count; live detail (situation, timeouts, latest play) yields first,
// latest play before timeouts before situation.
func (m model) spotlightPanel(inner, maxLines int) string {
	label := favoriteLabel(m.favorite)
	week, hasWeek := m.favoriteMatch()
	if hasWeek {
		away, home := week.sides()
		if away.Team.Abbreviation == m.favorite {
			label = strings.ToUpper(teamName(away))
		}
		if home.Team.Abbreviation == m.favorite {
			label = strings.ToUpper(teamName(home))
		}
	}
	heading := "★  " + label + " WATCH"
	if m.favorite == "" {
		heading = "★  FAVORITE TEAM"
	}
	if record := m.favoriteRecord(); record != "" {
		heading += "  ·  " + record
	}
	lines := []string{accentStyle.Render(clipText(heading, inner))}
	next, last := m.favoriteScheduleGames()
	switch {
	case next != nil:
		opponent, location, _ := next.matchup(m.favorite)
		matchup := fmt.Sprintf("NEXT  W%d  %s %s %s", next.Week.Number, m.favorite, location, opponent)
		status := nextKickoff(*next, inner)
		if hasWeek && week.ID == next.ID && week.Status.Type.State == "in" {
			away, home := week.sides()
			matchup = fmt.Sprintf("● LIVE  %s %s – %s %s", away.Team.Abbreviation, away.Score, home.Score, home.Team.Abbreviation)
			status = strings.ToUpper(week.statusText())
		} else if next.state() == "in" {
			matchup = fmt.Sprintf("● LIVE  W%d  %s %s %s", next.Week.Number, m.favorite, location, opponent)
		}
		if !hasWeek && m.week == m.teamData.ByeWeek {
			kickoff := "Kickoff TBD"
			if !next.dateTime().IsZero() {
				kickoff = next.dateTime().Local().Format("1/2 3:04 PM")
			}
			status = fmt.Sprintf("BYE WEEK %d  ·  %s", m.week, kickoff)
		}
		lines = append(lines, textStyle.Bold(true).Render(clipText(matchup, inner)))
		if strings.Contains(matchup, "LIVE") {
			lines = append(lines, liveStyle.Render(clipText(status, inner)))
		} else {
			lines = append(lines, dimStyle.Render(clipText(status, inner)))
		}
	case hasWeek && (week.Status.Type.State != "post" || m.teamSeason != m.season):
		away, home := week.sides()
		matchup := fmt.Sprintf("%s %s  @  %s %s", away.Team.Abbreviation, teamName(away), home.Team.Abbreviation, teamName(home))
		if week.Status.Type.State != "pre" {
			matchup = fmt.Sprintf("%s %s  –  %s %s", away.Team.Abbreviation, away.Score, home.Score, home.Team.Abbreviation)
		}
		lines = append(lines, textStyle.Bold(true).Render(clipText(matchup, inner)))
		status := strings.ToUpper(week.statusText())
		if week.Status.Type.State == "in" {
			status = "● LIVE  " + status
			lines = append(lines, liveStyle.Render(clipText(status, inner)))
		} else {
			lines = append(lines, dimStyle.Render(clipText(status, inner)))
		}
	case m.teamSeason == m.season && len(m.teamData.Events) > 0:
		lines = append(lines, textStyle.Render("Season schedule complete"))
	case m.teamLoading:
		lines = append(lines, dimStyle.Render("Loading team schedule…"))
	case m.teamError != "":
		lines = append(lines, dimStyle.Render("Team schedule unavailable · r to retry"))
	default:
		message := "No " + favoriteLabel(m.favorite) + " game this week"
		if m.favorite == "" {
			message = "Set a team with -favorite GB"
		}
		lines = append(lines, textStyle.Render(clipText(message, inner)))
		lines = append(lines, dimStyle.Render("Use ←/→ to browse other weeks"))
	}
	if injuries := m.spotlightInjuryLine(); injuries != "" {
		lines = append(lines, dimStyle.Render(clipText(injuries, inner)))
	}
	var live []string
	if hasWeek && week.Status.Type.State == "in" {
		live = spotlightLiveLines(week, inner)
	}
	lastLine := ""
	if last != nil {
		opponent, location, score := last.matchup(m.favorite)
		if score != "" {
			lastLine = fmt.Sprintf("LAST  %s  %s %s", score, location, opponent)
		}
	}
	if hasWeek && week.Status.Type.State == "post" && (last == nil || last.ID == week.ID || week.dateTime().After(last.dateTime())) {
		lastLine = favoriteGameResult(week, m.favorite)
	}
	reserved := len(lines)
	if lastLine != "" {
		reserved++
	}
	if maxLines > 0 {
		live = live[:max(0, min(len(live), maxLines-reserved))]
	}
	lines = append(lines, live...)
	if lastLine != "" {
		lines = append(lines, sectionStyle.Render(clipText(lastLine, inner)))
	}
	return framedLines(lines, inner)
}

// spotlightInjuryLine is "INJURIES  2 OUT · 1 Q" for the favorite, or ""
// while loading or when nobody is Out, Doubtful or Questionable.
func (m model) spotlightInjuryLine() string {
	entry, ok := m.injuriesFor(m.favorite)
	if !ok || entry.Report.Team == "" {
		return ""
	}
	if summary := entry.Report.gameDaySummary(); summary != "" {
		return "INJURIES  " + summary
	}
	return ""
}

// lineCount is the number of terminal lines a rendered block occupies.
func lineCount(block string) int {
	if block == "" {
		return 0
	}
	return strings.Count(block, "\n") + 1
}

// feedPanel renders the newest scoring events, at most entries of them.
func (m model) feedPanel(inner, entries int) string {
	lines := []string{sectionStyle.Render("SCORING FEED"), borderStyle.Render(strings.Repeat("─", inner))}
	if len(m.feed) == 0 {
		lines = append(lines, dimStyle.Render("No scores yet this session"))
		return framedLines(lines, inner)
	}
	for _, e := range m.feed[:min(entries, len(m.feed))] {
		line := clipText(e.feedLine(), inner)
		if e.Favorite {
			lines = append(lines, accentStyle.Render(line))
		} else {
			lines = append(lines, textStyle.Render(line))
		}
	}
	return framedLines(lines, inner)
}

// feedLineNarrow is the single newest feed entry for the narrow dashboard,
// or "" when the feed is empty.
func (m model) feedLineNarrow(width int) string {
	if len(m.feed) == 0 {
		return ""
	}
	return centerLine(liveStyle.Render(clipText("⚡ "+m.feed[0].feedLine(), width)), width)
}

// panelBudget is the number of lines View leaves for the dashboard panel:
// header, subhead, a blank line, a blank line and the footer surround it.
func (m model) panelBudget() int {
	if m.height <= 0 {
		return 1 << 20
	}
	return m.height - 5
}

// spotlightLiveLines adds the situation, timeouts and latest play (at most
// two lines) for the favorite team's game in progress.
func spotlightLiveLines(g game, inner int) []string {
	var lines []string
	if situation := situationText(g); situation != "" {
		lines = append(lines, liveStyle.Render(clipText(situation, inner)))
	}
	if timeouts := timeoutsText(g); timeouts != "" {
		lines = append(lines, dimStyle.Render(clipText(timeouts, inner)))
	}
	if latest := latestPlayText(g); latest != "" {
		wrapped := wrapWords(latest, inner)
		for _, row := range wrapped[:min(2, len(wrapped))] {
			lines = append(lines, dimStyle.Render(clipText(row, inner)))
		}
	}
	return lines
}

func (m model) favoriteScheduleGames() (next, last *scheduledGame) {
	if m.teamSeason != m.season {
		return nil, nil
	}
	week, hasWeek := m.favoriteMatch()
	for i := range m.teamData.Events {
		g := &m.teamData.Events[i]
		state := g.state()
		if hasWeek && week.ID == g.ID {
			state = week.Status.Type.State
		}
		if state == "post" {
			last = g
		} else if next == nil {
			next = g
		}
	}
	return next, last
}

func nextKickoff(g scheduledGame, inner int) string {
	if g.dateTime().IsZero() {
		return "Kickoff TBD"
	}
	if inner < 42 {
		return g.dateTime().Local().Format("Mon Jan 2, 3:04 PM")
	}
	return g.dateTime().Local().Format("Mon Jan 2, 3:04 PM MST")
}

func favoriteGameResult(g game, favorite string) string {
	away, home := g.sides()
	own, other, location := away, home, "@"
	if home.Team.Abbreviation == favorite {
		own, other, location = home, away, "vs"
	}
	if own.Team.Abbreviation != favorite || own.Score == "" || other.Score == "" {
		return ""
	}
	a, errA := strconv.Atoi(own.Score)
	b, errB := strconv.Atoi(other.Score)
	if errA != nil || errB != nil {
		return ""
	}
	result := "T"
	if a > b {
		result = "W"
	} else if a < b {
		result = "L"
	}
	return fmt.Sprintf("LAST  %s %s-%s  %s %s", result, own.Score, other.Score, location, other.Team.Abbreviation)
}

func (m model) favoriteConference() (string, []standingEntry) {
	if m.standingsSeason != m.season {
		if team, ok := findTeam(m.favorite); ok {
			return team.conference, nil
		}
		return "NFC", nil
	}
	for _, group := range m.standings.Children {
		for _, e := range group.Standings.Entries {
			if e.Team.Abbreviation == m.favorite {
				return group.Abbreviation, group.Standings.Entries
			}
		}
	}
	if team, ok := findTeam(m.favorite); ok {
		return team.conference, nil
	}
	return "NFC", nil
}

func (m model) snapshotPanel(inner int) string {
	conference, entries := m.favoriteConference()
	lines := []string{sectionStyle.Render(conference + " SNAPSHOT"), borderStyle.Render(strings.Repeat("─", inner))}
	if len(entries) == 0 {
		message := "Loading standings…"
		if m.standingsError != "" {
			message = "Standings unavailable"
		}
		lines = append(lines, dimStyle.Render(message))
		return framedLines(lines, inner)
	}
	indices := []int{}
	for i := 0; i < min(5, len(entries)); i++ {
		indices = append(indices, i)
	}
	favoriteIndex := -1
	for i, e := range entries {
		if e.Team.Abbreviation == m.favorite {
			favoriteIndex = i
			break
		}
	}
	if favoriteIndex >= 5 && len(indices) == 5 {
		indices[4] = favoriteIndex
	}
	for _, i := range indices {
		e := entries[i]
		marker := " "
		if e.Team.Abbreviation == m.favorite {
			marker = "★"
		}
		name := e.Team.DisplayName
		words := strings.Fields(name)
		if len(words) > 0 {
			name = words[len(words)-1]
		}
		record := e.stat("wins") + "-" + e.stat("losses")
		line := fmt.Sprintf("%s %2d  %-3s %-13s %5s", marker, i+1, e.Team.Abbreviation, clipText(name, 13), record)
		if e.Team.Abbreviation == m.favorite {
			lines = append(lines, accentStyle.Render(clipText(line, inner)))
		} else {
			lines = append(lines, dimStyle.Render(clipText(line, inner)))
		}
	}
	return framedLines(lines, inner)
}

func (m model) narrowDashboard(inner int) string {
	width := m.width
	if width <= 0 {
		width = 80
	}
	feedLine := m.feedLineNarrow(width)
	// Reserve the summary, the feed line, the list frame and two header rows,
	// and keep at least one game row (two lines) when sizing the spotlight.
	spotlightMax := m.panelBudget() - 1 - lineCount(feedLine) - 2 - 2 - 2 - 2
	spotlight := m.spotlightPanel(inner, max(3, spotlightMax))
	rows := 0 // no cap without a known height
	if m.height > 0 {
		used := 1 + lineCount(feedLine) + lineCount(spotlight) + 4
		rows = max(1, (m.panelBudget()-used)/2)
	}
	parts := []string{m.dashboardSummary(width)}
	if feedLine != "" {
		parts = append(parts, feedLine)
	}
	parts = append(parts, centerBlock(spotlight, width), centerBlock(m.scoreboardPanel(inner, rows), width))
	return strings.Join(parts, "\n")
}

func (m model) wideDashboard() string {
	leftInner := min(74, max(58, m.width*58/100-4))
	rightInner := min(48, max(32, m.width-leftInner-15))
	budget := m.panelBudget() - 2 // summary line and the blank line below it
	snapshot := m.snapshotPanel(rightInner)
	// Smallest feed panel: frame, header, rule, one entry.
	const minFeed = 5
	spotlight := m.spotlightPanel(rightInner, budget-lineCount(snapshot)-minFeed-2)
	remaining := budget - lineCount(spotlight) - lineCount(snapshot) - 4
	right := []string{spotlight}
	if len(m.feed) == 0 {
		if remaining >= 1 {
			right = append(right, m.feedPanel(rightInner, 0))
		}
	} else if entries := min(6, len(m.feed), remaining); entries >= 1 {
		right = append(right, m.feedPanel(rightInner, entries))
	}
	right = append(right, snapshot)
	rows := 0 // no cap without a known height
	if m.height > 0 {
		rows = max(1, (budget-4)/2)
	}
	left := m.scoreboardPanel(leftInner, rows)
	return m.dashboardSummary(m.width) + "\n\n" + centerBlock(joinBlocks(left, strings.Join(right, "\n"), 3), m.width)
}

func joinBlocks(left, right string, gap int) string {
	leftLines, rightLines := strings.Split(left, "\n"), strings.Split(right, "\n")
	leftWidth := 0
	for _, line := range leftLines {
		leftWidth = max(leftWidth, lipgloss.Width(line))
	}
	var b strings.Builder
	for i := 0; i < max(len(leftLines), len(rightLines)); i++ {
		if i > 0 {
			b.WriteByte('\n')
		}
		line := ""
		if i < len(leftLines) {
			line = leftLines[i]
		}
		b.WriteString(padLine(line, leftWidth))
		b.WriteString(strings.Repeat(" ", gap))
		if i < len(rightLines) {
			b.WriteString(rightLines[i])
		}
	}
	return b.String()
}
