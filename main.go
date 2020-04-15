package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const Usage = "Usage: ping [-6] [-W timeout] [-s bodysize] {destination}"

var bodySize = 56
var icmpIpv4HeaderSize = 28
var icmpIpv6HeaderSize = 48
var maxIcmpEchoIpv4 int
var maxIcmpEchoIpv6 int
var interval int

type PingPacket struct {
	Seq        uint16
	SentAt     *time.Time
	ReceivedAt *time.Time
	Dropped    bool
}

var sentPackets []*PingPacket
var sentPacketsMutex sync.Mutex

var target string
var useIpv6 bool
var timeout float64 = 3000
var startedAt time.Time

func parseFlags() {
	ipv6 := flag.Bool("6", false, "use ipv6")
	flag.Float64Var(&timeout, "W", 3000, "timeout")
	flag.IntVar(&bodySize, "s", 56, "bodysize")
	flag.IntVar(&interval, "i", 1, "interval")
	flag.Parse()
	useIpv6 = *ipv6
	maxIcmpEchoIpv4 = icmpIpv4HeaderSize + bodySize
	maxIcmpEchoIpv6 = icmpIpv6HeaderSize + bodySize
	if flag.NArg() != 1 {
		fmt.Println(Usage)
		os.Exit(1)
	}
	target = flag.Args()[0]
}

func createMessage(seq uint16, icmpType icmp.Type) *icmp.Message {
	return &icmp.Message{
		Type: icmpType,
		Code: 0,
		Body: &icmp.Echo{
			// we need a unique identifier for this session so the OS can
			// demux the packet back to this process, perhaps the PID will suffice
			ID:   os.Getpid(),
			Seq:  int(seq),
			Data: make([]byte, bodySize), // send the typical 56 data bytes
		},
	}
}

func udpAddress(addr string) *net.UDPAddr {
	return &net.UDPAddr{
		IP: net.ParseIP(addr),
	}
}

func printStatistics() {
	realRuntime := uint32(timeDiffMillis(startedAt, time.Now()))
	var rttMin, rttSum, rttMax float64
	rttMin = math.MaxFloat64
	var numPackets, packetsReceived int
	// Some packets may have been sent just as we were ctrl-c'ing,
	// so we disregard those in the total packet count.
	// Only packets that are received (have a ReceivedAt) or were dropped
	// (have dropped = true) are counted
	sentPacketsMutex.Lock()
	for _, packet := range sentPackets {
		if packet.Dropped || packet.ReceivedAt != nil {
			numPackets++
			timeDiff := timeDiffMillis(*packet.SentAt, *packet.ReceivedAt)
			rttMin = math.Min(rttMin, timeDiff)
			rttMax = math.Max(rttMax, timeDiff)
			rttSum += timeDiff
			if packet.ReceivedAt != nil {
				packetsReceived++
			}
		}
	}
	rttAvg := rttSum / float64(numPackets)
	var rttTotalDev float64
	// looping again for standard deviation
	for _, packet := range sentPackets {
		timeDiff := timeDiffMillis(*packet.SentAt, *packet.ReceivedAt)
		rttTotalDev += math.Pow(timeDiff-rttAvg, 2)
	}
	sentPacketsMutex.Unlock()
	rttStdDev := math.Sqrt(rttTotalDev / float64(numPackets))
	fmt.Printf("--- %s ping statistics ---\n", target)
	fmt.Printf(
		"%d packets transmitted, %d received, +%d errors, %.2f%% packet loss, time %dms\n",
		numPackets,
		packetsReceived,
		0,
		float64(numPackets-packetsReceived)/float64(numPackets),
		realRuntime,
	)
	fmt.Printf(
		"rtt min/avg/max/mdev = %.3f/%.3f/%.3f/%.3f ms\n",
		rttMin,
		rttAvg,
		rttMax,
		rttStdDev,
	)
}

func handleInterrupt() {
	channel := make(chan os.Signal, 1)
	signal.Notify(channel, syscall.SIGINT)
	<-channel
	println() // ping seems to print a newline upon interrupt to put statistics starting on its own new line
	printStatistics()
	os.Exit(0)
}

func timeDiffMillis(start, end time.Time) float64 {
	timeDiff := end.Sub(start)
	milliFrac := float64(timeDiff.Microseconds()) / float64(1000)
	tot := float64(timeDiff.Milliseconds()) + milliFrac
	return tot
}

func readConn(conn *icmp.PacketConn) {
	for {
		maxReply := maxIcmpEchoIpv4
		if useIpv6 {
			maxReply = maxIcmpEchoIpv6
		}
		replyBytes := make([]byte, maxReply)
		nBytes, _, err := conn.ReadFrom(replyBytes)
		if err != nil {
			log.Fatalf("ReadFrom err %s", err)
		}
		var ttl int
		if useIpv6 {
			ttl, err = conn.IPv6PacketConn().HopLimit()
		} else {
			ttl, err = conn.IPv4PacketConn().TTL()
		}
		// bytes 6-7 of the raw response (the icmp header) are the seq
		reconstructedSeq := binary.BigEndian.Uint16(replyBytes[6:8])
		sentPacketsMutex.Lock()
		now := time.Now()
		packet := sentPackets[reconstructedSeq]
		sentPacketsMutex.Unlock()
		packet.ReceivedAt = &now
		rtt := timeDiffMillis(*packet.SentAt, *packet.ReceivedAt)
		fmt.Printf(
			"%d bytes from %s: icmp_seq=%d ttl=%d time=%.3f ms\n",
			nBytes,
			target,
			reconstructedSeq,
			ttl,
			rtt,
		)
	}
}

// after x seconds we declare a packet to be dropped
func checkDropped() {
	for _, packet := range sentPackets {
		if (packet.ReceivedAt == nil) && (timeDiffMillis(*packet.SentAt, time.Now()) > timeout) && !packet.Dropped {
			packet.Dropped = true
			fmt.Printf(
				"icmp_seq=%d Destination Host Unreachable\n",
				packet.Seq,
			)
		}
	}
}

func getIP() string {
	addrs, err := net.LookupIP(target)
	if err != nil {
		log.Fatalf("LookupIP err %s", err)
	}
	targetIP := addrs[0].To16().String()
	if useIpv6 && len(addrs) > 1 {
		targetIP = addrs[1].To16().String()
	}
	return targetIP
}

func main() {
	startedAt = time.Now()
	parseFlags()
	var headerSize = icmpIpv4HeaderSize
	if useIpv6 {
		headerSize = icmpIpv6HeaderSize
	}
	fmt.Printf("PING %s (%s) %d(%d) bytes of data\n", target, getIP(), bodySize, bodySize+headerSize)
	go handleInterrupt()
	network := "udp4"
	listenOn := "0.0.0.0"
	if useIpv6 {
		network = "udp6"
		listenOn = "::"
	}
	conn, err := icmp.ListenPacket(network, listenOn)
	if err != nil {
		log.Fatalf("ListenPacket err %s", err)
	}
	defer conn.Close()
	var seq uint16 = 0
	go readConn(conn)
	for {
		sentPacketsMutex.Lock()
		checkDropped()
		icmpType := icmp.Type(ipv4.ICMPTypeEcho)
		if useIpv6 {
			icmpType = icmp.Type(ipv6.ICMPTypeEchoRequest)
		}
		msg := createMessage(seq, icmpType)
		bytes, err := msg.Marshal(nil)
		if err != nil {
			log.Fatalf("Marshal err %s", err)
		}
		_, err = conn.WriteTo(bytes, udpAddress(target))
		if err != nil {
			log.Fatalf("WriteTo err %s", err)
		}
		now := time.Now()
		sentPackets = append(sentPackets, &PingPacket{
			Seq:    seq,
			SentAt: &now,
		})
		sentPacketsMutex.Unlock()
		seq++
		time.Sleep(time.Duration(int(time.Second) * interval))
	}
}
