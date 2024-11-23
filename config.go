// config.go
package main

import (
	"log"
	"net"
	"time"
)

type PunchConfig struct {
	MaxRetries    int           // Number of punch attempts
	MaxPorts      int           // Number of ports to try around target
	PunchTimeout  time.Duration // Timeout for each punch attempt
	PunchInterval time.Duration // Interval between punch attempts
	MessageSize   int           // Size of message buffer
}

type Config struct {
	Mode       string
	LocalAddr  string
	RemoteAddr string
	Timeout    int
	Debug      bool

	Punch PunchConfig
}

func DefaultPunchConfig() PunchConfig {
	return PunchConfig{
		MaxRetries:    100, // Default 100 retries
		MaxPorts:      10,  // Try ±10 ports around target
		PunchTimeout:  2 * time.Second,
		PunchInterval: 200 * time.Millisecond,
		MessageSize:   1024,
	}
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
