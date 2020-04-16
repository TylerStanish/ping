package main

import (
	"fmt"
	"math"
	"time"
)

type Statistics struct {
	NumPackets      int
	PacketsReceived int
	RealRuntime     uint32
	RTTMin          float64
	RTTAvg          float64
	RTTMax          float64
	RTTStdDev       float64
}

func timeDiffMillis(start, end time.Time) float64 {
	timeDiff := end.Sub(start)
	milliFrac := float64(timeDiff.Microseconds()) / float64(1000)
	tot := float64(timeDiff.Milliseconds()) + milliFrac
	return tot
}

func calculateStatistics() *Statistics {
	realRuntime := uint32(timeDiffMillis(startedAt, time.Now()))
	var rttMin, rttSum, rttMax float64
	rttMin = math.MaxFloat64
	var numPackets, packetsReceived int
	// Some packets may have been sent just as we were ctrl-c'ing,
	// so we disregard those in the total packet count.
	// Only packets that are received (have a ReceivedAt) or were dropped
	// (have dropped = true) are counted
	for _, packet := range sentPackets {
		if packet.ReceivedAt == nil {
			if packet.Dropped {
				numPackets++
			}
			continue
		}
		numPackets++
		timeDiff := timeDiffMillis(*packet.SentAt, *packet.ReceivedAt)
		rttMin = math.Min(rttMin, timeDiff)
		rttMax = math.Max(rttMax, timeDiff)
		rttSum += timeDiff
		packetsReceived++
	}
	rttAvg := rttSum / float64(numPackets)
	var rttTotalDev float64
	// looping again for standard deviation
	for _, packet := range sentPackets {
		if packet.ReceivedAt == nil {
			continue
		}
		timeDiff := timeDiffMillis(*packet.SentAt, *packet.ReceivedAt)
		rttTotalDev += math.Pow(timeDiff-rttAvg, 2)
	}
	rttStdDev := math.Sqrt(rttTotalDev / float64(numPackets))
	return &Statistics{
		NumPackets:      numPackets,
		PacketsReceived: packetsReceived,
		RealRuntime:     realRuntime,
		RTTMin:          rttMin,
		RTTAvg:          rttAvg,
		RTTMax:          rttMax,
		RTTStdDev:       rttStdDev,
	}
}

func printStatistics() {
	sentPacketsMutex.Lock()
	defer sentPacketsMutex.Unlock()
	stats := calculateStatistics()
	fmt.Printf("--- %s ping statistics ---\n", target)
	fmt.Printf(
		"%d packets transmitted, %d received, %.2f%% packet loss, time %dms\n",
		stats.NumPackets,
		stats.PacketsReceived,
		float64(stats.NumPackets-stats.PacketsReceived)/float64(stats.NumPackets)*100,
		stats.RealRuntime,
	)
	fmt.Printf(
		"rtt min/avg/max/mdev = %.3f/%.3f/%.3f/%.3f ms\n",
		stats.RTTMin,
		stats.RTTAvg,
		stats.RTTMax,
		stats.RTTStdDev,
	)
}
