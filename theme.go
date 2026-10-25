package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type palette struct{ dim, live, selectedFG, selectedBG, border, text, accent, section, logo, shadow, subtitle string }

var themeNames = []string{"mint", "amber", "mono"}

func themeIndex(name string) int {
	for i, candidate := range themeNames {
		if candidate == strings.ToLower(strings.TrimSpace(name)) {
			return i
		}
	}
	return -1
}

func applyTheme(name string) error {
	themes := map[string]palette{
		"mint":  {"#88959B", "#FF6961", "#F0FFF9", "#23483E", "#47615C", "#EDF5F2", "#79E4B7", "#85BBD3", "#F5F4FF", "#667489", "#9FA9B6"},
		"amber": {"#A79B8A", "#FF726C", "#221A0E", "#F4BD69", "#8B6843", "#FFF4DF", "#F8C474", "#E5A96F", "#FFF0CE", "#7E5936", "#C5AA85"},
		"mono":  {"#999999", "#FFFFFF", "#101010", "#E6E6E6", "#888888", "#F0F0F0", "#FFFFFF", "#D0D0D0", "#FFFFFF", "#777777", "#AAAAAA"},
	}
	p, ok := themes[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return fmt.Errorf("unknown theme %q: choose mint, amber, or mono", name)
	}
	dimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.dim))
	liveStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(p.live))
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(p.selectedFG)).Background(lipgloss.Color(p.selectedBG))
	borderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.border))
	textStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.text))
	accentStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(p.accent))
	sectionStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(p.section))
	logoStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(p.logo))
	shadowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(p.shadow))
	subtitleStyle = lipgloss.NewStyle().Italic(true).Foreground(lipgloss.Color(p.subtitle))
	return nil
}
