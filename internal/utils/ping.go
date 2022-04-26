package utils

import (
	"fmt"

	"github.com/go-ping/ping"
	"go.uber.org/zap"
)

func Ping(address string, numTries int, logger *zap.Logger) bool {
	pinger, err := ping.NewPinger(address)
	if err != nil {
		logger.Error(fmt.Sprintf("Cannot create pinger due to error: %s", err))
		return false
	}
	pinger.Count = numTries
	// Run 2 seconds max
	pinger.Timeout = 2
	err = pinger.Run() // Blocks until finished.
	if err != nil {
		logger.Error(fmt.Sprintf("Cannot ping due to error: %s", err))
		return false
	}
	stats := pinger.Statistics()

	// If there are loss packets
	if stats.PacketsRecv != 0 {
		// If the round trip time is more than 2s
		if stats.AvgRtt.Seconds() > 2 {
			return false
		}
	}
	return true
}
