package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"golang.org/x/net/icmp"
)

const Usage = "Usage: ping [-6] [-i interval] [-W timeout] [-s bodysize] {destination}"

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
var timeout float64
var startedAt time.Time
var bodySize = 56
var icmpHeaderSize = 8
var interval int

func parseFlags() {
	ipv6 := flag.Bool("6", false, "use ipv6")
	flag.Float64Var(&timeout, "W", 3000, "timeout")
	flag.IntVar(&bodySize, "s", 56, "bodysize")
	flag.IntVar(&interval, "i", 1, "interval in seconds")
	flag.Parse()
	useIpv6 = *ipv6
	if flag.NArg() != 1 {
		fmt.Println(Usage)
		os.Exit(1)
	}
	target = flag.Args()[0]
}

func handleInterrupt() {
	channel := make(chan os.Signal, 1)
	signal.Notify(channel, syscall.SIGINT)
	<-channel
	println() // ping seems to print a newline upon interrupt to put statistics starting on its own new line
	printStatistics()
	os.Exit(0)
}

func printPingHeader() {
	ipHeaderSize := 20
	if useIpv6 {
		ipHeaderSize = 40
	}
	fmt.Printf("PING %s (%s) %d(%d) bytes of data\n", target, getIP(), bodySize, bodySize+icmpHeaderSize+ipHeaderSize)
}

func main() {
	startedAt = time.Now()
	parseFlags()
	printPingHeader()
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
		sendICMP(conn, seq)
		sentPacketsMutex.Unlock()
		seq++
		time.Sleep(time.Duration(int(time.Second) * interval))
	}
}
