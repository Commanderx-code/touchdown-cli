package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// playerRef is what the player card knows before any stats arrive: the row
// it was opened from.
type playerRef struct {
	ID       string
	Name     string
	Position string
	Team     string
	TeamName string
	Jersey   string
	Height   string
	Weight   string
	College  string
	Injury   string // the roster row's status, shown until the injury report answers
	Age      int
	Years    int
	HasYears bool
}

func playerFromRoster(p rosterPlayer, team, teamName string) playerRef {
	return playerRef{
		ID: p.ID, Name: p.Name, Position: p.Position, Team: team, TeamName: teamName,
		Jersey: p.Jersey, Height: p.Height, Weight: p.Weight, College: p.College,
		Injury: p.Injury, Age: p.Age, Years: p.Years, HasYears: p.HasYears,
	}
}

// playerFromInjury builds a reference from an injury row, borrowing the bio
// from the team's roster when it is already loaded.
func (m model) playerFromInjury(i injury, team, teamName string) playerRef {
	ref := playerRef{ID: i.AthleteID, Name: i.Name, Position: i.Position, Team: team, TeamName: teamName, Jersey: "--"}
	if entry, ok := m.rosterFor(team); ok && i.AthleteID != "" {
		for _, group := range entry.Data.Groups {
			for _, p := range group.Players {
				if p.ID == i.AthleteID {
					return playerFromRoster(p, team, teamName)
				}
			}
		}
	}
	return ref
}

func (m model) openPlayer(ref playerRef) (model, tea.Cmd) {
	if ref.ID == "" {
		return m, nil
	}
	m.showPlayer = true
	m.player = ref
	m.playerScroll = 0
	m, stats := m.requestPlayer(ref.ID, false)
	m, injuries := m.requestInjuries(ref.Team, false)
	return m, tea.Batch(stats, injuries)
}

// openSelectedPlayer opens the card for the selected roster or injury row.
func (m model) openSelectedPlayer() (model, tea.Cmd) {
	page := m.activeTeamPage()
	teamName := plainText(page.Data.Team.DisplayName)
	if teamName == "" {
		teamName = page.Team
	}
	switch m.teamTab {
	case tabRoster:
		if rows := m.rosterRows(); len(rows) > 0 {
			return m.openPlayer(playerFromRoster(rows[m.rosterIndex()], page.Team, teamName))
		}
	case tabInjuries:
		if rows := m.injuryRows(); len(rows) > 0 {
			return m.openPlayer(m.playerFromInjury(rows[m.injuryIndex()], page.Team, teamName))
		}
	}
	return m, nil
}

func (m model) updatePlayerKey(key string) (model, tea.Cmd) {
	switch key {
	case "up", "k":
		m.playerScroll = max(0, m.playerScroll-1)
	case "down", "j":
		m.playerScroll = min(m.maxPlayerScroll(), m.playerScroll+1)
	case "pgup":
		m.playerScroll = max(0, m.playerScroll-m.playerPageSize())
	case "pgdown":
		m.playerScroll = min(m.maxPlayerScroll(), m.playerScroll+m.playerPageSize())
	case "r":
		return m.requestPlayer(m.player.ID, true)
	case "esc", "backspace":
		m.showPlayer = false
	}
	return m, nil
}

func (m model) playerPageSize() int {
	height := m.height
	if height <= 0 {
		height = 24
	}
	return max(1, height-7)
}

func (m model) maxPlayerScroll() int {
	return max(0, len(m.playerLines(m.playerInner()))-m.playerPageSize())
}

func (m model) playerInner() int {
	width := m.width
	if width <= 0 {
		width = 80
	}
	return max(24, min(74, width-6))
}

func (m model) playerLines(inner int) []string {
	p := m.player
	lines := []string{textStyle.Bold(true).Render(clipText(p.Name, inner))}
	lines = append(lines, sectionStyle.Render(clipText(fmt.Sprintf("#%s  %s  ·  %s", p.Jersey, p.Position, p.TeamName), inner)))
	var bio []string
	if p.Age > 0 {
		bio = append(bio, fmt.Sprintf("%d yrs", p.Age))
	}
	for _, part := range []string{p.Height, p.Weight, seasonLabelFor(rosterPlayer{Years: p.Years, HasYears: p.HasYears}), p.College} {
		if part != "" {
			bio = append(bio, part)
		}
	}
	if len(bio) > 0 {
		lines = append(lines, dimStyle.Render(clipText(strings.Join(bio, "  ·  "), inner)))
	}
	report, loaded := m.injuriesFor(p.Team)
	if i, found := report.Report.find(p.ID); loaded && found {
		parts := []string{statusLabel(i)}
		for _, part := range []string{i.BodyPart, returnLabel(i.ReturnDate)} {
			if part != "" {
				parts = append(parts, part)
			}
		}
		lines = append(lines, liveStyle.Render(clipText(strings.Join(parts, "  ·  "), inner)))
		for _, part := range wrapWords(i.Comment, inner-2) {
			lines = append(lines, dimStyle.Render("  "+part))
		}
	} else if badge := rosterBadge(rosterPlayer{Injury: p.Injury}); badge != "" {
		lines = append(lines, liveStyle.Render(badge))
	}
	lines = append(lines, "", sectionStyle.Render("SEASON STATS"))
	entry, ok := m.playerFor(p.ID)
	if notice := staleLine(entry.Stats.CachedAt, ""); notice != "" {
		lines = append(lines, liveStyle.Render(clipText(notice, inner)))
	}
	switch {
	case !ok || entry.Loading && entry.Stats.ID == "":
		lines = append(lines, dimStyle.Render("Loading stats…"))
	case entry.Error != "":
		lines = append(lines, liveStyle.Render("Stats: "+entry.Error))
	case len(entry.Stats.Sections) == 0:
		lines = append(lines, dimStyle.Render("No season stats yet."))
	}
	if ok && entry.Error == "" && !(entry.Loading && entry.Stats.ID == "") {
		for n, section := range entry.Stats.Sections {
			if n > 0 {
				lines = append(lines, "")
			}
			title := section.Title
			if section.Split != "" {
				title += "  ·  " + section.Split
			}
			lines = append(lines, accentStyle.Render(clipText(title, inner)))
			labels, values := alignColumns(section.Labels, section.Values)
			lines = append(lines, dimStyle.Render(clipText(labels, inner)), textStyle.Render(clipText(values, inner)))
		}
	}
	return lines
}

// alignColumns right-aligns each value under its label.
func alignColumns(labels, values []string) (string, string) {
	var top, bottom []string
	for i, label := range labels {
		value := ""
		if i < len(values) {
			value = values[i]
		}
		width := max(len(label), len(value))
		top = append(top, fmt.Sprintf("%*s", width, label))
		bottom = append(bottom, fmt.Sprintf("%*s", width, value))
	}
	return strings.Join(top, "  "), strings.Join(bottom, "  ")
}

func (m model) renderPlayer(width int) string {
	height := m.height
	if height <= 0 {
		height = 24
	}
	inner := m.playerInner()
	rows := m.playerLines(inner)
	start := min(m.playerScroll, m.maxPlayerScroll())
	end := min(len(rows), start+m.playerPageSize())
	footer := dimStyle.Render("↑/↓ scroll  ·  Esc back  ·  r refresh  ·  q quit")
	if m.compact || height < 14 || width < 30 {
		lines := append([]string{accentStyle.Render("◆ PLAYER")}, rows[start:end]...)
		lines = append(lines, footer)
		for i := range lines {
			lines[i] = clipStyled(lines[i], width)
		}
		return strings.Join(lines, "\n")
	}
	lines := []string{accentStyle.Render("◆  PLAYER"), borderStyle.Render(strings.Repeat("─", inner))}
	lines = append(lines, rows[start:end]...)
	lines = append(lines, borderStyle.Render(strings.Repeat("─", inner)), footer)
	for i := range lines {
		lines[i] = clipStyled(lines[i], inner)
	}
	return centerBlock(framedLines(lines, inner), width)
}
