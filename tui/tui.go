package gui

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

func InitChart(width int, height int) tslc.Model {
	// TODO: Fix scrolling / key control for viewing
	chart := tslc.New(width, height)
	chart.YLabelFormatter = func(i int, y float64) string {
		return fmt.Sprintf("%.3f", y)
	}

	chart.XLabelFormatter = tslc.HourTimeLabelFormatter()
	chart.UpdateHandler = tslc.HourNoZoomUpdateHandler(1)
	//chart.AutoMaxY = true
	//chart.SetViewYRange(0, 0.05)

	return chart
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
	// call bubblezone Manager.Scan() at root model
	// Not sure this is the correct use of scan here but we'll see
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
