package output

import (
	"graph-ping/data"
	"os"
)

func Init(fileOutputPath string, outputFileChan chan *data.DataSetPacket) error {
	// Really fileOutputPath should have been validated at this point but whatevs
	if fileOutputPath != "" {
		outputFile, err := os.Create(fileOutputPath)
		if err != nil {
			return err
		}

		outputFile.WriteString("Timestamp,Addr,Rtt,IPAddr,Nbytes,Seq,TTL,ID\n")

		// Start the loop writing to the output file
		go func() {
			// We close outputFileChan when the TUI is done so this should be a clean exit
			for packet := range outputFileChan {
				_, err := outputFile.WriteString(packet.CSV() + "\n")
				if err != nil {
					panic(err)
				}
			}
			// When we are done with the channel, close the file too
			outputFile.Close()
		}()
	}

	return nil
}
