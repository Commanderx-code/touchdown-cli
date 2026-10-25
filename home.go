package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	logoStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F5F4FF"))
	shadowStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#667489"))
	subtitleStyle = lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color("#9FA9B6"))
)

var touchdownGlyphs = map[rune][]string{
	'T': {"███████", "   █   ", "   █   ", "   █   ", "   █   ", "   █   ", "   █   "},
	'O': {" █████ ", "██   ██", "██   ██", "██   ██", "██   ██", "██   ██", " █████ "},
	'U': {"██   ██", "██   ██", "██   ██", "██   ██", "██   ██", "██   ██", " █████ "},
	'C': {" ██████", "██     ", "██     ", "██     ", "██     ", "██     ", " ██████"},
	'H': {"██   ██", "██   ██", "██   ██", "███████", "██   ██", "██   ██", "██   ██"},
	'D': {"██████ ", "██   ██", "██   ██", "██   ██", "██   ██", "██   ██", "██████ "},
	'W': {"██   ██", "██   ██", "██   ██", "██ █ ██", "██ █ ██", "██ █ ██", " █████ "},
	'N': {"██   ██", "███  ██", "████ ██", "██ ████", "██  ███", "██   ██", "██   ██"},
}

func touchdownLogo(spacing int) string {
	const word = "TOUCHDOWN"
	const glyphWidth = 7
	const glyphHeight = 7
	width := len(word) * (glyphWidth + spacing)
	grid := make([][]rune, glyphHeight+1)
	for y := range grid {
		grid[y] = make([]rune, width)
		for x := range grid[y] {
			grid[y][x] = ' '
		}
	}
	for i, letter := range word {
		for y, row := range touchdownGlyphs[letter] {
			for x, pixel := range []rune(row) {
				if pixel == '█' {
					grid[y+1][i*(glyphWidth+spacing)+x+1] = '░'
				}
			}
		}
	}
	for i, letter := range word {
		for y, row := range touchdownGlyphs[letter] {
			for x, pixel := range []rune(row) {
				if pixel == '█' {
					grid[y][i*(glyphWidth+spacing)+x] = '█'
				}
			}
		}
	}
	var lines []string
	for _, row := range grid {
		var b strings.Builder
		for _, pixel := range row {
			switch pixel {
			case '█':
				b.WriteString(logoStyle.Render("█"))
			case '░':
				b.WriteString(shadowStyle.Render("░"))
			default:
				b.WriteRune(' ')
			}
		}
		lines = append(lines, b.String())
	}
	return strings.Join(lines, "\n")
}

func (m model) homeOptions() []string {
	live := 0
	for _, g := range m.board.Events {
		if g.Status.Type.State == "in" {
			live++
		}
	}
	favoriteCount := 0
	if _, ok := m.favoriteMatch(); ok {
		favoriteCount = 1
	}
	favorite := favoriteLabel(m.favorite)
	return []string{
		fmt.Sprintf("ALL GAMES (%d)", len(m.board.Events)),
		fmt.Sprintf("LIVE NOW (%d)", live),
		fmt.Sprintf("%s WATCH (%d)", favorite, favoriteCount),
		fmt.Sprintf("%s SEASON", favorite),
		"NFC STANDINGS",
		"AFC STANDINGS",
	}
}

func (m model) renderHome(width int) string {
	height := m.height
	if height <= 0 {
		height = 24
	}
	if m.compact {
		height = min(height, 15)
	}
	if height < 11 {
		options := m.homeOptions()
		return accentStyle.Render("◆ TOUCHDOWN") + "\n" + selectedStyle.Render("› "+options[m.homeSelection]) + "\n" + dimStyle.Render("↑/↓ · Enter · g game · q")
	}
	inner := min(62, width-8)
	if width > 90 {
		inner = min(96, width*65/100)
	}
	if inner < 20 {
		inner = max(12, width-4)
	}
	var lines []string
	if height >= 16 {
		lines = append(lines, sectionStyle.Render("SELECT A VIEW"), borderStyle.Render(strings.Repeat("─", inner)))
	}
	icons := []string{"◉", "●", "★", "▦", "◌", "◌"}
	for i, option := range m.homeOptions() {
		marker := " "
		if i == m.homeSelection {
			marker = "›"
		}
		row := clipText(fmt.Sprintf("%s  %s  %s", marker, icons[i], option), inner)
		if i == m.homeSelection {
			lines = append(lines, selectedStyle.Render(padLine(row, inner)))
		} else {
			lines = append(lines, dimStyle.Render(padLine(row, inner)))
		}
	}
	menu := centerBlock(framedLines(lines, inner), width)
	footer := centerLine(dimStyle.Render(clipText("↑/↓ · Enter · g game · p season · c settings · ? help · q", width)), width)
	if m.pendingFavoriteGame {
		footer = centerLine(accentStyle.Render("Opening favorite game when scores load…"), width)
	}
	if m.errorText != "" {
		footer = centerLine(liveStyle.Render(clipText("Scores unavailable: "+m.errorText, width)), width)
	}
	brand := centerLine(accentStyle.Render("◆  TOUCHDOWN"), width)
	if height >= 23 && width >= 76 {
		spacing := 1
		if width >= 90 {
			spacing = 2
		}
		if width >= 115 {
			spacing = 3
		}
		brand = centerBlock(touchdownLogo(spacing), width)
	}
	subtitle := centerLine(subtitleStyle.Render("Terminal Live Football"), width)
	status := m.dashboardSummary(width)
	var screen string
	if height >= 23 {
		screen = brand + "\n" + subtitle + "\n\n" + status + "\n\n" + menu + "\n\n" + footer
	} else if height >= 16 {
		screen = brand + "\n" + subtitle + "\n" + status + "\n" + menu + "\n" + footer
	} else {
		screen = brand + "\n" + subtitle + "\n" + menu + "\n" + footer
	}
	free := height - strings.Count(screen, "\n") - 1
	if free > 0 {
		screen = strings.Repeat("\n", min(4, free/2)) + screen
	}
	return screen
}
