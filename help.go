package main

import "strings"

func (m model) renderHelp(width int) string {
	height := m.height
	if height <= 0 {
		height = 24
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
		textStyle.Render("1–4 in the game center: stats, players, plays, center"),
		"",
		sectionStyle.Render("ON THE DASHBOARD"),
		textStyle.Render("g  favorite game    p  favorite season    t  team schedules"),
		textStyle.Render("s  standings        f  favorite filter    c  settings"),
		textStyle.Render("P  playoffs         L  league leaders"),
		"",
		sectionStyle.Render("ON ANY SCREEN"),
		textStyle.Render("r  refresh          Esc  back             q  quit"),
		"",
		dimStyle.Render("Press ? or Esc to return"),
	}
	// Blank lines go first, then the long guide gives way to the short one.
	var tight []string
	for _, line := range lines {
		if line != "" {
			tight = append(tight, line)
		}
	}
	if width < 30 || len(tight)+2 > height {
		short := []string{
			accentStyle.Render("◆ TOUCHDOWN HELP"),
			textStyle.Render("SCORE  AWAY 7 – 3 HOME · 1ST = first quarter"),
			textStyle.Render("↑/↓ or j/k  move · wheel/PgUp/PgDn scroll"),
			textStyle.Render("Enter  open game or selected season matchup"),
			textStyle.Render("Dashboard: g game · p season · t teams"),
			textStyle.Render("Dashboard: s standings · f filter · c settings"),
			textStyle.Render("Dashboard: P playoffs · L league leaders"),
			textStyle.Render("←/→  week or conference · r  refresh"),
			dimStyle.Render("Esc or ?  close help · q  quit"),
		}
		short = short[:min(len(short), height)]
		for i := range short {
			short[i] = clipStyled(short[i], width)
		}
		return strings.Join(short, "\n")
	}
	if len(lines)+2 > height {
		lines = tight
	}
	for i := range lines {
		lines[i] = clipStyled(lines[i], inner)
	}
	return centerBlock(framedLines(lines, inner), width)
}
