package data

import (
	"fmt"
	"net"
	"time"

	"golang.org/x/net/context"
)

// Global data types and functions etc
const CSVHeader = "Timestamp,Addr,Rtt,Rtt(Milliseconds),IPAddr,Nbytes,Seq,TTL,ID,Timeout\n"

// Return a CSV representation of the packet data
func (dsp *DataSetPacket) CSV() string {
	return fmt.Sprintf("%s,%s,%s,%d,%s,%d,%d,%d,%d,%v",
		dsp.Timestamp.Format(time.DateTime),
		dsp.Addr,
		dsp.Rtt,
		dsp.Rtt.Milliseconds(),
		dsp.IPAddr.String(),
		dsp.Nbytes,
		dsp.Seq,
		dsp.TTL,
		dsp.ID,
		dsp.Timeout,
	)
}

// Convert a line of CSV into a packet data structure
func (*DataSetPacket) FromCSV(csv string) error {
	return nil
}

// Comments are mostly yoinked from the probing lib
type DataSetPacket struct {
	Context      context.Context
	ResponseChan chan *DataSetPacket
	// send or recv
	Type string
	// Timestamp of when this data was collected.
	Timestamp time.Time
	// Timed out
	Timeout bool
	// Addr is the string address of the host being pinged.
	Addr string
	// Rtt is the round-trip time it took to ping.
	Rtt time.Duration
	// IPAddr is the address of the host being pinged.
	IPAddr *net.IPAddr
	// NBytes is the number of bytes in the message.
	Nbytes int
	// Seq is the ICMP sequence number.
	Seq int
	// TTL is the Time To Live on the packet. Doesn't work on Windows
	TTL int
	// ID is the ICMP identifier.
	ID int
}

type ColorHost struct {
	Color string
	Host  string
}
