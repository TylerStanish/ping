package main

import (
	"log"
	"net"
)

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

func udpAddress(addr string) *net.UDPAddr {
	return &net.UDPAddr{
		IP: net.ParseIP(addr),
	}
}
