package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	borderStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#47615C"))
	textStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#EDF5F2"))
	accentStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#79E4B7"))
	sectionStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#85BBD3"))
)

func seasonLabel(t int) string {
	switch t {
	case 1:
		return "Preseason"
	case 3:
		return "Postseason"
	default:
		return "Regular Season"
	}
}

func (m model) View() string {
	width := m.width
	if width <= 0 {
		width = 80
	}
	if m.showHelp {
		return m.renderHelp(width)
	}
	if m.showSettings {
		return m.renderSettings(width)
	}
	if m.showHome {
		return m.renderHome(width)
	}
	if m.compact || m.height > 0 && m.height < 16 {
		return m.compactView(width)
	}
	inner := max(28, min(74, width-6))
	if inner+4 > width {
		inner = max(12, width-4)
	}

	header := centerLine(accentStyle.Render("◆  TOUCHDOWN"), width)
	season := "NFL COMMAND CENTER  ·  LIVE SCORES AND STATS"
	if m.season > 0 {
		season = fmt.Sprintf("%d %s  ·  WEEK %d", m.season, strings.ToUpper(seasonLabel(m.seasonType)), m.week)
	}
	subhead := centerLine(dimStyle.Render(clipText(season, width)), width)

	var panel string
	if m.showStandings {
		panel = m.renderStandings(inner)
	} else if m.showTeam {
		panel = m.renderTeam(inner)
	} else if m.showDetail {
		panel = m.renderDetail(inner)
	} else {
		panel = m.renderBoard(inner)
	}
	footer := fmt.Sprintf("↑/↓ games  ·  Enter details  ·  g %s  ·  p season  ·  c settings  ·  ? help  ·  f filter  ·  s standings  ·  Esc menu  ·  q quit", m.favorite)
	if m.showStandings {
		footer = "←/→ conference  ·  ↑/↓, PgUp/PgDn, wheel scroll  ·  Esc back  ·  q quit"
	} else if m.showTeam {
		footer = "↑/↓ select game  ·  Enter open  ·  Esc back  ·  r refresh  ·  q quit"
	} else if m.showDetail {
		footer = "1/2/3/4 tabs  ·  ↑/↓ scroll  ·  Esc back  ·  r refresh  ·  q quit"
	}
	if width < 100 && !m.showStandings && !m.showDetail && !m.showTeam {
		footer = fmt.Sprintf("↑/↓ · Enter · g %s · p season · c settings · ? help · s standings · q", m.favorite)
	}
	if width < 68 {
		if m.showStandings {
			footer = "←/→ AFC/NFC · ↑/↓/wheel · Esc · q"
		} else if m.showTeam {
			footer = "↑/↓ game · Enter open · Esc · q"
		} else if m.showDetail {
			footer = "1/2/3/4 tabs · ↑/↓ scroll · Esc back · q"
		} else {
			footer = "↑/↓ · Enter · g game · c settings · ? help · q"
		}
	}
	screen := header + "\n" + subhead + "\n\n" + panel + "\n\n" + centerLine(dimStyle.Render(clipText(footer, width)), width)
	if m.height > 0 {
		free := m.height - strings.Count(screen, "\n") - 1
		if free > 0 {
			screen = strings.Repeat("\n", min(2, free/2)) + screen
		}
	}
	return screen
}

func (m model) compactView(width int) string {
	lines := []string{accentStyle.Render("◆ TOUCHDOWN")}
	if m.showStandings {
		lines = append(lines, sectionStyle.Render("STANDINGS · "+m.conferenceName()+" · "+m.standingsRange()))
		rows := m.standingLines()
		start := min(m.standingsScroll, m.maxStandingsScroll())
		lines = append(lines, rows[start:min(len(rows), start+max(1, m.height-4))]...)
		lines = append(lines, dimStyle.Render("←/→ conference · Esc · q"))
	} else if m.showTeam {
		lines = append(lines, sectionStyle.Render(favoriteLabel(m.favorite)+" SEASON"))
		rows := m.teamLines()
		start := min(m.teamScroll, m.maxTeamScroll())
		lines = append(lines, rows[start:min(len(rows), start+max(1, m.height-4))]...)
		lines = append(lines, dimStyle.Render("↑/↓ game · Enter open · Esc · q"))
	} else if m.showDetail {
		lines = append(lines, sectionStyle.Render("GAME CENTER · "+[]string{"TEAM STATS", "PLAYERS", "PLAY-BY-PLAY", "CENTER"}[m.detailTab]))
		context := m.liveContextLines()
		for _, line := range context {
			lines = append(lines, liveStyle.Render(line))
		}
		stats := m.statLines()
		start := min(m.detailScroll, m.maxDetailScroll())
		end := min(len(stats), start+max(1, m.height-4-len(context)))
		lines = append(lines, stats[start:end]...)
		lines = append(lines, dimStyle.Render("1/2/3/4 tabs · Esc back · q quit"))
	} else {
		games := m.visibleGames()
		lines = append(lines, sectionStyle.Render(fmt.Sprintf("WEEK %d · %d GAMES", m.week, len(games))))
		rows := min(len(games), max(1, m.height-4))
		start := min(max(0, m.selected-rows/2), max(0, len(games)-rows))
		for i := start; i < start+rows; i++ {
			g := games[i]
			away, home := g.sides()
			marker := " "
			if i == m.selected {
				marker = "›"
			}
			line := fmt.Sprintf("%s %s @ %s  %s", marker, away.Team.Abbreviation, home.Team.Abbreviation, g.statusText())
			if g.Status.Type.State != "pre" {
				line = fmt.Sprintf("%s %s %s – %s %s  %s", marker, away.Team.Abbreviation, away.Score, home.Score, home.Team.Abbreviation, g.statusText())
			}
			lines = append(lines, textStyle.Render(line))
		}
		lines = append(lines, dimStyle.Render("↑/↓ · Enter · g game · p season · q"))
	}
	for i, line := range lines {
		lines[i] = clipStyled(line, width)
	}
	return strings.Join(lines, "\n")
}

func (m model) scoreboardPanel(inner, maxRows int) string {
	games := m.visibleGames()
	count := len(games)
	rows := count
	if maxRows > 0 {
		rows = min(count, maxRows)
	}
	start := max(0, m.selected-rows/2)
	start = min(start, max(0, count-rows))
	end := min(count, start+rows)
	meta := fmt.Sprintf("WEEK %d MATCHUPS  ·  %d GAMES", m.week, count)
	if m.season == 0 {
		meta = "NFL MATCHUPS"
	}
	if m.favoritesOnly && m.favorite != "" {
		meta = fmt.Sprintf("%s FAVORITES  ·  WEEK %d  ·  %d GAME(S)", favoriteLabel(m.favorite), m.week, count)
	} else if m.liveOnly {
		meta = fmt.Sprintf("LIVE NOW  ·  %d GAME(S)", count)
	}
	if m.loading && count > 0 {
		meta += "  ·  SYNCING"
	}
	if count > rows {
		meta += fmt.Sprintf("  ·  %d–%d SHOWN", start+1, end)
	}
	lines := []string{accentStyle.Render(meta), borderStyle.Render(strings.Repeat("─", inner))}
	if m.errorText != "" {
		lines = append(lines, liveStyle.Render("Error: "+m.errorText))
	} else if count == 0 {
		message := "No games available for this week."
		if m.favoritesOnly {
			message = "No " + favoriteLabel(m.favorite) + " game this week. Press f for all games."
		} else if m.liveOnly {
			message = "No games live right now. Press Esc for the menu."
		}
		if m.loading {
			message = "Loading games…"
		}
		lines = append(lines, dimStyle.Render(message))
	}
	for i := start; i < end; i++ {
		g := games[i]
		away, home := g.sides()
		selected := i == m.selected
		marker := " "
		if selected {
			marker = "›"
		} else if away.Team.Abbreviation == m.favorite || home.Team.Abbreviation == m.favorite {
			marker = "★"
		}
		nameWidth := 13
		if inner < 52 {
			nameWidth = 0
		}
		var matchup string
		if nameWidth == 0 {
			if g.Status.Type.State == "pre" {
				matchup = fmt.Sprintf("%s  %s  vs  %s", marker, away.Team.Abbreviation, home.Team.Abbreviation)
			} else {
				matchup = fmt.Sprintf("%s  %s %s  –  %s %s", marker, away.Team.Abbreviation, away.Score, home.Score, home.Team.Abbreviation)
			}
		} else {
			awayName := clipText(teamName(away), nameWidth)
			homeName := clipText(teamName(home), nameWidth)
			if g.Status.Type.State == "pre" {
				matchup = fmt.Sprintf("%s  %-13s  %-3s   vs   %-3s  %s", marker, awayName, away.Team.Abbreviation, home.Team.Abbreviation, homeName)
			} else {
				matchup = fmt.Sprintf("%s  %-13s  %-3s %3s  –  %3s %-3s  %s", marker, awayName, away.Team.Abbreviation, away.Score, home.Score, home.Team.Abbreviation, homeName)
			}
		}
		badge := strings.ToUpper(g.statusText())
		if g.Status.Type.State == "in" {
			badge = "● LIVE  " + badge
		}
		if g.Status.Type.State == "pre" {
			badge = "UPCOMING  ·  " + badge
		}
		venue := ""
		if len(g.Competitions) > 0 {
			venue = g.Competitions[0].Venue.FullName
		}
		if venue != "" {
			badge += "  ·  " + venue
		}
		matchup = clipText(matchup, inner)
		badge = clipText("   "+badge, inner)
		if selected {
			lines = append(lines, selectedStyle.Render(padLine(matchup, inner)), selectedStyle.Render(padLine(badge, inner)))
		} else {
			lines = append(lines, textStyle.Render(padLine(matchup, inner)))
			if g.Status.Type.State == "in" {
				lines = append(lines, liveStyle.Render(padLine(badge, inner)))
			} else {
				lines = append(lines, dimStyle.Render(padLine(badge, inner)))
			}
		}
	}
	return framedLines(lines, inner)
}

func (m model) renderDetail(inner int) string {
	g := m.currentDetailGame()
	if g == nil {
		return centeredFrame([]string{liveStyle.Render("Game is no longer in this week. Press Esc.")}, inner, m.width)
	}
	away, home := g.sides()
	meta := "GAME CENTER"
	if !m.detail.CachedAt.IsZero() {
		meta += "  ·  CACHED"
	}
	lines := []string{accentStyle.Render(meta), borderStyle.Render(strings.Repeat("─", inner))}
	hero := fmt.Sprintf("%s  %s", away.Team.Abbreviation, teamName(away))
	if g.Status.Type.State == "pre" {
		hero += "     vs     "
	} else {
		hero += fmt.Sprintf("   %s  –  %s   ", away.Score, home.Score)
	}
	hero += fmt.Sprintf("%s  %s", home.Team.Abbreviation, teamName(home))
	lines = append(lines, textStyle.Bold(true).Render(centerLine(clipText(hero, inner), inner)))
	status := strings.ToUpper(g.statusText())
	if g.Status.Type.State == "in" {
		status = "● LIVE  " + status
	}
	if len(g.Competitions) > 0 && g.Competitions[0].Venue.FullName != "" {
		status += "  ·  " + g.Competitions[0].Venue.FullName
	}
	status = clipText(status, inner)
	if g.Status.Type.State == "in" {
		lines = append(lines, centerLine(liveStyle.Render(status), inner))
	} else {
		lines = append(lines, centerLine(dimStyle.Render(status), inner))
	}
	for _, context := range m.liveContextLines() {
		lines = append(lines, centerLine(sectionStyle.Render(clipText(context, inner)), inner))
	}
	tabs := []string{"1 TEAM", "2 PLAYERS", "3 PLAYS", "4 CENTER"}
	for i := range tabs {
		if i == m.detailTab {
			tabs[i] = "[ " + tabs[i] + " ]"
		}
	}
	lines = append(lines, centerLine(sectionStyle.Render(clipText(strings.Join(tabs, "     "), inner)), inner))
	lines = append(lines, borderStyle.Render(strings.Repeat("─", inner)))
	stats := m.statLines()
	start := min(m.detailScroll, m.maxDetailScroll())
	end := min(len(stats), start+m.detailPageSize(len(stats)))
	for _, line := range stats[start:end] {
		line = clipStyled(line, inner)
		if m.detailTab == 0 {
			line = centerLine(line, inner)
		}
		lines = append(lines, line)
	}
	if m.errorText != "" {
		lines = append(lines, liveStyle.Render(clipText("Error: "+m.errorText, inner)))
	}
	return centeredFrame(lines, inner, m.width)
}

func (m model) statLines() []string {
	if m.detailTab == 3 {
		return m.gameCenterLines()
	}
	if m.detailTab == 2 {
		return m.playLines()
	}
	if len(m.detail.Boxscore.Teams) == 0 {
		return []string{dimStyle.Render("Stats loading or not yet available.")}
	}
	g := m.currentDetailGame()
	if g == nil {
		return []string{dimStyle.Render("Game unavailable.")}
	}
	away, home := g.sides()
	if m.detailTab == 0 {
		teamStats := map[string]map[string]string{}
		for _, team := range m.detail.Boxscore.Teams {
			values := map[string]string{}
			for _, stat := range team.Statistics {
				values[stat.Name] = stat.DisplayValue
			}
			teamStats[team.Team.Abbreviation] = values
		}
		lines := []string{sectionStyle.Render(fmt.Sprintf("%8s   %-20s %8s", away.Team.Abbreviation, "", home.Team.Abbreviation))}
		for _, stat := range []struct{ name, label string }{
			{"firstDowns", "First downs"}, {"totalYards", "Total yards"}, {"netPassingYards", "Passing yards"},
			{"rushingYards", "Rushing yards"}, {"thirdDownEff", "Third down"}, {"redZoneAttempts", "Red zone"},
			{"totalPenaltiesYards", "Penalties / yards"}, {"turnovers", "Turnovers"},
		} {
			av, hv := teamStats[away.Team.Abbreviation][stat.name], teamStats[home.Team.Abbreviation][stat.name]
			if av == "" && hv == "" {
				continue
			}
			lines = append(lines, textStyle.Render(fmt.Sprintf("%8s   %-20s %8s", av, stat.label, hv)))
		}
		if len(lines) == 1 {
			return []string{dimStyle.Render("Team stats will appear after kickoff.")}
		}
		return lines
	}
	lines := []string{}
	for _, category := range []string{"passing", "rushing", "receiving"} {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, sectionStyle.Render(strings.ToUpper(category)))
		for _, team := range []string{away.Team.Abbreviation, home.Team.Abbreviation} {
			for _, players := range m.detail.Boxscore.Players {
				if players.Team.Abbreviation != team {
					continue
				}
				for _, group := range players.Statistics {
					if group.Name != category {
						continue
					}
					for _, athlete := range group.Athletes[:min(2, len(group.Athletes))] {
						parts := []string{}
						for i, label := range group.Labels {
							if i >= len(athlete.Stats) {
								break
							}
							if label == "YDS" || label == "TD" || label == "INT" || label == "C/ATT" || label == "REC" || label == "CAR" {
								parts = append(parts, label+" "+athlete.Stats[i])
							}
						}
						lines = append(lines, textStyle.Render(fmt.Sprintf("  %-3s %-22s %s", team, athlete.Athlete.DisplayName, strings.Join(parts, "  ·  "))))
					}
				}
			}
		}
	}
	if len(lines) == 5 {
		return []string{dimStyle.Render("Player stats will appear after kickoff.")}
	}
	return lines
}

func (m model) playLines() []string {
	if len(m.detail.Plays) == 0 {
		return []string{dimStyle.Render("Play-by-play is not available yet.")}
	}
	width := m.width
	if width <= 0 {
		width = 80
	}
	wrapWidth := max(12, min(68, width-12))
	lines := make([]string, 0, len(m.detail.Plays)*3)
	for _, p := range m.detail.Plays {
		label := fmt.Sprintf("Q%d  %-5s  %-3s  %s", p.Period.Number, p.Clock.DisplayValue, p.Team, p.Type.Text)
		if p.ScoringPlay {
			label += fmt.Sprintf("  ·  SCORE %d–%d", p.AwayScore, p.HomeScore)
		}
		if p.IsTurnover {
			label += "  ·  TURNOVER"
		}
		if p.ScoringPlay {
			lines = append(lines, accentStyle.Render(label))
		} else if p.IsTurnover {
			lines = append(lines, liveStyle.Render(label))
		} else {
			lines = append(lines, sectionStyle.Render(label))
		}
		for _, text := range wrapWords(p.Text, wrapWidth) {
			lines = append(lines, dimStyle.Render("  "+text))
		}
		lines = append(lines, "")
	}
	return lines
}

func wrapWords(s string, width int) []string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	line := ""
	for _, word := range words {
		if line != "" && len([]rune(line))+1+len([]rune(word)) > width {
			lines = append(lines, line)
			line = ""
		}
		if line != "" {
			line += " "
		}
		line += word
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}

func (m model) conferenceName() string {
	if m.standingsConference < len(m.standings.Children) {
		return m.standings.Children[m.standingsConference].Abbreviation
	}
	if m.standingsConference == 1 {
		return "NFC"
	}
	return "AFC"
}

func (m model) standingEntries() []standingEntry {
	if m.standingsSeason != m.season || m.standingsConference >= len(m.standings.Children) {
		return nil
	}
	return m.standings.Children[m.standingsConference].Standings.Entries
}

func (m model) standingsPageSize() int {
	if m.height > 0 && m.height < 16 {
		return max(1, m.height-4)
	}
	if m.height > 0 {
		return max(1, m.height-9)
	}
	return len(m.standingEntries())
}

func (m model) maxStandingsScroll() int {
	return max(0, len(m.standingEntries())-m.standingsPageSize())
}

func (m model) standingsRange() string {
	count := len(m.standingEntries())
	if count == 0 {
		return "LOADING"
	}
	if m.maxStandingsScroll() == 0 {
		return fmt.Sprintf("ALL %d TEAMS", count)
	}
	start := min(m.standingsScroll, m.maxStandingsScroll())
	return fmt.Sprintf("%d–%d OF %d", start+1, min(count, start+m.standingsPageSize()), count)
}

func (m model) standingLines() []string {
	entries := m.standingEntries()
	if len(entries) == 0 {
		if m.standingsError != "" {
			return []string{liveStyle.Render("Error: " + m.standingsError)}
		}
		return []string{dimStyle.Render("Loading standings…")}
	}
	lines := make([]string, 0, len(entries))
	for i, e := range entries {
		wins, losses, ties := e.stat("wins"), e.stat("losses"), e.stat("ties")
		star := " "
		if e.Team.Abbreviation == m.favorite {
			star = "★"
		}
		record := wins + "-" + losses + "-" + ties
		line := fmt.Sprintf("%s %2d  %-3s %-24s  %7s  %5s  %4s", star, i+1, e.Team.Abbreviation, clipText(e.Team.DisplayName, 24), record, e.stat("winPercent"), e.stat("streak"))
		if e.Team.Abbreviation == m.favorite {
			lines = append(lines, selectedStyle.Render(line))
		} else {
			lines = append(lines, textStyle.Render(line))
		}
	}
	return lines
}

func (m model) renderStandings(inner int) string {
	entries := m.standingEntries()
	meta := m.conferenceName() + " STANDINGS  ·  " + fmt.Sprint(m.season) + "  ·  " + m.standingsRange()
	if !m.standings.CachedAt.IsZero() {
		meta += "  ·  CACHED"
	}
	if m.standingsError != "" && len(entries) > 0 {
		meta += "  ·  UPDATE FAILED"
	}
	lines := []string{accentStyle.Render(meta)}
	if inner >= 60 {
		lines = append(lines, sectionStyle.Render("  #  TEAM                               W-L-T    PCT  STRK"))
	} else {
		lines = append(lines, sectionStyle.Render("  #  TEAM                 W-L-T"))
	}
	if len(entries) == 0 {
		lines = append(lines, m.standingLines()...)
	} else {
		start := min(m.standingsScroll, m.maxStandingsScroll())
		end := min(len(entries), start+m.standingsPageSize())
		for _, row := range m.standingLines()[start:end] {
			lines = append(lines, clipStyled(row, inner))
		}
	}
	return centeredFrame(lines, inner, m.width)
}

func (m model) detailPageSize(lineCount int) int {
	if m.height > 0 && m.height < 16 {
		return max(1, m.height-4-len(m.liveContextLines()))
	}
	if m.height > 0 {
		return max(1, m.height-13-len(m.liveContextLines()))
	}
	return lineCount
}

func (m model) maxDetailScroll() int {
	lines := m.statLines()
	return max(0, len(lines)-m.detailPageSize(len(lines)))
}

func teamName(c competitor) string {
	if c.Team.Name != "" {
		return c.Team.Name
	}
	if c.Team.DisplayName != "" {
		return c.Team.DisplayName
	}
	return c.Team.Abbreviation
}

func clipText(s string, width int) string {
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	if width < 2 {
		return string(runes[:max(0, width)])
	}
	return string(runes[:width-1]) + "…"
}

func clipStyled(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(s)
}

func padLine(s string, width int) string {
	s = clipStyled(s, width)
	return s + strings.Repeat(" ", max(0, width-lipgloss.Width(s)))
}

func centerLine(s string, width int) string {
	return strings.Repeat(" ", max(0, (width-lipgloss.Width(s))/2)) + s
}

func centeredFrame(lines []string, inner, terminalWidth int) string {
	return centerBlock(framedLines(lines, inner), terminalWidth)
}

func framedLines(lines []string, inner int) string {
	var b strings.Builder
	b.WriteString(borderStyle.Render("╭"+strings.Repeat("─", inner+2)+"╮") + "\n")
	for _, line := range lines {
		b.WriteString(borderStyle.Render("│") + " " + padLine(line, inner) + " " + borderStyle.Render("│") + "\n")
	}
	b.WriteString(borderStyle.Render("╰" + strings.Repeat("─", inner+2) + "╯"))
	return b.String()
}

func centerBlock(block string, terminalWidth int) string {
	if terminalWidth <= 0 {
		terminalWidth = 80
	}
	lines := strings.Split(block, "\n")
	blockWidth := 0
	for _, line := range lines {
		blockWidth = max(blockWidth, lipgloss.Width(line))
	}
	prefix := strings.Repeat(" ", max(0, (terminalWidth-blockWidth)/2))
	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(prefix + line)
	}
	return b.String()
}
