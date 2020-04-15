package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const Usage = "Usage: ping [-6] {destination}"

const MaxIcmpEchoIpv4 = 28
const MaxIcmpEchoIpv6 = 48

var startTimes = make(map[uint16]time.Time)
var endTimes = make(map[uint16]time.Time)
var target string
var useIpv6 bool

func parseFlags() {
	ipv6 := flag.Bool("6", false, "use ipv6")
	flag.Parse()
	useIpv6 = *ipv6
	if flag.NArg() != 1 {
		log.Fatal(Usage)
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
			Data: nil,
		},
	}
}

func udpAddress(addr string) *net.UDPAddr {
	return &net.UDPAddr{
		IP: net.ParseIP(addr),
	}
}

func handleInterrupt() {
	channel := make(chan os.Signal, 1)
	signal.Notify(channel, syscall.SIGINT)
	<-channel
	fmt.Println("printing statistics...")
	os.Exit(0)
}

func main() {
	go handleInterrupt()
	parseFlags()
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
	for {
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
		startTimes[seq] = time.Now()
		maxReply := MaxIcmpEchoIpv4
		if useIpv6 {
			maxReply = MaxIcmpEchoIpv6
		}
		replyBytes := make([]byte, maxReply)
		nBytes, addr, err := conn.ReadFrom(replyBytes)
		if err != nil {
			log.Fatalf("ReadFrom err %s", err)
		}
		var ttl int
		if useIpv6 {
			ttl, err = conn.IPv6PacketConn().HopLimit()
		} else {
			ttl, err = conn.IPv4PacketConn().TTL()
		}
		// conn.IPv6PacketConn() has no property TTL like ipv4's PacketConn, I'm assuming it's the same?
		reconstructedSeq := binary.BigEndian.Uint16(replyBytes[6:8])
		endTimes[reconstructedSeq] = time.Now()
		timeDiff := endTimes[reconstructedSeq].Sub(startTimes[reconstructedSeq])
		milliFrac := float64(timeDiff.Microseconds()) / float64(1000)
		tot := float64(timeDiff.Milliseconds()) + milliFrac
		fmt.Printf(
			"%d bytes from %s: icmp_seq=%d ttl=%d time=%.3f ms\n",
			nBytes,
			addr,
			reconstructedSeq,
			ttl,
			tot,
		)
		seq++
		time.Sleep(time.Second)
	}
}
