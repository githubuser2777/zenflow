package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"zenflow/pkg/logger"
)

type tickMsg time.Time

type model struct {
	logs          []logger.LogEvent
	allowedCount  int
	blockedCount  int
	width         int
	height        int
	showBlocklist bool
	quitting      bool
	startTime     time.Time
}

// New creates and returns a new Bubble Tea model for the ZenFlow TUI.
func New() tea.Model {
	return model{
		logs:      make([]logger.LogEvent, 0),
		startTime: time.Now(),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		waitForActivity(),
		tickCmd(),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func waitForActivity() tea.Cmd {
	return func() tea.Msg {
		return <-logger.EventChannel
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tickMsg:
		return m, tickCmd()

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "c":
			m.logs = make([]logger.LogEvent, 0)
			return m, nil
		case "b":
			m.showBlocklist = !m.showBlocklist
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case logger.LogEvent:
		m.logs = append(m.logs, msg)
		if len(m.logs) > 100 { // keep last 100 logs
			m.logs = m.logs[1:]
		}
		if msg.Action == "ALLOW" {
			m.allowedCount++
		} else if msg.Action == "BLOCK" {
			m.blockedCount++
		}
		return m, waitForActivity()
	}

	return m, nil
}

func (m model) View() string {
	if m.quitting {
		return "Shutting down ZenFlow...\n"
	}

	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	// Styles
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FAFAFA")).Background(lipgloss.Color("#7D56F4")).Padding(0, 1)
	allowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575"))
	blockStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	panelStyle := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)

	// Calculate widths
	logWidth := m.width*2/3 - 4
	statWidth := m.width/3 - 4

	if logWidth <= 0 || statWidth <= 0 {
		return "Terminal too small"
	}

	// Logs Panel
	var logsText string
	if m.showBlocklist {
		logsText = titleStyle.Render("Blocklist Viewer") + "\n\n(Blocklist viewing not yet fully implemented. Press 'b' to return to logs)"
	} else {
		logsText = titleStyle.Render("Live Traffic Logs") + "\n\n"
		for i := len(m.logs) - 1; i >= 0; i-- { // reverse order (newest top)
			event := m.logs[i]
			timeStr := event.Timestamp.Format("15:04:05")
			var line string
			if event.Action == "ALLOW" {
				line = allowStyle.Render(fmt.Sprintf("[%s] ALLOW %s %s", timeStr, event.Method, event.Host))
			} else {
				line = blockStyle.Render(fmt.Sprintf("[%s] BLOCK %s %s (%s)", timeStr, event.Method, event.Host, event.Reason))
			}

			// truncate string
			if len(line) > logWidth+100 { // basic approximation because of ANSI codes
				// In a real terminal app we'd strip ANSI to check length or use lipgloss.Truncate
			}
			logsText += line + "\n"
		}
	}
	logsPanel := panelStyle.Width(logWidth).Height(m.height - 2).Render(logsText)

	// Stats Panel
	statsText := titleStyle.Render("ZenFlow Stats") + "\n\n"
	
	uptime := time.Since(m.startTime)
	h := int(uptime.Hours())
	min := int(uptime.Minutes()) % 60
	s := int(uptime.Seconds()) % 60
	statsText += fmt.Sprintf("Uptime:  %02d:%02d:%02d\n", h, min, s)

	statsText += fmt.Sprintf("Allowed: %s\n", allowStyle.Render(fmt.Sprintf("%d", m.allowedCount)))
	statsText += fmt.Sprintf("Blocked: %s\n", blockStyle.Render(fmt.Sprintf("%d", m.blockedCount)))
	total := m.allowedCount + m.blockedCount
	if total > 0 {
		blockedPct := float64(m.blockedCount) / float64(total) * 100
		statsText += fmt.Sprintf("Block Rate: %.1f%%\n", blockedPct)
	}

	statsText += "\n\n" + titleStyle.Render("Hotkeys") + "\n\n"
	statsText += "[q] Quit\n[c] Clear logs\n[b] Toggle blocklist view"

	statsPanel := panelStyle.Width(statWidth).Height(m.height - 2).Render(statsText)

	// Join horizontally
	return lipgloss.JoinHorizontal(lipgloss.Top, logsPanel, statsPanel)
}
