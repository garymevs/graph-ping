package cmd

import (
	"context"
	"fmt"
	"graph-ping/data"
	"graph-ping/gui"
	"os"
	"time"

	"github.com/alitto/pond/v2"
	probing "github.com/prometheus-community/pro-bing"
	"github.com/urfave/cli/v3"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

func Init() {
	cmd := &cli.Command{
		Name:        "graph-ping",
		Usage:       "Graph ping results in a GUI",
		Description: "graph-ping is a tool that pings multiple hosts and displays the results in a graphical interface. It uses the Prometheus community's pro-bing library to perform the pinging and the Charmbracelet's bubbletea library to create the GUI.",
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:  "host",
				Usage: "Provide hosts to ping. Example: graph-ping --host google.com --host 192.168.1.1",
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "Specify output file path. Example: ./output.gpr or C:/output.gpr. NOT IMPLEMENTED YET",
			},
			&cli.IntFlag{
				Name:    "interval",
				Aliases: []string{"i"},
				Usage:   "Set the interval between pings in milliseconds",
				Value:   1000,
			},
			&cli.IntFlag{
				Name:    "count",
				Aliases: []string{"c"},
				Usage:   "Set the number of pings to send. 0: infinite",
				Value:   0,
			},
			&cli.BoolFlag{
				Name:  "nogui",
				Usage: "Disable GUI output (why you no like graph? ;-;) NOT IMPLEMENTED YET",
				Value: false,
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "replay",
				Usage: "Load a previously saved ping session to inspect",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "input",
						Aliases: []string{"i"},
						Usage:   "Path to the saved ping session file",
					},
				},
			},
			{
				Name:  "convert",
				Usage: "Convert a previously saved ping session to CSV or JSON",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "input",
						Aliases: []string{"i"},
						Usage:   "Path to the saved ping session file",
					},
					&cli.StringFlag{
						Name:    "output",
						Aliases: []string{"o"},
						Usage:   "Specify output file path. Example: ./output.gpr or C:/output.gpr. NOT IMPLEMENTED YET",
					},
				},
			},
		},
		Action: func(ctx context.Context, com *cli.Command) error {
			if len(com.StringSlice("host")) == 0 {
				cli.ShowAppHelp(com)
				println("\nAt least one host must be specified.\n")
				return nil
			}
			pingConfig := &PingConfig{
				Hosts:          com.StringSlice("host"),
				Interval:       int(com.Int("interval")),
				Count:          int(com.Int("count")),
				OutputFilePath: com.String("output"),
				NoGUI:          com.Bool("nogui"),
			}
			Ping(pingConfig)

			return nil
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		panic(err)
	}

}

type PingConfig struct {
	Hosts          []string
	Interval       int
	Count          int
	OutputFilePath string
	NoGUI          bool
}

func Ping(config *PingConfig) {
	chart := gui.InitChart(80, 24) // width, height
	pingDstList := []data.ColorHost{}

	//TODO: Make this only run if we are doing gui shizzles
	for i, host := range config.Hosts {
		style := lipgloss.NewStyle().
			Foreground(lipgloss.Color(fmt.Sprintf("%d", i+1)))
		chart.SetDataSetStyle(host, style)
		pingDstList = append(pingDstList, data.ColorHost{
			Host:  host,
			Color: fmt.Sprintf("%d", i+1),
		})
	}

	// Set up packet channel to receive pings for processing
	// o.o packet-chan
	packetChan := make(chan *data.DataSetPacket)

	// Set up all the pingers
	pingerList := []*probing.Pinger{}
	for _, host := range pingDstList {
		pinger, err := probing.NewPinger(host.Host)
		if err != nil {
			// If we can't set up one of the pingers then we should just give up
			panic(err)
		}
		pinger.SetPrivileged(true)
		pinger.Count = config.Count
		pinger.Interval = time.Duration(config.Interval) * time.Millisecond

		pinger.OnRecv = func(pkt *probing.Packet) {
			//fmt.Printf("%s: %d bytes from %s: icmp_seq=%d time=%v\n",
			//	host, pkt.Nbytes, pkt.IPAddr, pkt.Seq, pkt.Rtt)
			packetChan <- &data.DataSetPacket{DataSetName: host.Host, Timestamp: time.Now(), Packet: pkt}
		}

		pinger.OnDuplicateRecv = func(pkt *probing.Packet) {
			// Ignore duplicates for now
			//fmt.Printf("%s: %d bytes from %s: icmp_seq=%d time=%v ttl=%v (DUP!)\n", host, pkt.Nbytes, pkt.IPAddr, pkt.Seq, pkt.Rtt, pkt.TTL)
		}

		pinger.OnFinish = func(stats *probing.Statistics) {
			// Don't do anything here. We really don't care
			//fmt.Printf("\n--- %s: %s ping statistics ---\n", host, stats.Addr)
			//fmt.Printf("%d packets transmitted, %d packets received, %v%% packet loss\n",
			//	stats.PacketsSent, stats.PacketsRecv, stats.PacketLoss)
			//fmt.Printf("round-trip min/avg/max/stddev = %v/%v/%v/%v\n",
			//	stats.MinRtt, stats.AvgRtt, stats.MaxRtt, stats.StdDevRtt)
		}

		pingerList = append(pingerList, pinger)
	}

	// Listen for Ctrl-C.
	// Don't think we need this anymore
	//c := make(chan os.Signal, 1)
	//signal.Notify(c, os.Interrupt)
	//go func() {
	//	for _ = range c {
	//		fmt.Println("Received interrupt signal. Shutting down...")
	//		for _, p := range pingerList {
	//			p.Stop()
	//		}
	//		os.Exit(0)
	//	}
	//}()

	// Pool up the pingers and wait for them to finish
	pingPool := pond.NewPool(len(pingerList))
	for _, pinger := range pingerList {
		pingPool.Submit(func() {
			pinger.Run()
		})
	}

	// mouse support is enabled with BubbleZone
	zoneManager := zone.New()
	chart.SetZoneManager(zoneManager)
	chart.Focus() // set focus to process keyboard and mouse messages

	// start new Bubble Tea program with mouse support enabled
	m := gui.Model{
		Chart:       chart,
		ZoneManager: zoneManager,
		PacketChan:  packetChan,
		PingerList:  pingerList,
		HostList:    pingDstList,
		HighestPing: 0.0,
		DebugText:   "",
	}

	if _, err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}

	// Wait for all the pingers to finish
	// Never actually hit if the GUI is doing its thing
	pingPool.StopAndWait()
}
