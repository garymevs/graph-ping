package main

import (
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/alitto/pond/v2"
	probing "github.com/prometheus-community/pro-bing"

	tslc "github.com/NimbleMarkets/ntcharts/linechart/timeserieslinechart"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

type TimestampedPacket struct {
	Timestamp time.Time
	Packet    probing.Packet
}

type model struct {
	chart       tslc.Model
	zoneManager *zone.Manager
	packetChan  chan *probing.Packet
	pingerList  []*probing.Pinger
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			// Kill the pingers when we close the gui
			for _, pinger := range m.pingerList {
				pinger.Stop()
			}
			return m, tea.Quit
		}
	}
	select {
	case packet := <-m.packetChan:
		m.chart.Push(tslc.TimePoint{Time: time.Now(), Value: float64(packet.Rtt.Seconds())})
	default:
	}

	// forward Bubble Tea Msg to time series chart
	// and draw all data sets using braille runes
	m.chart, _ = m.chart.Update(msg)
	m.chart.DrawBrailleAll()
	return m, nil
}

func (m model) View() string {
	// call bubblezone Manager.Scan() at root model
	return m.zoneManager.Scan(
		lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("63")). // purple
			Render(m.chart.View()),
	)
}

// TODO: Need to set up tea stuff to update on channel update like:
// https://github.com/charmbracelet/bubbletea/blob/main/examples/realtime/main.go

func main() {
	// TODO: structure this betterer
	width := 60
	height := 24
	chart := tslc.New(width, height)

	// TODO: stuff that will be cmd inputable
	pingDstList := []string{"www.google.com"}
	// count := 4
	// interval := 1000 // in milliseconds

	// Set up packet channel to receive pings for processing
	// o.o packet-chan
	packetChan := make(chan *probing.Packet)

	// Set up all the pingers
	pingerList := []*probing.Pinger{}
	for _, host := range pingDstList {
		pinger, err := probing.NewPinger(host)
		if err != nil {
			// If we can't set up one of the pingers then we should just give up
			panic(err)
		}
		pinger.SetPrivileged(true)
		pinger.Count = 10

		// TODO: set up the inputable parameters

		pinger.OnRecv = func(pkt *probing.Packet) {
			//fmt.Printf("%s: %d bytes from %s: icmp_seq=%d time=%v\n",
			//	host, pkt.Nbytes, pkt.IPAddr, pkt.Seq, pkt.Rtt)
			packetChan <- pkt
		}

		pinger.OnDuplicateRecv = func(pkt *probing.Packet) {
			// Ignore duplicates for now
			//fmt.Printf("%s: %d bytes from %s: icmp_seq=%d time=%v ttl=%v (DUP!)\n", host, pkt.Nbytes, pkt.IPAddr, pkt.Seq, pkt.Rtt, pkt.TTL)
		}

		pinger.OnFinish = func(stats *probing.Statistics) {
			fmt.Printf("\n--- %s: %s ping statistics ---\n", host, stats.Addr)
			fmt.Printf("%d packets transmitted, %d packets received, %v%% packet loss\n",
				stats.PacketsSent, stats.PacketsRecv, stats.PacketLoss)
			fmt.Printf("round-trip min/avg/max/stddev = %v/%v/%v/%v\n",
				stats.MinRtt, stats.AvgRtt, stats.MaxRtt, stats.StdDevRtt)
		}

		pingerList = append(pingerList, pinger)
	}

	// Listen for Ctrl-C.
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for _ = range c {
			fmt.Println("Received interrupt signal. Shutting down...")
			for _, p := range pingerList {
				p.Stop()
			}
			os.Exit(0)
		}
	}()

	// Pool up the pingers and wait for them to finish
	pingPool := pond.NewPool(len(pingerList))
	for _, pinger := range pingerList {
		pingPool.Submit(func() {
			pinger.Run()
		})
	}

	// Handle the packets from packetChan
	//go func() {
	//	for packet := range packetChan {
	//		chart.Push(tslc.TimePoint{Time: time.Now(), Value: float64(packet.Rtt.Seconds())})
	//		chart.Draw()
	//	}
	//}()

	// mouse support is enabled with BubbleZone
	zoneManager := zone.New()
	chart.SetZoneManager(zoneManager)
	chart.Focus() // set focus to process keyboard and mouse messages

	// start new Bubble Tea program with mouse support enabled
	m := model{chart, zoneManager, packetChan, pingerList}
	if _, err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}

	// Wait for all the pingers to finish
	pingPool.StopAndWait()
	os.Exit(0)
}
