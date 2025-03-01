package cmd

import (
	"context"
	"errors"
	"fmt"
	"graph-ping/data"
	"graph-ping/output"
	"graph-ping/tui"
	"os"
	"time"

	"github.com/alitto/pond/v2"
	probing "github.com/prometheus-community/pro-bing"
	"github.com/urfave/cli/v3"
)

func Init() {
	cmd := &cli.Command{
		Name:        "graph-ping",
		Usage:       "multi-ping TUI and logging solution",
		Description: "graph-ping is a tool that can ping multiple hosts, display the results in a TUI and log to a CSV",
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:  "host",
				Usage: "Provide hosts to ping. Example: graph-ping --host google.com --host 192.168.1.1",
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "Specify output file path. Example: ./output.csv or C:/output.csv",
			},
			&cli.IntFlag{
				Name:    "interval",
				Aliases: []string{"i"},
				Usage:   "Set the interval between pings in milliseconds",
				Value:   1000,
			},
			&cli.IntFlag{
				Name:    "timeout",
				Aliases: []string{"t"},
				Usage:   "Set the timeout for each ping in milliseconds (must be less than interval)",
				Value:   999,
			},
			&cli.IntFlag{
				Name:    "count",
				Aliases: []string{"c"},
				Usage:   "Set the number of pings to send. 0: infinite",
				Value:   0,
			},
			&cli.BoolFlag{
				Name:  "nogui",
				Usage: "Disable TUI output (why you no like graph? ;-;)",
				Value: false,
			},
		},
		//Commands: []*cli.Command{
		//	{
		//		Name:  "replay",
		//		Usage: "Load a previously saved ping session to inspect",
		//		Flags: []cli.Flag{
		//			&cli.StringFlag{
		//				Name:    "input",
		//				Aliases: []string{"i"},
		//				Usage:   "Path to the saved ping session file",
		//			},
		//		},
		//	},
		//},
		Action: func(ctx context.Context, com *cli.Command) error {
			if len(com.StringSlice("host")) == 0 {
				cli.ShowAppHelp(com)
				println("\nAt least one host must be specified.\n")
				return nil
			}
			if com.Int("timeout") > com.Int("interval") {
				println("\ntimeout must be less than interval\n")
				return nil
			}

			pingConfig := &PingConfig{
				Hosts:          com.StringSlice("host"),
				Interval:       int(com.Int("interval")),
				Count:          int(com.Int("count")),
				OutputFilePath: com.String("output"),
				NoGUI:          com.Bool("nogui"),
				Timeout:        int(com.Int("timeout")),
			}
			return Ping(pingConfig)
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		// Program should have stopped by this point so just tell the user what went wrong
		println(err.Error())
	}

}

type PingConfig struct {
	Hosts          []string
	Interval       int
	Count          int
	OutputFilePath string
	NoGUI          bool
	Timeout        int
}

func Ping(config *PingConfig) error {
	// Set up packet channel to receive pings for processing
	// o.o packet-chan
	tuiPacketChan := make(chan *data.DataSetPacket)
	// Send data to be written to the output file
	outputFileChan := make(chan *data.DataSetPacket)

	// Channel for managing send and recv packets
	pingManagerChan := make(chan *data.DataSetPacket)

	// Existing file check
	if _, err := os.Stat(config.OutputFilePath); err == nil {
		return errors.New("output file already exists")
	}

	// Output file writer setup
	if config.OutputFilePath != "" {
		err := output.Init(config.OutputFilePath, outputFileChan)
		if err != nil {
			// TODO: This can probably be handled betterer but this works for now
			return errors.New("failed to initialize output file writer")
		}
	}

	// Set up the pingers
	pingerList := []*probing.Pinger{}
	for _, host := range config.Hosts {
		pinger, err := probing.NewPinger(host)
		if err != nil {
			// If we can't set up one of the pingers then we should just give up
			return err
		}
		// Needed for Windows (not sure on other platforms)
		pinger.SetPrivileged(true)
		pinger.RecordRtts = false
		pinger.RecordTTLs = false
		pinger.Count = config.Count
		pinger.Interval = time.Duration(config.Interval) * time.Millisecond

		pinger.OnSend = func(pkt *probing.Packet) {
			dataSetPacket := &data.DataSetPacket{
				Type:      "send",
				Timestamp: time.Now(),
				Addr:      host,
				Rtt:       pkt.Rtt,
				IPAddr:    pkt.IPAddr,
				Nbytes:    pkt.Nbytes,
				Seq:       pkt.Seq,
				TTL:       pkt.TTL,
				ID:        pkt.ID,
			}
			pingManagerChan <- dataSetPacket
		}

		pinger.OnRecv = func(pkt *probing.Packet) {
			dataSetPacket := &data.DataSetPacket{
				Type:      "recv",
				Timestamp: time.Now(),
				Addr:      host,
				Rtt:       pkt.Rtt,
				IPAddr:    pkt.IPAddr,
				Nbytes:    pkt.Nbytes,
				Seq:       pkt.Seq,
				TTL:       pkt.TTL,
				ID:        pkt.ID,
			}
			pingManagerChan <- dataSetPacket
		}

		// Kinda worthless as it just spits out errors regarding 0.0.0.0
		pinger.OnRecvError = func(err error) {
			if config.NoGUI {
				//fmt.Printf("Error receiving packet: %v\n", err)
			}
		}

		pinger.OnFinish = func(stats *probing.Statistics) {
			// If nogui is enabled then print this
			if config.NoGUI {
				fmt.Printf("\n--- %s: %s ping statistics ---\n", host, stats.Addr)
				fmt.Printf("%d packets transmitted, %d packets received, %v%% packet loss\n",
					stats.PacketsSent, stats.PacketsRecv, stats.PacketLoss)
				fmt.Printf("round-trip min/avg/max/stddev = %v/%v/%v/%v\n",
					stats.MinRtt, stats.AvgRtt, stats.MaxRtt, stats.StdDevRtt)
			}
		}

		pingerList = append(pingerList, pinger)
	}

	if config.NoGUI {
		fmt.Printf("Starting %d pingers", len(pingerList))
	}

	go ManagePackets(pingManagerChan,
		config.Timeout,
		config.NoGUI,
		(config.OutputFilePath != ""),
		outputFileChan,
		tuiPacketChan,
	)

	// Pool up the pingers and wait for them to finish
	pingPool := pond.NewPool(len(pingerList))
	for _, pinger := range pingerList {
		pingPool.Submit(func() {
			pinger.Run()
		})
	}

	// TUI enabled
	if !config.NoGUI {
		err := tui.StartTUI(tuiPacketChan, pingerList, config.Hosts)
		if err != nil {
			return err
		}
	} else {
		// TUI disabled, just run the pingers and print results to stdout
		// Wait for the pool to finish
		pingPool.StopAndWait()
	}

	// Close channels
	close(tuiPacketChan)
	close(outputFileChan)
	close(pingManagerChan)

	return nil
}

// There has got to be a better way to do this but I dunno
func ManagePackets(
	pingManagerChan chan *data.DataSetPacket,
	timeout int,
	noTUI bool,
	fileOutput bool,
	outputFileChan chan *data.DataSetPacket,
	tuiPacketChan chan *data.DataSetPacket,
) {
	// Pings addressed by ID then Seq
	storedPackets := make(map[int]map[int]*data.DataSetPacket)
	for packet := range pingManagerChan {
		switch packet.Type {
		case "send":
			if _, ok := storedPackets[packet.ID]; !ok {
				storedPackets[packet.ID] = make(map[int]*data.DataSetPacket)
			}
			// Set up a timeout context waiting for the recv packet to clear it
			go ManageSentPacket(
				packet,
				timeout,
				noTUI,
				fileOutput,
				outputFileChan,
				tuiPacketChan,
			)
			// Store packet so we can check for a response later
			storedPackets[packet.ID][packet.Seq] = packet
		case "recv":
			// Check if storedPings still has a reference for this packet
			if storedPacket, ok := storedPackets[packet.ID][packet.Seq]; ok {
				// Tell the sent packet that we got a response
				storedPacket.ResponseChan <- packet
				// Clear stored sent packet
				delete(storedPackets[packet.ID], packet.Seq)
			}
		}
	}
}

func ManageSentPacket(
	packet *data.DataSetPacket,
	timeout int,
	noTUI bool,
	fileOutput bool,
	outputFileChan chan *data.DataSetPacket,
	tuiPacketChan chan *data.DataSetPacket,
) {
	var cancel context.CancelFunc
	packet.Context, cancel = context.WithTimeout(context.Background(), time.Duration(timeout)*time.Millisecond)
	defer cancel()
	packet.ResponseChan = make(chan *data.DataSetPacket)
	select {
	case <-packet.Context.Done():
		// TODO: in this case the sent packet will be left in storedPackets but I can live with that for now
		// We hit the timeout for the packet
		// Set the RTT to 0 so it looks like a timeout (or a really fast ping I guess lol)
		// Also set a flag to indicate that this was a timeout
		packet.Rtt = 0
		packet.Timeout = true
		if !noTUI {
			tuiPacketChan <- packet
		} else {
			PrintPacket(packet)
		}
		if fileOutput {
			outputFileChan <- packet
		}
	case recvPacket := <-packet.ResponseChan:
		// Received a packet matching our sent one
		if !noTUI {
			tuiPacketChan <- recvPacket
		} else {
			PrintPacket(recvPacket)
		}
		if fileOutput {
			outputFileChan <- recvPacket
		}
	}
}

func PrintPacket(packet *data.DataSetPacket) {
	fmt.Printf("%s: %d bytes from %s: icmp_seq=%d time=%v id=%d timeout=%v\n",
		packet.Addr,
		packet.Nbytes,
		packet.IPAddr,
		packet.Seq,
		packet.Rtt,
		packet.ID,
		packet.Timeout,
	)
}
