package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"
)

const (
	// More aggressive constants
	maxRetries    = 100             // Increased from 60
	maxPorts      = 10              // Try multiple ports around target
	punchTimeout  = 2 * time.Second // Reduced timeout
	punchInterval = 200 * time.Millisecond
	messageSize   = 1024
)

type Message struct {
	Type    string // "PING", "PONG", "PUNCH", "READY"
	ID      int    // Random identifier for this node
	Port    int    // Sender's port
	Payload string // Additional data if needed
}

type PunchSession struct {
	LocalAddr  *net.UDPAddr
	RemoteAddr *net.UDPAddr
	Conn       *net.UDPConn
	ID         int
	Mode       string
}

func setupSession(conf *Config) (*PunchSession, error) {
	laddr := conf.GetLocalUDPAddr()
	raddr := conf.GetRemoteUDPAddr()

	// Generate random ID for this session
	id := rand.Intn(10000)

	conn, err := net.ListenUDP("udp4", laddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create UDP socket: %v", err)
	}

	return &PunchSession{
		LocalAddr:  laddr,
		RemoteAddr: raddr,
		Conn:       conn,
		ID:         id,
		Mode:       conf.Mode,
	}, nil
}

func (s *PunchSession) performAggressiveHolePunch(ctx context.Context) error {
	var wg sync.WaitGroup
	success := make(chan bool, 1)
	basePort := s.RemoteAddr.Port

	// Start aggressive sender
	wg.Add(1)
	go func() {
		defer wg.Done()
		msg := Message{
			Type: "PUNCH",
			ID:   s.ID,
			Port: s.LocalAddr.Port,
		}

		for i := 0; i < maxRetries; i++ {
			select {
			case <-ctx.Done():
				return
			default:
				// Try multiple ports around the target port
				for portOffset := 0; portOffset < maxPorts; portOffset++ {
					targetAddr := *s.RemoteAddr
					targetAddr.Port = basePort + portOffset

					data := fmt.Sprintf("%+v", msg)
					s.Conn.WriteToUDP([]byte(data), &targetAddr)

					// Also try symmetric port
					if portOffset > 0 {
						targetAddr.Port = basePort - portOffset
						s.Conn.WriteToUDP([]byte(data), &targetAddr)
					}
				}

				time.Sleep(punchInterval)
			}
		}
	}()

	// Start aggressive receiver
	wg.Add(1)
	go func() {
		defer wg.Done()
		buffer := make([]byte, messageSize)

		for {
			select {
			case <-ctx.Done():
				return
			default:
				s.Conn.SetReadDeadline(time.Now().Add(punchTimeout))
				n, remoteAddr, err := s.Conn.ReadFromUDP(buffer)

				if err != nil {
					if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
						continue
					}
					log.Printf("Read error: %v", err)
					continue
				}

				// Process received message
				var msg Message
				fmt.Sscanf(string(buffer[:n]), "%+v", &msg)

				// Validate message
				if msg.Type == "PUNCH" || msg.Type == "READY" {
					log.Printf("📥 Received %s from %v (ID: %d)", msg.Type, remoteAddr, msg.ID)

					// Send confirmation
					response := Message{
						Type: "READY",
						ID:   s.ID,
						Port: s.LocalAddr.Port,
					}
					data := fmt.Sprintf("%+v", response)
					s.Conn.WriteToUDP([]byte(data), remoteAddr)

					// Update remote address if different
					if remoteAddr.Port != s.RemoteAddr.Port {
						log.Printf("🔄 Updating remote port from %d to %d",
							s.RemoteAddr.Port, remoteAddr.Port)
						s.RemoteAddr = remoteAddr
					}

					success <- true
					return
				}
			}
		}
	}()

	// Wait with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-success:
		log.Printf("✅ Hole punch successful! Connection established with %v", s.RemoteAddr)
		return nil
	case <-time.After(time.Duration(30) * time.Second):
		return fmt.Errorf("hole punch timed out")
	}
}

func (s *PunchSession) startKeepalive() {
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for range ticker.C {
			msg := Message{
				Type: "PING",
				ID:   s.ID,
				Port: s.LocalAddr.Port,
			}
			data := fmt.Sprintf("%+v", msg)
			s.Conn.WriteToUDP([]byte(data), s.RemoteAddr)
		}
	}()
}

func runImprovedServer(conf *Config) error {
	session, err := setupSession(conf)
	if err != nil {
		return err
	}
	defer session.Conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(conf.Timeout)*time.Second)
	defer cancel()

	// Perform hole punching
	if err := session.performAggressiveHolePunch(ctx); err != nil {
		return err
	}

	// Start keepalive
	session.startKeepalive()

	// Handle data transfer
	buffer := make([]byte, messageSize)
	for {
		n, addr, err := session.Conn.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("Read error: %v", err)
			continue
		}

		// Echo data back
		session.Conn.WriteToUDP(buffer[:n], addr)
	}
}

func runImprovedClient(conf *Config) error {
	session, err := setupSession(conf)
	if err != nil {
		return err
	}
	defer session.Conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(conf.Timeout)*time.Second)
	defer cancel()

	// Perform hole punching
	if err := session.performAggressiveHolePunch(ctx); err != nil {
		return err
	}

	// Start keepalive
	session.startKeepalive()

	// Start ping/pong test
	for {
		msg := fmt.Sprintf("PING %d", time.Now().UnixNano())
		if _, err := session.Conn.WriteToUDP([]byte(msg), session.RemoteAddr); err != nil {
			log.Printf("Write error: %v", err)
			continue
		}

		// Wait for response
		buffer := make([]byte, messageSize)
		session.Conn.SetReadDeadline(time.Now().Add(time.Second))
		n, _, err := session.Conn.ReadFromUDP(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				log.Printf("⚠️ Response timeout")
				continue
			}
			log.Printf("Read error: %v", err)
			continue
		}

		response := string(buffer[:n])
		if response == msg {
			log.Printf("✅ Ping successful!")
		}

		time.Sleep(time.Second)
	}
}
