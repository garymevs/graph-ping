package data

import (
	"time"

	probing "github.com/prometheus-community/pro-bing"
)

// Global data types etc

type DataSetPacket struct {
	DataSetName string
	Timestamp   time.Time
	Packet      *probing.Packet
}

type ColorHost struct {
	Color string
	Host  string
}
