package tui

import (
	"fmt"
	"graph-ping/data"

	probing "github.com/prometheus-community/pro-bing"

	tslc "github.com/NimbleMarkets/ntcharts/linechart/timeserieslinechart"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

type Model struct {
	Chart       tslc.Model
	ZoneManager *zone.Manager
	PacketChan  chan *data.DataSetPacket
	PingerList  []*probing.Pinger
	HostList    []data.ColorHost
	HighestPing float64
	DebugText   string
}

func StartTUI(
	packetChan chan *data.DataSetPacket,
	pingerList []*probing.Pinger,
	hostList []string,
) error {
	// TODO: Fix scrolling / key control for viewing
	chart := tslc.New(80, 24)
	chart.YLabelFormatter = func(i int, y float64) string {
		return fmt.Sprintf("%.3f", y)
	}

	chart.XLabelFormatter = tslc.HourTimeLabelFormatter()
	chart.UpdateHandler = tslc.HourNoZoomUpdateHandler(1)
	//chart.AutoMaxY = true
	//chart.SetViewYRange(0, 0.05)

	pingDstList := []data.ColorHost{}
	for i, host := range hostList {
		style := lipgloss.NewStyle().
			Foreground(lipgloss.Color(fmt.Sprintf("%d", i+1)))
		chart.SetDataSetStyle(host, style)
		pingDstList = append(pingDstList,
			data.ColorHost{
				Color: fmt.Sprintf("%d", i+1),
				Host:  host,
			})
	}

	// TODO: this doesn't seem to work
	// mouse support is enabled with BubbleZone
	zoneManager := zone.New()
	chart.SetZoneManager(zoneManager)
	chart.Focus() // set focus to process keyboard and mouse messages

	// start new Bubble Tea program with mouse support enabled
	m := Model{
		Chart:       chart,
		ZoneManager: zoneManager,
		PacketChan:  packetChan,
		PingerList:  pingerList,
		HostList:    pingDstList,
		HighestPing: 0.0,
		DebugText:   "",
	}

	// Seems to block until tea.Quit is fired
	if _, err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run(); err != nil {
		return err
	}

	// Kill the pingers when we close the gui
	for _, pinger := range m.PingerList {
		pinger.Stop()
	}

	return nil
}

func waitForPing(packetChan chan *data.DataSetPacket) tea.Cmd {
	return func() tea.Msg {
		return <-packetChan
	}
}

func (m Model) Init() tea.Cmd {
	m.Chart.DrawXYAxisAndLabel()
	return tea.Batch(
		waitForPing(m.PacketChan),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Really hacky chart scaling
		newWidth := int(float64(msg.Width) * 0.99)
		newHeight := int(float64(msg.Height) * 0.9)
		if msg.Width <= 100 {
			newWidth = int(float64(msg.Width)*0.99) - 1
		}
		if msg.Height <= 20 {
			newHeight = int(float64(msg.Height)*0.9) - 1
		}
		if msg.Height <= 10 {
			newHeight = int(float64(msg.Height)*0.9) - 2
		}
		m.Chart.Resize(newWidth, newHeight)
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "down", "left", "right", "pgup", "pgdown":
			//m.DebugText = "Key pressed: " + msg.String()
			m.Chart, _ = m.Chart.Update(msg)
			m.Chart.DrawBrailleAll()
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case tea.MouseMsg:
		m.Chart, _ = m.Chart.Update(msg)
		m.Chart.DrawBrailleAll()
	case *data.DataSetPacket:
		m.Chart.PushDataSet(msg.Addr, tslc.TimePoint{Time: msg.Timestamp, Value: msg.Rtt.Seconds()})
		if msg.Rtt.Seconds() > m.HighestPing {
			m.HighestPing = msg.Rtt.Seconds()
			m.Chart.SetViewYRange(0, m.HighestPing*2)
		}
		//m.Chart, _ = m.Chart.Update(msg)
		m.Chart.DrawBrailleAll()
		return m, waitForPing(m.PacketChan)
	}
	return m, nil
}

func (m Model) View() string {
	s :=
		lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("63")). // purple
			Render(m.Chart.View())

	s += "\n"
	s += "Exit: q/ctrl+c    "
	for _, host := range m.HostList {
		s += lipgloss.NewStyle().
			Foreground(lipgloss.Color(host.Color)).
			Render(host.Host)
		s += "    "
	}
	s += m.DebugText
	return m.ZoneManager.Scan(s)
	//return s
}
