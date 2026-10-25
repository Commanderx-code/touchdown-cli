package main

import (
	"encoding/json"
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
	FavoriteTeam string `json:"favorite_team"`
	Theme        string `json:"theme"`
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
	return config, nil
}

func saveFavorite(path, value string) (teamInfo, error) {
	team, err := resolveTeam(value)
	if err != nil {
		return team, err
	}
	if err := updateConfig(path, map[string]string{"favorite_team": team.abbreviation}); err != nil {
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
	return updateConfig(path, map[string]string{"favorite_team": team.abbreviation, "theme": themeNames[index]})
}

func updateConfig(path string, values map[string]string) error {
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
