package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// The league screens share one scrolling page layout.

func (m model) leagueCompact() bool {
	return m.compact || m.height > 0 && m.height < 16 || m.width > 0 && m.width < 40
}

// leagueWidth is the width rows are built to.
func (m model) leagueWidth() int {
	width := m.width
	if width <= 0 {
		width = 100
	}
	if m.leagueCompact() {
		return max(12, width)
	}
	return panelInner(width)
}

// leaguePageSize is how many rows fit below the title block. The frame takes
// two lines, the title, tabs and rules four, and the footer one.
func (m model) leaguePageSize() int {
	height := m.height
	if height <= 0 {
		height = 24
	}
	if m.leagueCompact() {
		return max(1, height-3)
	}
	return max(1, height-7)
}

func (m model) renderLeaguePage(title, sub string, rows []string, scroll int, footer, short string, width int) string {
	page := m.leaguePageSize()
	start := max(0, min(scroll, len(rows)-page))
	visible := rows[start:min(len(rows), start+page)]
	inner := m.leagueWidth()
	if lipgloss.Width(footer) > inner {
		footer = short
	}
	head := accentStyle.Render(title)
	if m.leagueCompact() {
		if sub == "" {
			sub = dimStyle.Render(short)
		}
		lines := []string{clipStyled(head, width), clipStyled(sub, width)}
		lines = append(lines, visible...)
		lines = append(lines, dimStyle.Render(clipText(short, width)))
		for i := range lines {
			lines[i] = clipStyled(lines[i], width)
		}
		return strings.Join(lines, "\n")
	}
	rule := borderStyle.Render(strings.Repeat("─", inner))
	if sub == "" {
		sub = dimStyle.Render("TOP FIVE IN EACH CATEGORY")
	}
	lines := []string{head, sub, rule}
	lines = append(lines, visible...)
	lines = append(lines, rule, dimStyle.Render(footer))
	for i := range lines {
		lines[i] = clipStyled(lines[i], inner)
	}
	return centerBlock(framedLines(lines, inner), width)
}
