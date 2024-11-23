package main

import (
	"log"
	"net"
)

type Config struct {
	Mode       string
	LocalAddr  string
	RemoteAddr string
	Timeout    int
	Debug      bool
}

func (c *Config) GetLocalUDPAddr() *net.UDPAddr {
	addr, err := net.ResolveUDPAddr("udp4", c.LocalAddr)
	if err != nil {
		log.Fatalf("Invalid local address: %v", err)
	}
	return addr
}

func (c *Config) GetRemoteUDPAddr() *net.UDPAddr {
	addr, err := net.ResolveUDPAddr("udp4", c.RemoteAddr)
	if err != nil {
		log.Fatalf("Invalid remote address: %v", err)
	}
	return addr
}

func debugLog(format string, v ...interface{}) {
	if debug {
		log.Printf("[DEBUG] "+format, v...)
	}
}
