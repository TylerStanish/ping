package main

import (
	"flag"
	"log"
	"os"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

const Usage = "Usage: ping [-46] {destination}"

// https://stackoverflow.com/a/9449851
const MaxIcmpEcho = 65535

//const targetIP = "127.0.0.1"

func parse_flags(ipv4, ipv6 *bool) {
	flag.BoolVar(ipv4, "4", true, "use ipv4")
	flag.BoolVar(ipv6, "6", false, "use ipv6")
	flag.Parse()
	if flag.NArg() != 1 {
		log.Fatal(Usage)
	}
}

func create_message(seq int, icmpType icmp.Type) icmp.Message {
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

func main() {
	var useIpv4, useIpv6 bool
	parse_flags(&useIpv4, &useIpv6)
	conn, err := icmp.ListenPacket("udp4", "0.0.0.0")
	if err != nil {
		log.Fatalf("ListenPacket err %s", err)
	}
	defer conn.Close()
	seq := 0
	for {
		msg := create_message(seq, ipv4.ICMPTypeEcho)
		seq++
	}
}
