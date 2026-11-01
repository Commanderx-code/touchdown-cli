package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type teamInfo struct{ abbreviation, city, nickname, conference string }

var nflTeams = []teamInfo{
	{"ARI", "Arizona", "Cardinals", "NFC"}, {"ATL", "Atlanta", "Falcons", "NFC"},
	{"BAL", "Baltimore", "Ravens", "AFC"}, {"BUF", "Buffalo", "Bills", "AFC"},
	{"CAR", "Carolina", "Panthers", "NFC"}, {"CHI", "Chicago", "Bears", "NFC"},
	{"CIN", "Cincinnati", "Bengals", "AFC"}, {"CLE", "Cleveland", "Browns", "AFC"},
	{"DAL", "Dallas", "Cowboys", "NFC"}, {"DEN", "Denver", "Broncos", "AFC"},
	{"DET", "Detroit", "Lions", "NFC"}, {"GB", "Green Bay", "Packers", "NFC"},
	{"HOU", "Houston", "Texans", "AFC"}, {"IND", "Indianapolis", "Colts", "AFC"},
	{"JAX", "Jacksonville", "Jaguars", "AFC"}, {"KC", "Kansas City", "Chiefs", "AFC"},
	{"LV", "Las Vegas", "Raiders", "AFC"}, {"LAC", "Los Angeles", "Chargers", "AFC"},
	{"LAR", "Los Angeles", "Rams", "NFC"}, {"MIA", "Miami", "Dolphins", "AFC"},
	{"MIN", "Minnesota", "Vikings", "NFC"}, {"NE", "New England", "Patriots", "AFC"},
	{"NO", "New Orleans", "Saints", "NFC"}, {"NYG", "New York", "Giants", "NFC"},
	{"NYJ", "New York", "Jets", "AFC"}, {"PHI", "Philadelphia", "Eagles", "NFC"},
	{"PIT", "Pittsburgh", "Steelers", "AFC"}, {"SF", "San Francisco", "49ers", "NFC"},
	{"SEA", "Seattle", "Seahawks", "NFC"}, {"TB", "Tampa Bay", "Buccaneers", "NFC"},
	{"TEN", "Tennessee", "Titans", "AFC"}, {"WSH", "Washington", "Commanders", "NFC"},
}

func findTeam(abbreviation string) (teamInfo, bool) {
	for _, team := range nflTeams {
		if team.abbreviation == strings.ToUpper(abbreviation) {
			return team, true
		}
	}
	return teamInfo{}, false
}

func resolveTeam(value string) (teamInfo, error) {
	value = strings.ToLower(strings.Join(strings.Fields(value), " "))
	for _, team := range nflTeams {
		if value == strings.ToLower(team.abbreviation) || value == strings.ToLower(team.nickname) || value == strings.ToLower(team.city+" "+team.nickname) {
			return team, nil
		}
	}
	return teamInfo{}, fmt.Errorf("unknown NFL team %q; use an abbreviation such as GB or BUF", value)
}

func favoriteLabel(abbreviation string) string {
	if team, ok := findTeam(abbreviation); ok {
		return strings.ToUpper(team.nickname)
	}
	return abbreviation
}

type appConfig struct {
	FavoriteTeam  string   `json:"favorite_team"`
	Theme         string   `json:"theme"`
	FollowedTeams []string `json:"followed_teams"`
}

func defaultConfigPath() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "touchdown", "config.json"), nil
}

func loadConfig(path string) (appConfig, error) {
	config := appConfig{FavoriteTeam: "GB", Theme: "mint"}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return config, nil
	}
	if err != nil {
		return config, err
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return config, fmt.Errorf("read config %s: %w", path, err)
	}
	if config.FavoriteTeam == "" {
		config.FavoriteTeam = "GB"
	}
	team, err := resolveTeam(config.FavoriteTeam)
	if err != nil {
		return config, fmt.Errorf("read config %s: %w", path, err)
	}
	config.FavoriteTeam = team.abbreviation
	if config.Theme == "" {
		config.Theme = "mint"
	}
	if themeIndex(config.Theme) < 0 {
		return config, fmt.Errorf("read config %s: unknown theme %q", path, config.Theme)
	}
	config.Theme = themeNames[themeIndex(config.Theme)]
	if config.FollowedTeams, err = normalizeFollowed(config.FollowedTeams, config.FavoriteTeam); err != nil {
		return config, fmt.Errorf("read config %s: %w", path, err)
	}
	return config, nil
}

// normalizeFollowed resolves team names to abbreviations, keeping the first
// of any repeats. The favorite is already always shown, so it is dropped.
func normalizeFollowed(values []string, favorite string) ([]string, error) {
	var teams []string
	seen := map[string]bool{favorite: true}
	for _, value := range values {
		team, err := resolveTeam(strings.TrimSpace(value))
		if err != nil {
			return nil, err
		}
		if !seen[team.abbreviation] {
			seen[team.abbreviation] = true
			teams = append(teams, team.abbreviation)
		}
	}
	return teams, nil
}

// parseFollowed reads a comma-separated team list; an empty one follows nobody.
func parseFollowed(value, favorite string) ([]string, error) {
	var values []string
	for _, part := range strings.Split(value, ",") {
		if strings.TrimSpace(part) != "" {
			values = append(values, part)
		}
	}
	return normalizeFollowed(values, favorite)
}

func followedLabel(teams []string) string {
	if len(teams) == 0 {
		return "none"
	}
	return strings.Join(teams, ", ")
}

func flagWasSet(fs *flag.FlagSet, name string) bool {
	found := false
	fs.Visit(func(f *flag.Flag) { found = found || f.Name == name })
	return found
}

// followNote explains why a favorite named in -follow is not in the saved list.
func followNote(value, favorite string) string {
	for _, part := range strings.Split(value, ",") {
		if team, err := resolveTeam(strings.TrimSpace(part)); err == nil && team.abbreviation == favorite {
			return favorite + " is your favorite, so it is always shown."
		}
	}
	return ""
}

func saveFollowed(path string, teams []string) error {
	if teams == nil {
		teams = []string{}
	}
	return updateConfig(path, map[string]any{"followed_teams": teams})
}

func saveFavorite(path, value string) (teamInfo, error) {
	team, err := resolveTeam(value)
	if err != nil {
		return team, err
	}
	if err := updateConfig(path, map[string]any{"favorite_team": team.abbreviation}); err != nil {
		return team, err
	}
	return team, nil
}

func saveSettings(path, favorite, theme string) error {
	team, err := resolveTeam(favorite)
	if err != nil {
		return err
	}
	index := themeIndex(theme)
	if index < 0 {
		return fmt.Errorf("unknown theme %q", theme)
	}
	return updateConfig(path, map[string]any{"favorite_team": team.abbreviation, "theme": themeNames[index]})
}

func updateConfig(path string, values map[string]any) error {
	data := map[string]json.RawMessage{}
	if existing, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(existing, &data); err != nil {
			return fmt.Errorf("read config %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if data == nil {
		data = map[string]json.RawMessage{}
	}
	for key, value := range values {
		data[key], _ = json.Marshal(value)
	}
	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(encoded, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	return nil
}
