package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"zenflow/pkg/filter"
	"zenflow/pkg/logger"
	"strings"
)

type tickMsg time.Time

type model struct {
	logs          []logger.LogEvent
	allowedCount  int
	blockedCount  int
	width         int
	height        int
	showBlocklist bool
	quitting       bool
	startTime      time.Time
	scrollOffset   int
	blockScroll    int
	adBlocker      *filter.Blocker
	malwareBlocker *filter.Blocker
	adPreview      []string
	mwPreview      []string
}

// RestartRequested is set to true when the user presses 'r' to restart the proxy
var RestartRequested bool

// New creates and returns a new Bubble Tea model for the ZenFlow TUI.
func New(adBlocker, malwareBlocker *filter.Blocker) tea.Model {
	return model{
		logs:           make([]logger.LogEvent, 0),
		startTime:      time.Now(),
		adBlocker:      adBlocker,
		malwareBlocker: malwareBlocker,
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
		case "r":
			RestartRequested = true
			m.quitting = true
			return m, tea.Quit
		case "c":
			m.logs = make([]logger.LogEvent, 0)
			return m, nil
		case "b":
			m.showBlocklist = !m.showBlocklist
			if m.showBlocklist {
				if m.adBlocker != nil && len(m.adPreview) == 0 {
					m.adPreview = m.adBlocker.GetAll()
				}
				if m.malwareBlocker != nil && len(m.mwPreview) == 0 {
					m.mwPreview = m.malwareBlocker.GetAll()
				}
			}
			return m, nil
		case "up", "k":
			if m.showBlocklist {
				if m.blockScroll > 0 {
					m.blockScroll--
				}
			} else {
				if m.scrollOffset < len(m.logs)-1 {
					m.scrollOffset++
				}
			}
			return m, nil
		case "down", "j":
			if m.showBlocklist {
				m.blockScroll++ // clamped in view
			} else {
				if m.scrollOffset > 0 {
					m.scrollOffset--
				}
			}
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
		adCount := 0
		if m.adBlocker != nil {
			adCount = m.adBlocker.Count()
		}
		mwCount := 0
		if m.malwareBlocker != nil {
			mwCount = m.malwareBlocker.Count()
		}

		headerText := titleStyle.Render("Blocklist Viewer") + "\n\n"
		headerText += fmt.Sprintf("Ad Domains Loaded:      %s\n", allowStyle.Render(fmt.Sprintf("%d", adCount)))
		headerText += fmt.Sprintf("Malware Domains Loaded: %s\n\n", blockStyle.Render(fmt.Sprintf("%d", mwCount)))
		
		maxScrollAd := len(m.adPreview) - 10
		if maxScrollAd < 0 { maxScrollAd = 0 }
		
		maxScrollMw := len(m.mwPreview) - 10
		if maxScrollMw < 0 { maxScrollMw = 0 }
		
		scrollAd := m.blockScroll
		if scrollAd > maxScrollAd { scrollAd = maxScrollAd }
		
		scrollMw := m.blockScroll
		if scrollMw > maxScrollMw { scrollMw = maxScrollMw }
		
		adEnd := scrollAd + 10
		if adEnd > len(m.adPreview) { adEnd = len(m.adPreview) }
		
		mwEnd := scrollMw + 10
		if mwEnd > len(m.mwPreview) { mwEnd = len(m.mwPreview) }
		
		adSlice := m.adPreview[scrollAd:adEnd]
		mwSlice := m.mwPreview[scrollMw:mwEnd]
		
		var adStr string
		if len(adSlice) > 0 {
			adStr = "  - " + strings.Join(adSlice, "\n  - ")
		} else {
			adStr = "  (No data)"
		}
		
		var mwStr string
		if len(mwSlice) > 0 {
			mwStr = "  - " + strings.Join(mwSlice, "\n  - ")
		} else {
			mwStr = "  (No data)"
		}
		
		colWidth := (logWidth - 4) / 2
		if colWidth < 20 { colWidth = 20 }
		colStyle := lipgloss.NewStyle().Width(colWidth).PaddingRight(2)
		
		adCol := colStyle.Render(allowStyle.Render("Ad Domains") + "\n" + adStr)
		mwCol := colStyle.Render(blockStyle.Render("Malware Domains") + "\n" + mwStr)
		
		listsText := lipgloss.JoinHorizontal(lipgloss.Top, adCol, mwCol)
		
		logsText = headerText + listsText
	} else {
		logsText = titleStyle.Render("Live Traffic Logs") + "\n\n"
		maxLogLines := m.height - 6
		if maxLogLines < 0 {
			maxLogLines = 0
		}
		count := 0
		startIndex := len(m.logs) - 1 - m.scrollOffset
		for i := startIndex; i >= 0; i-- { // reverse order (newest top)
			if count >= maxLogLines {
				break
			}
			event := m.logs[i]
			timeStr := event.Timestamp.Format("15:04:05")
			var line string
			if event.Action == "ALLOW" {
				line = allowStyle.Render(fmt.Sprintf("[%s] ALLOW %s %s", timeStr, event.Method, event.Host))
			} else {
				line = blockStyle.Render(fmt.Sprintf("[%s] BLOCK %s %s (%s)", timeStr, event.Method, event.Host, event.Reason))
			}

			vWidth := lipgloss.Width(line)
			linesUsed := 1
			if logWidth > 0 && vWidth > logWidth {
				linesUsed = (vWidth / logWidth) + 1
			}
			if count+linesUsed > maxLogLines && count > 0 {
				break
			}

			logsText += line + "\n"
			count += linesUsed
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
	statsText += "[q] Quit\n[r] Restart\n[c] Clear logs\n[b] Toggle blocklist view\n[up/k] Scroll up\n[down/j] Scroll down"

	statsPanel := panelStyle.Width(statWidth).Height(m.height - 2).Render(statsText)

	// Join horizontally
	return lipgloss.JoinHorizontal(lipgloss.Top, logsPanel, statsPanel)
}
