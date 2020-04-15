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
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const Usage = "Usage: ping [-6] [-i interval] [-W timeout] [-s bodysize] {destination}"

var bodySize = 56
var icmpHeaderSize = 8
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
var timeout float64
var startedAt time.Time

func parseFlags() {
	ipv6 := flag.Bool("6", false, "use ipv6")
	flag.Float64Var(&timeout, "W", 3000, "timeout")
	flag.IntVar(&bodySize, "s", 56, "bodysize")
	flag.IntVar(&interval, "i", 1, "interval")
	flag.Parse()
	useIpv6 = *ipv6
	maxIcmpEchoIpv4 = icmpHeaderSize + bodySize
	maxIcmpEchoIpv6 = icmpHeaderSize + bodySize
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

func main() {
	startedAt = time.Now()
	parseFlags()
	fmt.Printf("PING %s (%s) %d(%d) bytes of data\n", target, getIP(), bodySize, bodySize+icmpHeaderSize)
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
