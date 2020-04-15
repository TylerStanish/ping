package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

const Usage = "Usage: ping [-46] {destination}"

// https://stackoverflow.com/a/9449851
const MaxIcmpEchoIpv4 = 28
const MaxIcmpEchoIpv6 = 48

var times = make(map[int]time.Time)

const target = "127.0.0.1"

func parseFlags(ipv4, ipv6 *bool) {
	flag.BoolVar(ipv4, "4", true, "use ipv4")
	flag.BoolVar(ipv6, "6", false, "use ipv6")
	flag.Parse()
	if flag.NArg() != 1 {
		log.Fatal(Usage)
	}
}

func createMessage(seq int, icmpType icmp.Type) *icmp.Message {
	return &icmp.Message{
		Type: icmpType,
		Code: 0,
		Body: &icmp.Echo{
			// we need a unique identifier for this session so the OS can
			// demux the packet back to this process, perhaps the PID will suffice
			ID:   os.Getpid(),
			Seq:  seq,
			Data: nil,
		},
	}
}

func udpAddress(addr string) *net.UDPAddr {
	return &net.UDPAddr{
		IP: net.ParseIP(addr),
	}
}

func main() {
	var useIpv4, useIpv6 bool
	parseFlags(&useIpv4, &useIpv6)
	conn, err := icmp.ListenPacket("udp4", "0.0.0.0")
	if err != nil {
		log.Fatalf("ListenPacket err %s", err)
	}
	defer conn.Close()
	seq := 0
	for {
		msg := createMessage(seq, ipv4.ICMPTypeEcho)
		times[seq] = time.Now()
		//sendMsg(conn, &msg, target)
		bytes, err := msg.Marshal(nil)
		if err != nil {
			log.Fatalf("Marshal err %s", err)
		}
		_, err = conn.WriteTo(bytes, udpAddress(target))
		times[seq] = time.Now()
		if err != nil {
			log.Fatalf("WriteTo err %s", err)
		}
		//sendMsg
		//readReply(conn, seq)
		rb := make([]byte, MaxIcmpEchoIpv4)
		nBytes, addr, err := conn.ReadFrom(rb)
		if err != nil {
			log.Fatalf("ReadFrom err %s", err)
		}
		ttl, err := conn.IPv4PacketConn().TTL()
		//msg := parseReply(rb, ipv4.ICMPTypeEcho.Protocol())
		msg, err = icmp.ParseMessage(ipv4.ICMPTypeEcho.Protocol(), rb)
		if err != nil {
			log.Fatalf("ParseMessage err %s", err)
		}
		//parseReply
		fmt.Printf("%d bytes from %s: icmp_seq=%d ttl=%d time=TODO ms\n", nBytes, addr, seq, ttl)
		//readReply
		seq++
		time.Sleep(time.Second)
	}
}
