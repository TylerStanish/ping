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

//const targetIP = "127.0.0.1"

func parseFlags(ipv4, ipv6 *bool) {
	flag.BoolVar(ipv4, "4", true, "use ipv4")
	flag.BoolVar(ipv6, "6", false, "use ipv6")
	flag.Parse()
	if flag.NArg() != 1 {
		log.Fatal(Usage)
	}
}

func createMessage(seq int, icmpType icmp.Type) icmp.Message {
	return icmp.Message{
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

func sendMsg(conn *icmp.PacketConn, msg *icmp.Message, target string) {
	bytes, err := msg.Marshal(nil)
	if err != nil {
		log.Fatalf("Marshal err %s", err)
	}
	_, err = conn.WriteTo(bytes, udpAddress(target))
	if err != nil {
		log.Fatalf("WriteTo err %s", err)
	}
}

func parseReply(data []byte, proto int) *icmp.Message {
	msg, err := icmp.ParseMessage(proto, data)
	if err != nil {
		log.Fatalf("ParseMessage err %s", err)
	}
	return msg
}

func readReply(conn *icmp.PacketConn, seq int) {
	rb := make([]byte, MaxIcmpEchoIpv4)
	nBytes, addr, err := conn.ReadFrom(rb)
	if err != nil {
		log.Fatalf("ReadFrom err %s", err)
	}
	ttl, err := conn.IPv4PacketConn().TTL()
	parseReply(rb, ipv4.ICMPTypeEcho.Protocol())
	fmt.Printf("%d bytes from %s: icmp_seq=%d ttl=%d time=TODO ms\n", nBytes, addr, seq, ttl)
}

func main() {
	var useIpv4, useIpv6 bool
	target := "8.8.8.8"
	parseFlags(&useIpv4, &useIpv6)
	conn, err := icmp.ListenPacket("udp4", "0.0.0.0")
	if err != nil {
		log.Fatalf("ListenPacket err %s", err)
	}
	defer conn.Close()
	seq := 0
	for {
		msg := createMessage(seq, ipv4.ICMPTypeEcho)
		sendMsg(conn, &msg, target)
		readReply(conn, seq)
		seq++
		time.Sleep(time.Second)
	}
}
