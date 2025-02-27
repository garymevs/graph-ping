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
}

func InitChart(width int, height int) tslc.Model {
	// TODO: Fix scrolling / key control for viewing
	chart := tslc.New(width, height)
	chart.YLabelFormatter = func(i int, y float64) string {
		return fmt.Sprintf("%.3f", y)
	}

	chart.XLabelFormatter = tslc.HourTimeLabelFormatter()
	chart.AutoMaxY = true
	chart.SetViewYRange(0, 0.05)

	return chart
}

func waitForPing(packetChan chan *data.DataSetPacket) tea.Cmd {
	return func() tea.Msg {
		return <-packetChan
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		waitForPing(m.PacketChan),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// TODO: implement window resizing logic here
		println(msg.Width)
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			// Kill the pingers when we close the gui
			for _, pinger := range m.PingerList {
				pinger.Stop()
			}
			return m, tea.Quit
		}
	case *data.DataSetPacket:
		// TODO: Change this to pushing data sets
		m.Chart.PushDataSet(msg.DataSetName, tslc.TimePoint{Time: msg.Timestamp, Value: msg.Packet.Rtt.Seconds()})
		if msg.Packet.Rtt.Seconds() > m.HighestPing {
			m.HighestPing = msg.Packet.Rtt.Seconds()
			m.Chart.SetViewYRange(0, m.HighestPing*2)
		}
		m.Chart, _ = m.Chart.Update(msg)
		m.Chart.DrawBrailleAll()
		return m, waitForPing(m.PacketChan)
	}
	return m, nil
}

func (m Model) View() string {
	// call bubblezone Manager.Scan() at root model
	// Not sure this is the correct use of scan here but we'll see
	s := m.ZoneManager.Scan(
		lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("63")). // purple
			Render(m.Chart.View()),
	)
	s += "\n"
	for _, host := range m.HostList {
		s += lipgloss.NewStyle().
			Foreground(lipgloss.Color(host.Color)).
			Render(host.Host)
		s += "    "
	}
	return s
}
