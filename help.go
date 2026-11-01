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
		textStyle.Render("GB ball  3rd & 7 at CAR 22  ·  possession and down for live games"),
		textStyle.Render("RED ZONE  offense inside the 20  ·  TO ●●○  timeouts remaining"),
		"",
		sectionStyle.Render("NAVIGATION"),
		textStyle.Render("↑/↓ or j/k     choose rows; wheel or PgUp/PgDn scroll"),
		textStyle.Render("Enter           open a game or selected season matchup"),
		textStyle.Render("←/→ or h/l      change week, conference, or settings theme"),
		textStyle.Render("Home menu       Team schedules picks any team; Season lists every week"),
		textStyle.Render("1/2/3 on a season page: schedule, roster, injuries; Enter opens a player"),
		"",
		sectionStyle.Render("QUICK KEYS"),
		textStyle.Render("g  favorite game    p  favorite season    t  team schedules"),
		textStyle.Render("s  standings        f  favorite filter    c  settings"),
		textStyle.Render("r  refresh          1–4  game tabs        Esc  back    q  quit"),
		"",
		dimStyle.Render("Press ? or Esc to return"),
	}
	for i := range lines {
		lines[i] = clipStyled(lines[i], inner)
	}
	return centerBlock(framedLines(lines, inner), width)
}
