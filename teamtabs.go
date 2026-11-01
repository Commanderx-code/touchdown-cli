package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// The season page has three tabs: schedule (the original page), roster and
// injury report. Each keeps its own selection and scroll.

const (
	tabSchedule = iota
	tabRoster
	tabInjuries
)

var teamTabNames = []string{"1 SCHEDULE", "2 ROSTER", "3 INJURIES"}

func (m model) tabLine() string {
	tabs := append([]string(nil), teamTabNames...)
	if m.teamTab >= 0 && m.teamTab < len(tabs) {
		tabs[m.teamTab] = "[ " + tabs[m.teamTab] + " ]"
	}
	return sectionStyle.Render(strings.Join(tabs, "     "))
}

// resetTeamTabs returns the season page to the schedule tab with every
// selection at the top; every path that opens a season page calls it.
func (m model) resetTeamTabs() model {
	m.teamTab = tabSchedule
	m.rosterSelected, m.rosterScroll = 0, 0
	m.injurySelected, m.injuryScroll = 0, 0
	return m
}

// setTeamTab switches tabs and starts loading the tab's data when needed.
func (m model) setTeamTab(tab int) (model, tea.Cmd) {
	m.teamTab = tab
	team := m.activeTeamPage().Team
	switch tab {
	case tabRoster:
		return m.requestRoster(team, false)
	case tabInjuries:
		return m.requestInjuries(team, false)
	}
	return m, nil
}

// refreshTeamTab refetches the active tab's data.
func (m model) refreshTeamTab() (model, tea.Cmd) {
	team := m.activeTeamPage().Team
	switch m.teamTab {
	case tabRoster:
		return m.requestRoster(team, true)
	case tabInjuries:
		return m.requestInjuries(team, true)
	}
	return m, nil
}

func (m model) teamLines() []string {
	switch m.teamTab {
	case tabRoster:
		lines, _ := m.rosterRender()
		return lines
	case tabInjuries:
		lines, _ := m.injuryRender()
		return lines
	}
	return m.scheduleLines()
}

// teamTabScroll is the active tab's scroll offset.
func (m model) teamTabScroll() int {
	switch m.teamTab {
	case tabRoster:
		return m.rosterScroll
	case tabInjuries:
		return m.injuryScroll
	}
	return m.teamScroll
}

func (m model) teamRowWidth() int {
	width := m.width
	if width <= 0 {
		width = 100
	}
	return max(30, min(96, width-8))
}

// rosterRows is every selectable player in render order.
func (m model) rosterRows() []rosterPlayer {
	entry, _ := m.rosterFor(m.activeTeamPage().Team)
	var rows []rosterPlayer
	for _, group := range entry.Data.Groups {
		rows = append(rows, group.Players...)
	}
	return rows
}

// rosterRender returns the roster tab's lines and, for each selectable row,
// the index of its line.
func (m model) rosterRender() ([]string, []int) {
	team := m.activeTeamPage().Team
	entry, ok := m.rosterFor(team)
	switch {
	case !ok || entry.Loading && entry.Data.Team == "":
		return []string{dimStyle.Render("Loading roster…")}, nil
	case entry.Error != "" && entry.Data.Team == "":
		return []string{liveStyle.Render("Roster: " + entry.Error)}, nil
	}
	var lines []string
	var rowLines []int
	width := m.teamRowWidth()
	index := 0
	for _, group := range entry.Data.Groups {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, sectionStyle.Render(group.Title))
		for _, p := range group.Players {
			row := rosterRow(p, width)
			if index == m.rosterSelected {
				row = selectedStyle.Render(padLine(row, width))
			} else {
				row = textStyle.Render(row)
			}
			rowLines = append(rowLines, len(lines))
			lines = append(lines, row)
			index++
		}
	}
	if len(rowLines) == 0 {
		lines = append(lines, dimStyle.Render("No players listed."))
	}
	return lines, rowLines
}

func rosterRow(p rosterPlayer, width int) string {
	age := ""
	if p.Age > 0 {
		age = fmt.Sprint(p.Age)
	}
	row := fmt.Sprintf("#%-3s %-4s %-24s %2s  %-6s %-8s %-7s %s", p.Jersey, p.Position, clipText(p.Name, 24), age, p.Height, p.Weight, experienceLabel(p), p.College)
	if width < 80 {
		row = fmt.Sprintf("#%-3s %-4s %-24s %2s  %-7s", p.Jersey, p.Position, clipText(p.Name, 24), age, experienceLabel(p))
	}
	row = strings.TrimRight(row, " ")
	if badge := rosterBadge(p); badge != "" {
		row += "  " + badge
	}
	return clipText(row, width)
}

func rosterBadge(p rosterPlayer) string {
	switch p.Injury {
	case "":
		return ""
	case "Out":
		return "OUT"
	case "Doubtful":
		return "D"
	case "Questionable":
		return "Q"
	case "Injured Reserve":
		return "IR"
	}
	return strings.ToUpper(p.Injury)
}

func (m model) rosterSelectedLine() int {
	_, rowLines := m.rosterRender()
	if len(rowLines) == 0 {
		return 0
	}
	return rowLines[max(0, min(m.rosterSelected, len(rowLines)-1))]
}

// injuryRows is every selectable injury row in render order.
func (m model) injuryRows() []injury {
	entry, _ := m.injuriesFor(m.activeTeamPage().Team)
	return entry.Report.Entries
}

func (m model) injuryRender() ([]string, []int) {
	team := m.activeTeamPage().Team
	entry, ok := m.injuriesFor(team)
	switch {
	case !ok || entry.Loading && entry.Report.Team == "":
		return []string{dimStyle.Render("Loading injury report…")}, nil
	case entry.Error != "" && entry.Report.Team == "":
		return []string{liveStyle.Render("Injuries: " + entry.Error)}, nil
	}
	header := "INJURY REPORT"
	if summary := entry.Report.summary(); summary != "" {
		header += "  ·  " + summary
	}
	lines := []string{sectionStyle.Render(header)}
	if len(entry.Report.Entries) == 0 {
		return append(lines, dimStyle.Render("No injuries reported.")), nil
	}
	width := m.teamRowWidth()
	var rowLines []int
	for i, row := range entry.Report.Entries {
		text := clipText(injuryRow(row), width)
		if i == m.injurySelected {
			text = selectedStyle.Render(padLine(text, width))
		} else {
			text = textStyle.Render(text)
		}
		rowLines = append(rowLines, len(lines))
		lines = append(lines, text)
		for _, part := range wrapWords(row.Comment, width-4) {
			lines = append(lines, dimStyle.Render("    "+part))
		}
	}
	return lines, rowLines
}

func injuryRow(i injury) string {
	row := fmt.Sprintf("%-5s %-4s %-24s %-15s %s", statusLabel(i), i.Position, clipText(i.Name, 24), clipText(i.BodyPart, 15), returnLabel(i.ReturnDate))
	return strings.TrimRight(row, " ")
}

func (m model) injurySelectedLine() int {
	_, rowLines := m.injuryRender()
	if len(rowLines) == 0 {
		return 0
	}
	return rowLines[max(0, min(m.injurySelected, len(rowLines)-1))]
}

// moveTabSelection moves the roster or injury selection by delta rows and
// keeps the selected line on screen.
func (m model) moveTabSelection(delta int) model {
	page := m.teamPageSize()
	switch m.teamTab {
	case tabRoster:
		rows := len(m.rosterRows())
		if rows == 0 {
			m.rosterScroll = max(0, min(m.rosterScroll+delta, m.maxTeamScroll()))
			return m
		}
		m.rosterSelected = max(0, min(m.rosterSelected+delta, rows-1))
		m.rosterScroll = keepVisible(m.rosterSelectedLine(), m.rosterScroll, page, m.maxTeamScroll())
		if m.rosterSelected == 0 {
			m.rosterScroll = 0
		}
	case tabInjuries:
		rows := len(m.injuryRows())
		if rows == 0 {
			m.injuryScroll = max(0, min(m.injuryScroll+delta, m.maxTeamScroll()))
			return m
		}
		m.injurySelected = max(0, min(m.injurySelected+delta, rows-1))
		m.injuryScroll = keepVisible(m.injurySelectedLine(), m.injuryScroll, page, m.maxTeamScroll())
		if m.injurySelected == 0 {
			m.injuryScroll = 0
		}
	}
	return m
}

func keepVisible(line, scroll, page, maxScroll int) int {
	if line < scroll {
		scroll = line
	}
	if line >= scroll+page {
		scroll = line - page + 1
	}
	return max(0, min(scroll, maxScroll))
}
