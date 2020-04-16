package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

func readConn(conn *icmp.PacketConn) {
	for {
		replyBytes := make([]byte, bodySize+icmpHeaderSize)
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
		sentPacketsMutex.Unlock()
	}
}

// after `timeout` seconds we declare a packet to be dropped
func checkDropped() {
	for _, packet := range sentPackets {
		if (packet.ReceivedAt == nil) && (timeDiffMillis(*packet.SentAt, time.Now()) > timeout) && !packet.Dropped {
			packet.Dropped = true
		}
	}
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
			Data: make([]byte, bodySize),
		},
	}
}

func sendICMP(conn *icmp.PacketConn, seq uint16) {
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
}
