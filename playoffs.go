package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	playoffPicture = iota
	playoffBracket
)

// playoffRounds are the postseason weeks, in ESPN's numbering. Week 4 is the
// Pro Bowl.
var playoffRounds = []struct {
	week  int
	title string
}{{1, "WILD CARD"}, {2, "DIVISIONAL ROUND"}, {3, "CONFERENCE CHAMPIONSHIPS"}, {5, "SUPER BOWL"}}

// seededTeam is one team in a conference's playoff order.
type seededTeam struct {
	Seed   int
	Team   string
	Name   string
	Record string
	Clinch string
}

type playoffRound struct {
	Board   scoreboard
	Loaded  bool
	Loading bool
	Error   string
}

type playoffRoundMsg struct {
	season, week int
	board        scoreboard
	err          error
}

// conferenceSeeds is ESPN's seeding in order. Teams without a seed are left
// out, so a conference with none yet comes back empty.
func conferenceSeeds(group standingGroup) []seededTeam {
	var seeds []seededTeam
	for _, entry := range group.Standings.Entries {
		seed, err := strconv.Atoi(entry.stat("playoffSeed"))
		if err != nil || seed < 1 {
			continue
		}
		team := seededTeam{Seed: seed, Team: plainText(entry.Team.Abbreviation), Name: plainText(entry.Team.DisplayName), Record: plainText(entry.stat("overall"))}
		if clinch := entry.stat("clincher"); clinch != "-" {
			team.Clinch = plainText(clinch)
		}
		seeds = append(seeds, team)
	}
	sort.SliceStable(seeds, func(i, j int) bool { return seeds[i].Seed < seeds[j].Seed })
	return seeds
}

func (s seededTeam) row(width int) string {
	row := fmt.Sprintf("%2d  %-4s %-24s %-6s %s", s.Seed, s.Team, clipText(s.Name, 24), s.Record, s.Clinch)
	return clipText(strings.TrimRight(row, " "), width)
}

// playoffLines is the picture tab: the seven who are in, the next four, and
// the first-round games if the season ended today.
func (m model) playoffLines(width int) []string {
	entries := m.standingEntries()
	if m.standingsSeason != m.season {
		if m.standingsError != "" {
			return []string{liveStyle.Render(clipText("Standings: "+plainText(m.standingsError), width))}
		}
		return []string{dimStyle.Render("Loading standings…")}
	}
	var lines []string
	if notice := staleLine(m.standings.CachedAt, m.standingsError); notice != "" {
		lines = append(lines, liveStyle.Render(clipText(notice, width)))
	}
	var seeds []seededTeam
	if m.standingsConference < len(m.standings.Children) {
		seeds = conferenceSeeds(m.standings.Children[m.standingsConference])
	}
	if len(entries) == 0 || len(seeds) == 0 {
		return append(lines, dimStyle.Render("No playoff seeds yet."))
	}
	section := func(title string, teams []seededTeam) {
		if len(teams) == 0 {
			return
		}
		lines = append(lines, sectionStyle.Render(title))
		for _, team := range teams {
			if team.Team == m.favorite {
				lines = append(lines, accentStyle.Render(team.row(width)))
			} else {
				lines = append(lines, textStyle.Render(team.row(width)))
			}
		}
		lines = append(lines, "")
	}
	bySeed := func(from, to int) []seededTeam {
		var teams []seededTeam
		for _, team := range seeds {
			if team.Seed >= from && team.Seed <= to {
				teams = append(teams, team)
			}
		}
		return teams
	}
	section("DIVISION LEADERS", bySeed(1, 4))
	section("WILD CARD", bySeed(5, 7))
	hunt := bySeed(8, 11)
	for _, team := range seeds {
		if team.Seed > 11 && team.Team == m.favorite {
			hunt = append(hunt, team)
		}
	}
	section("IN THE HUNT", hunt)
	if games := firstRoundGames(seeds); len(games) > 0 && m.seasonType != 3 {
		lines = append(lines, sectionStyle.Render("IF THE SEASON ENDED TODAY"))
		for _, game := range games {
			lines = append(lines, textStyle.Render(clipText(game, width)))
		}
		lines = append(lines, "")
	}
	for _, team := range seeds {
		if team.Clinch != "" {
			lines = append(lines, dimStyle.Render(clipText("* top seed · z division · y wild card · x playoff spot · e eliminated", width)))
			break
		}
	}
	return lines
}

func firstRoundGames(seeds []seededTeam) []string {
	team := func(seed int) string {
		for _, s := range seeds {
			if s.Seed == seed {
				return s.Team
			}
		}
		return ""
	}
	if team(7) == "" {
		return nil
	}
	return []string{
		fmt.Sprintf("7 %s @ 2 %s", team(7), team(2)),
		fmt.Sprintf("6 %s @ 3 %s", team(6), team(3)),
		fmt.Sprintf("5 %s @ 4 %s", team(5), team(4)),
		fmt.Sprintf("1 %s has the bye", team(1)),
	}
}

// bracketLines is the bracket tab. ESPN lists every round all season with
// TBD teams, so nothing shows until a real team is in one.
func (m model) bracketLines(width int) []string {
	set, loading, loaded, failure := false, false, false, ""
	for _, round := range playoffRounds {
		entry := m.playoffRoundData[round.week]
		loading = loading || entry.Loading
		loaded = loaded || entry.Loaded
		if entry.Error != "" {
			failure = entry.Error
		}
		for _, g := range entry.Board.Events {
			set = set || gameIsSet(g)
		}
	}
	if !set {
		switch {
		case loading:
			return []string{dimStyle.Render("Loading bracket…")}
		case !loaded && failure != "":
			return []string{liveStyle.Render(clipText("Bracket: "+failure, width))}
		}
		return []string{dimStyle.Render("Matchups appear when the regular season ends.")}
	}
	var lines []string
	for _, round := range playoffRounds {
		entry := m.playoffRoundData[round.week]
		lines = append(lines, sectionStyle.Render(round.title))
		if notice := staleLine(entry.Board.CachedAt, entry.Error); notice != "" {
			lines = append(lines, liveStyle.Render(clipText(notice, width)))
		}
		shown := 0
		for _, g := range entry.Board.Events {
			if !gameIsSet(g) {
				continue
			}
			shown++
			row := clipText(bracketRow(g), width)
			away, home := g.sides()
			if away.Team.Abbreviation == m.favorite || home.Team.Abbreviation == m.favorite {
				lines = append(lines, accentStyle.Render(row))
			} else {
				lines = append(lines, textStyle.Render(row))
			}
		}
		if shown == 0 {
			lines = append(lines, dimStyle.Render("To be decided."))
		}
		lines = append(lines, "")
	}
	return lines
}

func gameIsSet(g game) bool {
	away, home := g.sides()
	for _, team := range []string{away.Team.Abbreviation, home.Team.Abbreviation} {
		if team != "" && team != "TBD" {
			return true
		}
	}
	return false
}

func bracketRow(g game) string {
	away, home := g.sides()
	match := fmt.Sprintf("%s @ %s", away.Team.Abbreviation, home.Team.Abbreviation)
	if g.Status.Type.State != "pre" {
		match = fmt.Sprintf("%s %s @ %s %s", away.Team.Abbreviation, away.Score, home.Team.Abbreviation, home.Score)
	}
	return plainText(fmt.Sprintf("%-20s %s", match, g.statusText()))
}

// openPlayoffs shows the playoffs on the favorite's conference, with the
// bracket first once the postseason has started.
func (m model) openPlayoffs() (model, tea.Cmd) {
	origin := "dashboard"
	if m.showHome {
		origin = "home"
	}
	m = m.consumeOverview().push(backTarget{Screen: origin})
	m.showHome = false
	m.showPlayoffs = true
	m.playoffScroll = 0
	m.playoffTab = playoffPicture
	if m.seasonType == 3 {
		m.playoffTab = playoffBracket
	}
	if team, ok := findTeam(m.favorite); ok {
		m.standingsConference = 0
		if team.conference == "NFC" {
			m.standingsConference = 1
		}
	}
	return m.playoffLoads(false)
}

// playoffLoads fetches what the open tab needs.
func (m model) playoffLoads(force bool) (model, tea.Cmd) {
	if m.season == 0 {
		return m, nil
	}
	var cmds []tea.Cmd
	if (force || m.standingsSeason != m.season) && !m.standingsLoading {
		m.standingsLoading = true
		cmds = append(cmds, m.loadStandings())
	}
	if m.playoffTab == playoffBracket {
		var rounds tea.Cmd
		m, rounds = m.requestPlayoffRounds(force)
		cmds = append(cmds, rounds)
	}
	return m, tea.Batch(cmds...)
}

func (m model) requestPlayoffRounds(force bool) (model, tea.Cmd) {
	if m.playoffSeason != m.season || m.playoffRoundData == nil {
		m.playoffSeason = m.season
		m.playoffRoundData = map[int]playoffRound{}
	}
	var cmds []tea.Cmd
	for _, round := range playoffRounds {
		entry := m.playoffRoundData[round.week]
		if entry.Loading || (entry.Loaded && !force) {
			continue
		}
		entry.Loading = true
		m.playoffRoundData[round.week] = entry
		cmds = append(cmds, m.loadPlayoffRound(round.week))
	}
	return m, tea.Batch(cmds...)
}

func (m model) loadPlayoffRound(week int) tea.Cmd {
	season := m.season
	return func() tea.Msg {
		board, err := m.api.scoreboard(context.Background(), season, week, 3)
		return playoffRoundMsg{season: season, week: week, board: board, err: err}
	}
}

func (m model) applyPlayoffRound(msg playoffRoundMsg) model {
	if msg.season != m.playoffSeason || m.playoffRoundData == nil {
		return m
	}
	entry := m.playoffRoundData[msg.week]
	entry.Loading = false
	switch {
	case errors.Is(msg.err, errNoGames):
		entry.Board, entry.Loaded, entry.Error = scoreboard{}, true, ""
	case msg.err != nil:
		entry.Error = plainText(msg.err.Error())
	default:
		entry.Board, entry.Loaded, entry.Error = msg.board, true, ""
	}
	m.playoffRoundData[msg.week] = entry
	return m
}

func (m model) playoffTabLine() string {
	tabs := []string{"1 PICTURE", "2 BRACKET"}
	tabs[m.playoffTab] = "[ " + tabs[m.playoffTab] + " ]"
	return sectionStyle.Render(strings.Join(tabs, "     "))
}

func (m model) activePlayoffLines(width int) []string {
	if m.playoffTab == playoffBracket {
		return m.bracketLines(width)
	}
	return m.playoffLines(width)
}

func (m model) maxPlayoffScroll() int {
	return max(0, len(m.activePlayoffLines(m.leagueWidth()))-m.leaguePageSize())
}

func (m model) updatePlayoffsKey(key string) (model, tea.Cmd) {
	page := m.leaguePageSize()
	setTab := func(tab int) (model, tea.Cmd) {
		m.playoffTab, m.playoffScroll = tab, 0
		return m.playoffLoads(false)
	}
	switch key {
	case "up", "k":
		m.playoffScroll = max(0, m.playoffScroll-1)
	case "down", "j":
		m.playoffScroll = min(m.maxPlayoffScroll(), m.playoffScroll+1)
	case "pgup":
		m.playoffScroll = max(0, m.playoffScroll-page)
	case "pgdown":
		m.playoffScroll = min(m.maxPlayoffScroll(), m.playoffScroll+page)
	case "left", "right", "h", "l":
		if m.playoffTab == playoffPicture {
			m.standingsConference = 1 - m.standingsConference
			m.playoffScroll = 0
		}
	case "1":
		return setTab(playoffPicture)
	case "2":
		return setTab(playoffBracket)
	case "tab":
		return setTab(1 - m.playoffTab)
	case "r":
		return m.playoffLoads(true)
	case "esc", "backspace":
		return m.back()
	}
	return m, nil
}

func (m model) renderPlayoffs(width int) string {
	title := "◆  PLAYOFFS"
	if m.season > 0 {
		title = fmt.Sprintf("◆  PLAYOFFS  ·  %d", m.season)
	}
	sub := m.playoffTabLine()
	footer := "1/2 tabs  ·  ↑/↓ scroll  ·  r refresh  ·  Esc back  ·  q quit"
	short := "1/2 · ↑/↓ · Esc · q"
	if m.playoffTab == playoffPicture {
		title += "  ·  " + m.conferenceName()
		footer = "←/→ conference · 1/2 tabs · ↑/↓ · r refresh · Esc back · q quit"
		short = "←/→ · " + short
	}
	return m.renderLeaguePage(title, sub, m.activePlayoffLines(m.leagueWidth()), m.playoffScroll, footer, short, width)
}
