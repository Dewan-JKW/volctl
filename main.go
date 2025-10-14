package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type App struct {
	ID            string
	Name          string
	Volume        int
	DisplayVolume int
}

type model struct {
	apps     []App
	selected int
	width    int
	height   int
}

type TickMsg struct{}

func initialModel() model {
	apps := getApps()

	for i := range apps {
		if apps[i].Volume > 100 {
			apps[i].Volume = 100
		}
		apps[i].DisplayVolume = apps[i].Volume
	}
	return model{apps: apps, selected: 0}
}

func (m model) Init() tea.Cmd {
	return tick()
}

func tick() tea.Cmd {
	return tea.Tick(time.Millisecond*30, func(time.Time) tea.Msg {
		return TickMsg{}
	})
}

func getApps() []App {
	cmd := exec.Command("pactl", "list", "sink-inputs")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil
	}

	lines := strings.Split(out.String(), "\n")
	var apps []App
	var current App

	for _, line := range lines {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Sink Input #"):
			if current.ID != "" {
				apps = append(apps, current)
			}
			current = App{ID: strings.TrimPrefix(line, "Sink Input #")}
		case strings.HasPrefix(line, "application.name"):
			current.Name = extractValue(line)
		case strings.HasPrefix(line, "media.name"):
			if current.Name == "" {
				current.Name = extractValue(line)
			}
		case strings.HasPrefix(line, "Volume:"):
			if vol := extractVolume(line); vol >= 0 {
				if vol > 100 {
					vol = 100
				}
				current.Volume = vol
			}
		}
	}
	if current.ID != "" {
		apps = append(apps, current)
	}
	return apps
}

func extractValue(line string) string {
	if idx := strings.Index(line, "="); idx != -1 {
		val := strings.Trim(line[idx+1:], `" `)
		return val
	}
	return ""
}

func extractVolume(line string) int {
	parts := strings.Fields(line)
	for _, p := range parts {
		if strings.HasSuffix(p, "%") {
			p = strings.TrimSuffix(p, "%")
			if val, err := strconv.Atoi(p); err == nil {
				if val > 100 {
					val = 100
				} else if val < 0 {
					val = 0
				}
				return val
			}
		}
	}
	return -1
}

func setVolume(id string, delta int) {
	current := getAppVolume(id)
	target := current + delta
	if target > 100 {
		target = 100
	} else if target < 0 {
		target = 0
	}
	delta = target - current
	if delta == 0 {
		return
	}
	op := fmt.Sprintf("%+d%%", delta)
	exec.Command("pactl", "set-sink-input-volume", id, op).Run()
}

func getAppVolume(id string) int {
	apps := getApps()
	for _, a := range apps {
		if a.ID == id {
			if a.Volume > 100 {
				return 100
			}
			return a.Volume
		}
	}
	return 0
}

func updateVolumes(m *model) {
	systemApps := getApps()
	for i := range m.apps {
		for _, sysApp := range systemApps {
			if m.apps[i].ID == sysApp.ID {
				if sysApp.Volume > 100 {
					sysApp.Volume = 100
				}
				m.apps[i].Volume = sysApp.Volume
			}
		}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up":
			if m.selected > 0 {
				m.selected--
			}
		case "down":
			if m.selected < len(m.apps)-1 {
				m.selected++
			}
		case "right":
			if len(m.apps) > 0 {
				setVolume(m.apps[m.selected].ID, +2)
				updateVolumes(&m)
			}
		case "left":
			if len(m.apps) > 0 {
				setVolume(m.apps[m.selected].ID, -2)
				updateVolumes(&m)
			}
		}

	case TickMsg:
		changed := false
		for i := range m.apps {
			if m.apps[i].DisplayVolume < m.apps[i].Volume {
				m.apps[i].DisplayVolume++
				changed = true
			} else if m.apps[i].DisplayVolume > m.apps[i].Volume {
				m.apps[i].DisplayVolume--
				changed = true
			}
		}
		if changed {
			return m, tick()
		} else {
			return m, tick()
		}
	}

	return m, nil
}

func (m model) View() string {
	if len(m.apps) == 0 {
		return "No active audio streams found.\n"
	}

	s := "\n Volume Mixer \n\n"

	for i, app := range m.apps {
		bar := getVolumeBar(app.DisplayVolume, m.width)
		maxNameLen := 15
		name := app.Name
		if len(name) > maxNameLen {
			name = name[:maxNameLen-1] + "…"
		}
		line := fmt.Sprintf("%-15s %s %3d%%", name, bar, app.Volume)
		if i == m.selected {
			line = fmt.Sprintf("> %s <", line)
		} else {
			line = fmt.Sprintf("  %s", line)
		}
		s += line + "\n"
	}

	s += "\n q to quit\n"
	return s
}

func getVolumeBar(v, totalWidth int) string {
	padding := 25
	barLength := totalWidth - padding
	if barLength < 10 {
		barLength = 10
	}
	filled := v * barLength / 100
	bar := strings.Repeat("░", filled) + strings.Repeat(" ", barLength-filled)
	return "[" + bar + "]"
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	_, err := p.Run()
	if err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
