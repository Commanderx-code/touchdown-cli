package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#88959B"))
	liveStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF6961"))
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F0FFF9")).Background(lipgloss.Color("#23483E"))
)

type boardMsg struct {
	board                    scoreboard
	err                      error
	season, week, seasonType int
}
type detailMsg struct {
	id     string
	detail gameDetail
	err    error
}
type standingsMsg struct {
	season int
	data   standings
	err    error
}
type teamScheduleMsg struct {
	season int
	team   string
	data   teamSchedule
	err    error
}
type teamLeadersMsg struct {
	season  int
	team    string
	leaders []seasonLeader
	err     error
}
type alertMsg struct{ err error }
type tickMsg time.Time

type model struct {
	api                      *client
	board                    scoreboard
	detail                   gameDetail
	detailGame               game
	detailID                 string
	standings                standings
	standingsSeason          int
	standingsUpdated         time.Time
	standingsError           string
	standingsLoading         bool
	standingsConference      int
	standingsScroll          int
	showStandings            bool
	standingsFromHome        bool
	showHome                 bool
	showHelp                 bool
	showSettings             bool
	settingsFromHome         bool
	settingsTeam             int
	settingsTheme            int
	settingsScroll           int
	settingsError            string
	theme                    string
	configPath               string
	showTeam                 bool
	teamFromHome             bool
	teamData                 teamSchedule
	teamSeason               int
	teamUpdated              time.Time
	teamLeaders              []seasonLeader
	teamLeadersReady         bool
	teamLeadersLoading       bool
	teamError                string
	teamLeaderError          string
	teamLoading              bool
	teamScroll               int
	teamSelected             int
	teamNotice               string
	homeSelection            int
	favorite                 string
	favoritesOnly            bool
	liveOnly                 bool
	season, week, seasonType int
	selected                 int
	showDetail               bool
	detailFromHome           bool
	detailFromTeam           bool
	pendingFavoriteGame      bool
	detailTab                int
	detailScroll             int
	loading                  bool
	errorText                string
	updated                  time.Time
	width, height            int
	notify, notifyAll        bool
	compact                  bool
}

func (m model) loadBoard() tea.Cmd {
	season, week, seasonType := m.season, m.week, m.seasonType
	return func() tea.Msg {
		board, err := m.api.scoreboard(context.Background(), season, week, seasonType)
		return boardMsg{board, err, season, week, seasonType}
	}
}

func (m model) loadDetail(id string) tea.Cmd {
	return func() tea.Msg {
		detail, err := m.api.summary(context.Background(), id)
		return detailMsg{id, detail, err}
	}
}

func (m model) loadStandings() tea.Cmd {
	season := m.season
	return func() tea.Msg {
		data, err := m.api.standings(context.Background(), season)
		return standingsMsg{season, data, err}
	}
}

func (m model) loadTeam() tea.Cmd {
	season, team := m.season, m.favorite
	return func() tea.Msg {
		data, err := m.api.teamSchedule(context.Background(), team, season)
		return teamScheduleMsg{season, team, data, err}
	}
}

func (m model) loadTeamLeaders(data teamSchedule) tea.Cmd {
	season, team := m.season, m.favorite
	return func() tea.Msg {
		leaders, err := m.api.teamLeaders(context.Background(), team, data)
		return teamLeadersMsg{season, team, leaders, err}
	}
}

func (m model) teamScheduleDue() bool {
	if m.season <= 0 || m.favorite == "" || m.teamLoading {
		return false
	}
	return m.teamUpdated.IsZero() || time.Since(m.teamUpdated) > 5*time.Minute
}

func (m model) visibleGames() []game {
	if !m.favoritesOnly && !m.liveOnly {
		return m.board.Events
	}
	var games []game
	for _, g := range m.board.Events {
		away, home := g.sides()
		if (m.liveOnly && g.Status.Type.State == "in") || (m.favoritesOnly && m.favorite != "" && (away.Team.Abbreviation == m.favorite || home.Team.Abbreviation == m.favorite)) {
			games = append(games, g)
		}
	}
	return games
}

func (m model) openFavoriteSeason(notice string) (model, tea.Cmd) {
	m.teamFromHome = m.showHome
	m.showHome = false
	m.showTeam = true
	m.showDetail = false
	m.showStandings = false
	m.teamScroll = 0
	m.teamSelected = 0
	m.teamNotice = notice
	if m.season > 0 && m.teamSeason != m.season {
		if !m.teamLoading {
			m.teamLoading = true
			return m, m.loadTeam()
		}
		return m, nil
	}
	if m.teamSeason == m.season && len(m.teamData.Events) > 0 && !m.teamLeadersReady && !m.teamLeadersLoading {
		m.teamLeadersLoading = true
		return m, m.loadTeamLeaders(m.teamData)
	}
	return m, nil
}

func (m model) openFavoriteGame() (model, tea.Cmd) {
	for _, game := range m.board.Events {
		away, home := game.sides()
		if away.Team.Abbreviation != m.favorite && home.Team.Abbreviation != m.favorite {
			continue
		}
		m.detailFromHome = m.showHome
		m.detailFromTeam = false
		m.showHome = false
		m.showTeam = false
		m.showStandings = false
		m.showDetail = true
		m.detailID = game.ID
		m.detailGame = game
		m.detail = gameDetail{}
		m.detailTab = 0
		m.detailScroll = 0
		for i, visible := range m.visibleGames() {
			if visible.ID == game.ID {
				m.selected = i
				break
			}
		}
		return m, m.loadDetail(game.ID)
	}
	notice := "No " + favoriteLabel(m.favorite) + " game this week · showing season page"
	if m.loading {
		notice = "Scores are loading · showing season page"
	}
	return m.openFavoriteSeason(notice)
}

func (m model) currentDetailGame() *game {
	if m.detailGame.ID == m.detailID && m.detailID != "" {
		return &m.detailGame
	}
	for i := range m.board.Events {
		if m.board.Events[i].ID == m.detailID {
			return &m.board.Events[i]
		}
	}
	return nil
}

func (m model) scrollBy(delta int) model {
	switch {
	case m.showSettings:
		m.settingsTeam = max(0, min(m.settingsTeam+delta, len(nflTeams)-1))
		if m.settingsTeam < m.settingsScroll {
			m.settingsScroll = m.settingsTeam
		}
		if m.settingsTeam >= m.settingsScroll+m.settingsPageSize() {
			m.settingsScroll = m.settingsTeam - m.settingsPageSize() + 1
		}
	case m.showHome:
		m.homeSelection = max(0, min(m.homeSelection+delta, len(m.homeOptions())-1))
	case m.showStandings:
		m.standingsScroll = max(0, min(m.standingsScroll+delta, m.maxStandingsScroll()))
	case m.showTeam:
		if len(m.teamData.Events) == 0 {
			m.teamScroll = max(0, min(m.teamScroll+delta, m.maxTeamScroll()))
			break
		}
		m.teamSelected = max(0, min(m.teamSelected+delta, len(m.teamData.Events)-1))
		selectedLine := m.teamSelectedLine()
		if selectedLine < m.teamScroll {
			m.teamScroll = selectedLine
		}
		if selectedLine >= m.teamScroll+m.teamPageSize() {
			m.teamScroll = selectedLine - m.teamPageSize() + 1
		}
		if m.teamSelected == 0 && delta < 0 {
			m.teamScroll = 0
		}
		m.teamScroll = max(0, min(m.teamScroll, m.maxTeamScroll()))
	case m.showDetail:
		m.detailScroll = max(0, min(m.detailScroll+delta, m.maxDetailScroll()))
	default:
		m.selected = max(0, min(m.selected+delta, len(m.visibleGames())-1))
	}
	return m
}

func (m model) scrollPageSize() int {
	switch {
	case m.showSettings:
		return m.settingsPageSize()
	case m.showStandings:
		return m.standingsPageSize()
	case m.showTeam:
		return m.teamPageSize()
	case m.showDetail:
		return m.detailPageSize(len(m.statLines()))
	default:
		return 1
	}
}

func tick() tea.Cmd { return tea.Tick(30*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) }) }

func (m model) Init() tea.Cmd { return tea.Batch(m.loadBoard(), tick()) }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.settingsScroll = min(m.settingsScroll, max(0, len(nflTeams)-m.settingsPageSize()))
		m.standingsScroll = min(m.standingsScroll, m.maxStandingsScroll())
		m.teamScroll = min(m.teamScroll, m.maxTeamScroll())
		m.detailScroll = min(m.detailScroll, m.maxDetailScroll())
	case boardMsg:
		if m.season != msg.season || m.week != msg.week || m.seasonType != msg.seasonType {
			break
		}
		m.loading = false
		if msg.err != nil {
			m.errorText = msg.err.Error()
			m.pendingFavoriteGame = false
			break
		}
		alerts := []scoreAlert{}
		if m.notify && msg.board.CachedAt.IsZero() && m.board.CachedAt.IsZero() && len(m.board.Events) > 0 {
			alerts = scoreAlerts(m.board, msg.board, m.favorite, m.notifyAll)
		}
		m.board = msg.board
		if m.showDetail {
			for _, game := range msg.board.Events {
				if game.ID == m.detailID {
					m.detailGame = game
					break
				}
			}
		}
		m.errorText = ""
		m.updated = time.Now()
		if !msg.board.CachedAt.IsZero() {
			m.updated = msg.board.CachedAt
		}
		if m.season == 0 {
			if len(msg.board.Leagues) > 0 {
				m.season = msg.board.Leagues[0].Season.Year
				m.seasonType, _ = strconv.Atoi(msg.board.Leagues[0].Season.Type.ID)
			}
			m.week = msg.board.Week.Number
		}
		if m.selected >= len(m.visibleGames()) {
			m.selected = max(0, len(m.visibleGames())-1)
		}
		if m.pendingFavoriteGame {
			m.pendingFavoriteGame = false
			if !m.showDetail && !m.showStandings && !m.showTeam {
				next, cmd := m.openFavoriteGame()
				if next.teamScheduleDue() {
					next.teamLoading = true
					return next, tea.Batch(cmd, next.loadTeam())
				}
				return next, cmd
			}
		}
		var cmds []tea.Cmd
		if m.standingsSeason != m.season && m.season > 0 && !m.standingsLoading {
			m.standingsLoading = true
			cmds = append(cmds, m.loadStandings())
		}
		if m.teamScheduleDue() && m.teamSeason != m.season {
			m.teamLoading = true
			cmds = append(cmds, m.loadTeam())
		}
		if m.showDetail && m.detailID != "" {
			cmds = append(cmds, m.loadDetail(m.detailID))
		}
		for _, alert := range alerts {
			cmds = append(cmds, sendAlert(alert))
		}
		if len(cmds) > 0 {
			return m, tea.Batch(cmds...)
		}
	case detailMsg:
		if msg.id != m.detailID {
			break
		}
		if msg.err != nil {
			m.errorText = msg.err.Error()
			break
		}
		m.detail = msg.detail
		if m.detailGame.ID == msg.id && len(msg.detail.Header) > 0 {
			header := msg.detail.Header[0]
			if len(m.detailGame.Competitions) == 0 {
				m.detailGame.Competitions = []competition{{}}
			}
			if len(header.Competitors) > 0 {
				m.detailGame.Competitions[0].Competitors = header.Competitors
			}
			if header.Status.Type.State != "" {
				m.detailGame.Status = header.Status
			}
			if header.Situation.Possession != "" {
				m.detailGame.Competitions[0].Situation = header.Situation
			}
		}
		m.errorText = ""
	case standingsMsg:
		if msg.season != m.season {
			break
		}
		m.standingsLoading = false
		if m.showStandings {
			m.loading = false
		}
		if msg.err != nil {
			m.standingsError = msg.err.Error()
			break
		}
		firstLoad := m.standingsSeason != msg.season
		m.standings = msg.data
		m.standingsSeason = msg.season
		m.standingsUpdated = time.Now()
		m.standingsError = ""
		if firstLoad && !m.standingsFromHome {
			for i, group := range msg.data.Children {
				for _, entry := range group.Standings.Entries {
					if entry.Team.Abbreviation == m.favorite {
						m.standingsConference = i
						break
					}
				}
			}
		}
	case teamScheduleMsg:
		if msg.season != m.season || msg.team != m.favorite {
			break
		}
		m.teamLoading = false
		if msg.err != nil {
			m.teamError = msg.err.Error()
			m.teamUpdated = time.Now()
			break
		}
		m.teamData = msg.data
		m.teamSelected = max(0, min(m.teamSelected, len(msg.data.Events)-1))
		m.teamSeason = msg.season
		m.teamUpdated = time.Now()
		m.teamError = ""
		m.teamLeaderError = ""
		m.teamLeaders = nil
		m.teamLeadersReady = false
		if m.showTeam {
			m.teamLeadersLoading = true
			return m, m.loadTeamLeaders(msg.data)
		}
	case teamLeadersMsg:
		if msg.season != m.season || msg.team != m.favorite {
			break
		}
		m.teamLeaders = msg.leaders
		m.teamLeadersReady = true
		m.teamLeadersLoading = false
		if msg.err != nil {
			m.teamLeaderError = msg.err.Error()
		} else {
			m.teamLeaderError = ""
		}
	case alertMsg:
		// Notifications are optional; a missing desktop service does not interrupt the TUI.
	case tea.MouseMsg:
		if m.showHelp {
			break
		}
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m = m.scrollBy(-3)
		case tea.MouseButtonWheelDown:
			m = m.scrollBy(3)
		}
	case tickMsg:
		if (m.showHome || m.showTeam || !m.showDetail && !m.showStandings) && m.teamScheduleDue() {
			m.teamLoading = true
			return m, tea.Batch(m.loadBoard(), m.loadTeam(), tick())
		}
		if m.showStandings {
			if !m.standingsLoading {
				m.standingsLoading = true
				return m, tea.Batch(m.loadStandings(), tick())
			}
			return m, tick()
		}
		if m.season > 0 && !m.standingsLoading && (m.standingsSeason != m.season || time.Since(m.standingsUpdated) > 5*time.Minute) {
			m.standingsLoading = true
			return m, tea.Batch(m.loadBoard(), m.loadStandings(), tick())
		}
		return m, tea.Batch(m.loadBoard(), tick())
	case tea.KeyMsg:
		key := msg.String()
		if m.showHelp {
			if key == "?" || key == "esc" || key == "backspace" {
				m.showHelp = false
			}
			if key == "q" || key == "ctrl+c" {
				return m, tea.Quit
			}
			return m, nil
		}
		if key == "?" {
			m.showHelp = true
			return m, nil
		}
		if m.showSettings {
			return m.updateSettingsKey(key)
		}
		if key == "c" {
			m.pendingFavoriteGame = false
			return m.openSettings(), nil
		}
		if key == "p" && !m.showDetail && !m.showStandings && !m.showTeam && m.favorite != "" {
			m.pendingFavoriteGame = false
			return m.openFavoriteSeason("")
		}
		switch key {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc", "backspace":
			if m.showHome {
				break
			}
			wasStandings := m.showStandings
			wasTeam := m.showTeam
			wasDetail := m.showDetail
			m.showDetail = false
			m.showStandings = false
			m.showTeam = false
			m.detailID = ""
			m.errorText = ""
			if wasStandings && m.standingsFromHome {
				m.showHome = true
			}
			if wasTeam && m.teamFromHome {
				m.showHome = true
			}
			if wasDetail && m.detailFromHome {
				m.showHome = true
			}
			if wasDetail && m.detailFromTeam {
				m.showTeam = true
				m.detailFromTeam = false
			}
			if wasStandings {
				return m, m.loadBoard()
			}
			if !wasDetail && !wasTeam {
				m.showHome = true
			}
		case "r":
			m.loading = true
			if m.showTeam {
				m.teamLoading = true
				return m, m.loadTeam()
			}
			if m.showStandings {
				m.standingsLoading = true
				return m, m.loadStandings()
			}
			return m, m.loadBoard()
		case "up", "k":
			m = m.scrollBy(-1)
		case "down", "j":
			m = m.scrollBy(1)
		case "pgup":
			m = m.scrollBy(-m.scrollPageSize())
		case "pgdown":
			m = m.scrollBy(m.scrollPageSize())
		case "enter":
			if m.showHome {
				next, cmd := m.openHomeSelection()
				return next, cmd
			}
			if m.showTeam && m.teamSeason == m.season && len(m.teamData.Events) > 0 {
				event := m.teamData.Events[m.teamSelected]
				m.detailID = event.ID
				m.detailGame = event.toGame()
				for _, current := range m.board.Events {
					if current.ID == event.ID {
						m.detailGame = current
						break
					}
				}
				m.detail = gameDetail{}
				m.showTeam = false
				m.showDetail = true
				m.detailFromTeam = true
				m.detailFromHome = false
				m.detailTab = 0
				m.detailScroll = 0
				m.errorText = ""
				return m, m.loadDetail(event.ID)
			}
			games := m.visibleGames()
			if !m.showDetail && !m.showStandings && !m.showTeam && m.selected < len(games) {
				m.detailID = games[m.selected].ID
				m.detailGame = games[m.selected]
				m.detail = gameDetail{}
				m.showDetail = true
				m.detailFromHome = false
				m.detailFromTeam = false
				m.detailTab = 0
				m.detailScroll = 0
				return m, m.loadDetail(m.detailID)
			}
		case "1", "2", "3", "4", "tab":
			if m.showDetail {
				switch msg.String() {
				case "tab":
					m.detailTab = (m.detailTab + 1) % 4
				case "1":
					m.detailTab = 0
				case "2":
					m.detailTab = 1
				case "3":
					m.detailTab = 2
				case "4":
					m.detailTab = 3
				}
				m.detailScroll = 0
			}
		case "left", "h", "right", "l":
			if m.showHome {
				break
			}
			if m.showTeam {
				break
			}
			if m.showStandings {
				if msg.String() == "left" || msg.String() == "h" {
					m.standingsConference = 0
				} else {
					m.standingsConference = 1
				}
				m.standingsScroll = 0
				break
			}
			if m.showDetail {
				if msg.String() == "left" || msg.String() == "h" {
					m.detailTab = max(0, m.detailTab-1)
				} else {
					m.detailTab = min(3, m.detailTab+1)
				}
				m.detailScroll = 0
				break
			}
			if m.season == 0 {
				break
			}
			delta := -1
			if msg.String() == "right" || msg.String() == "l" {
				delta = 1
			}
			limit := 18
			if m.seasonType != 2 {
				limit = 4
			}
			if m.week+delta < 1 || m.week+delta > limit {
				break
			}
			m.week += delta
			m.selected = 0
			m.loading = true
			m.errorText = ""
			return m, m.loadBoard()
		case "f":
			if !m.showHome && !m.showDetail && !m.showStandings && !m.showTeam && m.favorite != "" {
				m.liveOnly = false
				m.favoritesOnly = !m.favoritesOnly
				m.selected = 0
			}
		case "g":
			if !m.showDetail && !m.showStandings && !m.showTeam && m.favorite != "" {
				if m.loading && len(m.board.Events) == 0 {
					m.pendingFavoriteGame = true
					return m, nil
				}
				return m.openFavoriteGame()
			}
		case "s":
			if !m.showDetail && !m.showStandings && !m.showTeam && m.season > 0 {
				m.standingsFromHome = m.showHome
				m.showHome = false
				m.showStandings = true
				m.standingsScroll = 0
				if team, ok := findTeam(m.favorite); ok && m.standingsSeason != m.season {
					if team.conference == "NFC" {
						m.standingsConference = 1
					} else {
						m.standingsConference = 0
					}
				}
				if m.standingsSeason != m.season {
					if !m.standingsLoading {
						m.standingsLoading = true
						return m, m.loadStandings()
					}
				}
			}
		case "m":
			m.showHome = true
			m.showStandings = false
			m.showTeam = false
			m.showDetail = false
			m.detailID = ""
			m.detailFromHome = false
			m.detailFromTeam = false
		}
	}
	return m, nil
}

func (m model) openHomeSelection() (model, tea.Cmd) {
	m.pendingFavoriteGame = false
	m.showHome = false
	m.showDetail = false
	m.showStandings = false
	m.showTeam = false
	m.selected = 0
	switch m.homeSelection {
	case 0:
		m.favoritesOnly, m.liveOnly = false, false
	case 1:
		m.favoritesOnly, m.liveOnly = false, true
	case 2:
		m.favoritesOnly, m.liveOnly = true, false
	case 3:
		m.showTeam = true
		m.teamFromHome = true
		m.teamScroll = 0
		m.teamSelected = 0
		m.teamNotice = ""
		if m.season > 0 && m.teamSeason != m.season {
			if !m.teamLoading {
				m.teamLoading = true
				return m, m.loadTeam()
			}
			return m, nil
		}
		if m.teamSeason == m.season && len(m.teamData.Events) > 0 && !m.teamLeadersReady && !m.teamLeadersLoading {
			m.teamLeadersLoading = true
			return m, m.loadTeamLeaders(m.teamData)
		}
	case 4, 5:
		m.standingsFromHome = true
		m.showStandings = true
		m.standingsScroll = 0
		if m.homeSelection == 4 {
			m.standingsConference = 1
		} else {
			m.standingsConference = 0
		}
		if m.standingsSeason != m.season && m.season > 0 && !m.standingsLoading {
			m.standingsLoading = true
			return m, m.loadStandings()
		}
	}
	return m, nil
}

func main() {
	defaultPath, err := defaultConfigPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	season := flag.Int("season", 0, "NFL season year (defaults to current)")
	week := flag.Int("week", 0, "week number (requires -season)")
	seasonType := flag.Int("type", 2, "season type: 1 preseason, 2 regular, 3 postseason")
	once := flag.Bool("once", false, "print scoreboard once without opening the TUI")
	favorite := flag.String("favorite", "", "favorite team for this launch (overrides config)")
	configPath := flag.String("config", defaultPath, "configuration file path")
	setFavorite := flag.String("set-favorite", "", "save favorite team (e.g. BUF or Bills) and exit")
	showConfig := flag.Bool("show-config", false, "show configuration path and current favorite")
	theme := flag.String("theme", "", "color theme for this launch: mint, amber, or mono")
	compact := flag.Bool("compact", false, "use a compact terminal layout")
	notify := flag.Bool("notify", false, "desktop alerts for favorite team scores and finals")
	notifyAll := flag.Bool("notify-all", false, "desktop alerts for all scores and finals (implies -notify)")
	flag.Parse()
	if *setFavorite != "" {
		team, err := saveFavorite(*configPath, *setFavorite)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		fmt.Printf("Favorite team: %s %s (%s)\nConfig: %s\n", team.city, team.nickname, team.abbreviation, *configPath)
		return
	}
	config, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	chosen := config.FavoriteTeam
	if *favorite != "" {
		chosen = *favorite
	}
	team, err := resolveTeam(chosen)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if *showConfig {
		fmt.Printf("Config: %s\nFavorite team: %s %s (%s)\nTheme: %s\nGame key: g\nSeason key: p\n", *configPath, team.city, team.nickname, team.abbreviation, config.Theme)
		return
	}
	chosenTheme := config.Theme
	if *theme != "" {
		chosenTheme = *theme
	}
	if err := applyTheme(chosenTheme); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	maxWeek := 18
	if *seasonType != 2 {
		maxWeek = 4
	}
	if (*season == 0) != (*week == 0) || *seasonType < 1 || *seasonType > 3 || *week < 0 || *week > maxWeek {
		fmt.Fprintln(os.Stderr, "use -season and -week together; -type must be 1–3 and week must fit the season type")
		os.Exit(2)
	}
	api := newClient()
	if *once {
		board, err := api.scoreboard(context.Background(), *season, *week, *seasonType)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if !board.CachedAt.IsZero() {
			fmt.Fprintf(os.Stderr, "Using cached scores from %s\n", board.CachedAt.Local().Format(time.RFC1123))
		}
		for _, g := range board.Events {
			away, home := g.sides()
			if g.Status.Type.State == "pre" {
				fmt.Printf("%-4s @ %-4s  %s\n", away.Team.Abbreviation, home.Team.Abbreviation, g.statusText())
			} else {
				fmt.Printf("%-4s %2s @ %-4s %2s  %s\n", away.Team.Abbreviation, away.Score, home.Team.Abbreviation, home.Score, g.statusText())
			}
		}
		return
	}
	m := model{api: api, season: *season, week: *week, seasonType: *seasonType, loading: true, favorite: team.abbreviation, showHome: true, compact: *compact, notify: *notify || *notifyAll, notifyAll: *notifyAll, theme: chosenTheme, configPath: *configPath}
	if _, err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
