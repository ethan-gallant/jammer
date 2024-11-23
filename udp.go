package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"sync"
	"time"
)

const (
	maxRetries   = 60
	punchTimeout = 5 * time.Second
)

func runServer(conf *Config) {
	conn := setupUDPConnection(conf)
	defer conn.Close()

	log.Printf("🚀 Starting UDP hole punch server...")
	log.Printf("📡 Local address: %s", conn.LocalAddr())
	log.Printf("🎯 Target client: %s", conf.RemoteAddr)

	// Establish hole punch
	if err := performHolePunch(conn, conf); err != nil {
		log.Fatalf("❌ Hole punch failed: %v", err)
	}

	log.Printf("✅ HOLE PUNCH SUCCESSFUL!")
	log.Printf("📞 Connection established with %s", conf.RemoteAddr)
	handleServerConnection(conn, conf)
}

func runClient(conf *Config) {
	conn := setupUDPConnection(conf)
	defer conn.Close()

	log.Printf("🚀 Starting UDP hole punch client...")
	log.Printf("📡 Local address: %s", conn.LocalAddr())
	log.Printf("🎯 Target server: %s", conf.RemoteAddr)

	// Establish hole punch
	if err := performHolePunch(conn, conf); err != nil {
		log.Fatalf("❌ Hole punch failed: %v", err)
	}

	log.Printf("✅ HOLE PUNCH SUCCESSFUL!")
	log.Printf("📞 Connection established with %s", conf.RemoteAddr)
	handleClientConnection(conn, conf)
}

func setupUDPConnection(conf *Config) *net.UDPConn {
	laddr := conf.GetLocalUDPAddr()
	conn, err := net.ListenUDP("udp4", laddr)
	if err != nil {
		log.Fatalf("Failed to create UDP socket: %v", err)
	}
	debugLog("Local UDP socket created on %s", laddr)
	return conn
}

func performHolePunch(conn *net.UDPConn, conf *Config) error {
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	success := make(chan bool, 1)
	raddr := conf.GetRemoteUDPAddr()

	// Start sender goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		msg := fmt.Sprintf("HELLO-%d", os.Getpid())

		for i := 0; i < maxRetries; i++ {
			select {
			case <-ctx.Done():
				return
			default:
				debugLog("📤 Sending punch message (%d/%d): %s", i+1, maxRetries, msg)
				if _, err := conn.WriteToUDP([]byte(msg), raddr); err != nil {
					log.Printf("Send error: %v", err)
				}
				time.Sleep(time.Duration(1000+rand.Intn(1000)) * time.Millisecond)
			}
		}
	}()

	// Start receiver goroutine
	errCh := make(chan error, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 1024)
		readCount := 0

		for {
			select {
			case <-ctx.Done():
				return
			default:
				conn.SetReadDeadline(time.Now().Add(punchTimeout))
				n, raddr, err := conn.ReadFromUDP(buf)
				if err != nil {
					if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
						readCount++
						if readCount%5 == 0 {
							debugLog("😴 Still waiting for response... (%d attempts)", readCount)
						}
						continue
					}
					errCh <- fmt.Errorf("read error: %v", err)
					return
				}

				msg := string(buf[:n])
				debugLog("📥 Received message: %s from %s", msg, raddr)

				if raddr.String() != conf.GetRemoteUDPAddr().String() {
					debugLog("⚠️  Ignoring message from unknown sender: %s", raddr)
					continue
				}

				if n >= 5 && (msg[:5] == "HELLO" || msg[:5] == "READY") {
					// Send confirmation
					response := "READY-" + fmt.Sprint(os.Getpid())
					debugLog("📤 Sending confirmation: %s", response)
					conn.WriteToUDP([]byte(response), raddr)

					// If we get here, hole punch worked!
					success <- true
					cancel() // Stop sender goroutine
					return
				}
			}
		}
	}()

	// Wait for completion or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("punch failed: %v", err)
	case <-success:
		return nil
	case <-time.After(time.Duration(conf.Timeout) * time.Second):
		cancel()
		return fmt.Errorf("hole punch timed out after %d seconds - check if both sides are running and ports are correct", conf.Timeout)
	}
}

func handleServerConnection(conn *net.UDPConn, conf *Config) {
	buf := make([]byte, 4096)
	log.Printf("🎮 Server ready - waiting for data...")

	for {
		conn.SetReadDeadline(time.Now().Add(time.Duration(conf.Timeout) * time.Second))
		n, raddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			if os.IsTimeout(err) {
				debugLog("Read timeout, continuing...")
				continue
			}
			log.Printf("❌ Read error: %v", err)
			return
		}

		data := string(buf[:n])
		debugLog("📥 Received %d bytes from %s: %s", n, raddr, data)

		// Echo back
		_, err = conn.WriteToUDP(buf[:n], raddr)
		if err != nil {
			log.Printf("❌ Write error: %v", err)
			return
		}
		debugLog("📤 Echoed data back to %s", raddr)
	}
}

func handleClientConnection(conn *net.UDPConn, conf *Config) {
	raddr := conf.GetRemoteUDPAddr()
	buf := make([]byte, 4096)

	// Simple ping/pong loop
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	log.Printf("🏓 Starting ping/pong test...")
	for {
		select {
		case <-ticker.C:
			msg := fmt.Sprintf("PING %d", time.Now().UnixNano())
			debugLog("📤 Sending: %s", msg)

			_, err := conn.WriteToUDP([]byte(msg), raddr)
			if err != nil {
				log.Printf("❌ Write error: %v", err)
				return
			}

			conn.SetReadDeadline(time.Now().Add(time.Duration(conf.Timeout) * time.Second))
			n, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				if os.IsTimeout(err) {
					log.Printf("⚠️  Response timeout")
					continue
				}
				log.Printf("❌ Read error: %v", err)
				return
			}

			response := string(buf[:n])
			debugLog("📥 Got response: %s", response)
			if response == msg {
				log.Printf("✅ Ping successful! Round trip complete")
			}
		}
	}
}
