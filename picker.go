package main

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
)

// openPicker shows the team picker with the favorite preselected.
func (m model) openPicker() model {
	origin := "dashboard"
	if m.showHome {
		origin = "home"
	}
	m = m.consumeOverview().push(backTarget{Screen: origin})
	m.showPicker = true
	m.showHome = false
	m.pickerBuffer = ""
	m.pickerIndex = 0
	for i, team := range nflTeams {
		if team.abbreviation == m.favorite {
			m.pickerIndex = i
			break
		}
	}
	m.pickerScroll = m.pickerScrollFor(m.pickerIndex)
	return m
}

func (m model) pickerPageSize() int {
	height := m.height
	if height <= 0 {
		height = 24
	}
	return max(1, height-9)
}

func (m model) pickerScrollFor(index int) int {
	return max(0, min(index-m.pickerPageSize()/2, len(nflTeams)-m.pickerPageSize()))
}

// updatePickerKey moves the cursor, jumps by typed abbreviation prefix, opens
// the chosen team's season page on Enter, and leaves on Esc.
func (m model) updatePickerKey(key string, now time.Time) (model, tea.Cmd) {
	page := m.pickerPageSize()
	switch key {
	case "up":
		m.pickerIndex = max(0, m.pickerIndex-1)
	case "down":
		m.pickerIndex = min(len(nflTeams)-1, m.pickerIndex+1)
	case "pgup":
		m.pickerIndex = max(0, m.pickerIndex-page)
	case "pgdown":
		m.pickerIndex = min(len(nflTeams)-1, m.pickerIndex+page)
	case "home":
		m.pickerIndex = 0
	case "end":
		m.pickerIndex = len(nflTeams) - 1
	case "enter":
		m.showPicker = false
		return m.push(backTarget{Screen: "picker"}).openTeamSeason(nflTeams[m.pickerIndex].abbreviation)
	case "esc", "backspace":
		return m.back()
	default:
		if len(key) == 1 && unicode.IsLetter(rune(key[0])) {
			if now.Sub(m.pickerTyped) > time.Second {
				m.pickerBuffer = ""
			}
			m.pickerBuffer += strings.ToUpper(key)
			m.pickerTyped = now
			for i, team := range nflTeams {
				if strings.HasPrefix(team.abbreviation, m.pickerBuffer) {
					m.pickerIndex = i
					break
				}
			}
		} else {
			m.pickerBuffer = ""
		}
		m.pickerScroll = m.pickerScrollFor(m.pickerIndex)
		return m, nil
	}
	m.pickerBuffer = ""
	if m.pickerIndex < m.pickerScroll {
		m.pickerScroll = m.pickerIndex
	}
	if m.pickerIndex >= m.pickerScroll+page {
		m.pickerScroll = m.pickerIndex - page + 1
	}
	return m, nil
}

func (m model) renderPicker(width int) string {
	height := m.height
	if height <= 0 {
		height = 24
	}
	selected := nflTeams[m.pickerIndex]
	if height < 14 || width < 30 {
		lines := []string{
			accentStyle.Render("◆ TEAM SCHEDULES"),
			sectionStyle.Render(fmt.Sprintf("TEAM  %s %s (%d/32)", selected.city, selected.nickname, m.pickerIndex+1)),
			dimStyle.Render("↑/↓ or type · Enter open · Esc back"),
		}
		for i := range lines {
			lines[i] = clipStyled(lines[i], width)
		}
		return strings.Join(lines, "\n")
	}
	inner := min(68, width-4)
	start := min(m.pickerScroll, max(0, len(nflTeams)-m.pickerPageSize()))
	end := min(len(nflTeams), start+m.pickerPageSize())
	lines := []string{
		accentStyle.Render("◆  TEAM SCHEDULES"),
		dimStyle.Render(fmt.Sprintf("CHOOSE A TEAM  ·  %d–%d OF %d", start+1, end, len(nflTeams))),
		borderStyle.Render(strings.Repeat("─", inner)),
	}
	for i := start; i < end; i++ {
		team := nflTeams[i]
		marker := " "
		if i == m.pickerIndex {
			marker = "›"
		}
		star := ""
		if team.abbreviation == m.favorite {
			star = "  ★"
		}
		row := fmt.Sprintf("%s %-3s  %s %s%s", marker, team.abbreviation, team.city, team.nickname, star)
		if i == m.pickerIndex {
			lines = append(lines, selectedStyle.Render(padLine(clipText(row, inner), inner)))
		} else {
			lines = append(lines, textStyle.Render(clipText(row, inner)))
		}
	}
	lines = append(lines, borderStyle.Render(strings.Repeat("─", inner)), dimStyle.Render("↑/↓ move  ·  type letters to jump  ·  Enter open season  ·  Esc back  ·  q quit"))
	for i := range lines {
		lines[i] = clipStyled(lines[i], inner)
	}
	return centerBlock(framedLines(lines, inner), width)
}
