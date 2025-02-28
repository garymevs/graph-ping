package data

import (
	"fmt"
	"net"
	"time"
)

// Global data types and functions etc

// Return a CSV representation of the packet data
func (dsp *DataSetPacket) CSV() string {
	return fmt.Sprintf("%s,%s,%s,%s,%d,%d,%d,%d",
		dsp.Timestamp.Format(time.DateTime),
		dsp.Addr,
		dsp.Rtt,
		dsp.IPAddr.String(),
		dsp.Nbytes,
		dsp.Seq,
		dsp.TTL,
		dsp.ID,
	)
}

// Convert a line of CSV into a packet data structure
func (*DataSetPacket) FromCSV(csv string) error {
	return nil
}

// Comments are mostly yoinked from the probing lib
type DataSetPacket struct {
	// Timestamp of when this data was collected.
	Timestamp time.Time
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
	// TTL is the Time To Live on the packet.
	TTL int
	// ID is the ICMP identifier.
	ID int
}

type ColorHost struct {
	Color string
	Host  string
}
