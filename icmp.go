package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"time"

	"golang.org/x/net/icmp"
)

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
			fmt.Printf(
				"icmp_seq=%d Destination Host Unreachable\n",
				packet.Seq,
			)
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
