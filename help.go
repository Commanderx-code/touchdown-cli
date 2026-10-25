package main

import "strings"

func (m model) renderHelp(width int) string {
	height := m.height
	if height <= 0 {
		height = 24
	}
	if height < 20 || width < 30 {
		lines := []string{
			accentStyle.Render("◆ TOUCHDOWN HELP"),
			textStyle.Render("SCORE  AWAY 7 – 3 HOME · 1ST = first quarter"),
			textStyle.Render("↑/↓ or j/k  move · wheel/PgUp/PgDn scroll"),
			textStyle.Render("Enter  open game or selected season matchup"),
			textStyle.Render("g  favorite game · p  favorite season"),
			textStyle.Render("s  standings · f  favorite filter"),
			textStyle.Render("c  team/theme settings · 1–4  game tabs"),
			textStyle.Render("←/→  week or conference · r  refresh"),
			dimStyle.Render("Esc or ?  close help · q  quit"),
		}
		lines = lines[:min(len(lines), height)]
		for i := range lines {
			lines[i] = clipStyled(lines[i], width)
		}
		return strings.Join(lines, "\n")
	}
	inner := min(70, width-4)
	lines := []string{
		accentStyle.Render("◆  TOUCHDOWN GUIDE"),
		borderStyle.Render(strings.Repeat("─", inner)),
		sectionStyle.Render("READING SCORES"),
		textStyle.Render("AWAY 7 – 3 HOME  ·  scores belong to the teams beside them"),
		textStyle.Render("1ST/2ND/3RD/4TH = quarter  ·  venue appears below the score"),
		"",
		sectionStyle.Render("NAVIGATION"),
		textStyle.Render("↑/↓ or j/k     choose rows; wheel or PgUp/PgDn scroll"),
		textStyle.Render("Enter           open a game or selected season matchup"),
		textStyle.Render("←/→ or h/l      change week, conference, or settings theme"),
		"",
		sectionStyle.Render("QUICK KEYS"),
		textStyle.Render("g  favorite game    p  favorite season    c  settings"),
		textStyle.Render("s  standings        f  favorite filter    1–4  game tabs"),
		textStyle.Render("r  refresh          Esc  back             q  quit"),
		"",
		dimStyle.Render("Press ? or Esc to return"),
	}
	for i := range lines {
		lines[i] = clipStyled(lines[i], inner)
	}
	return centerBlock(framedLines(lines, inner), width)
}
