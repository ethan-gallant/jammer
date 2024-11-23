package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

type Client struct {
	ID   int
	Addr *net.UDPAddr
}

type Message struct {
	Type    string   // "REGISTER", "PEERS", "PUNCH", "READY"
	ID      int      // Random identifier for this node
	Address string   // Sender's address
	Peers   []Client // List of known peers (only used in PEERS message)
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

	conn, err := net.ListenUDP("udp4", laddr)
	if err != nil {
		return nil, fmt.Errorf("failed to create UDP socket: %v", err)
	}

	return &PunchSession{
		LocalAddr:  laddr,
		RemoteAddr: raddr,
		Conn:       conn,
		ID:         conf.ID,
		Mode:       conf.Mode,
	}, nil
}

func (s *PunchSession) handleRegistration(clients *sync.Map) error {
	// Send registration to server
	msg := Message{
		Type:    "REGISTER",
		ID:      s.ID,
		Address: s.LocalAddr.String(),
	}

	data := fmt.Sprintf("%+v", msg)
	_, err := s.Conn.WriteToUDP([]byte(data), s.RemoteAddr)
	if err != nil {
		return fmt.Errorf("failed to send registration: %v", err)
	}

	// Wait for peer list from server
	buffer := make([]byte, 4096)
	s.Conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	n, _, err := s.Conn.ReadFromUDP(buffer)
	if err != nil {
		return fmt.Errorf("failed to receive peer list: %v", err)
	}

	var response Message
	fmt.Sscanf(string(buffer[:n]), "%+v", &response)

	if response.Type != "PEERS" {
		return fmt.Errorf("unexpected response type: %s", response.Type)
	}

	// Store received peers
	for _, peer := range response.Peers {
		clients.Store(peer.ID, peer)
	}

	return nil
}

func (s *PunchSession) performHolePunch(ctx context.Context, clients *sync.Map) error {
	var wg sync.WaitGroup
	success := make(chan bool, 1)

	// Start aggressive sender to all peers
	wg.Add(1)
	go func() {
		defer wg.Done()

		msg := Message{
			Type:    "PUNCH",
			ID:      s.ID,
			Address: s.LocalAddr.String(),
		}

		data := fmt.Sprintf("%+v", msg)

		for i := 0; i < 10; i++ { // Send multiple times
			clients.Range(func(key, value interface{}) bool {
				peer := value.(Client)
				s.Conn.WriteToUDP([]byte(data), peer.Addr)
				return true
			})
			time.Sleep(200 * time.Millisecond)
		}
	}()

	// Start receiver
	wg.Add(1)
	go func() {
		defer wg.Done()

		buffer := make([]byte, 4096)

		for {
			select {
			case <-ctx.Done():
				return
			default:
				s.Conn.SetReadDeadline(time.Now().Add(2 * time.Second))
				n, addr, err := s.Conn.ReadFromUDP(buffer)

				if err != nil {
					continue
				}

				var msg Message
				fmt.Sscanf(string(buffer[:n]), "%+v", &msg)

				if msg.Type == "PUNCH" || msg.Type == "READY" {
					log.Printf("Received %s from %v (ID: %d)", msg.Type, addr, msg.ID)

					// Send ready confirmation
					response := Message{
						Type:    "READY",
						ID:      s.ID,
						Address: s.LocalAddr.String(),
					}
					data := fmt.Sprintf("%+v", response)
					s.Conn.WriteToUDP([]byte(data), addr)

					success <- true
					return
				}
			}
		}
	}()

	// Wait with timeout
	select {
	case <-success:
		log.Printf("✅ Hole punch successful!")
		return nil
	case <-time.After(30 * time.Second):
		return fmt.Errorf("hole punch timed out")
	}
}

func runServer(conf *Config) error {
	session, err := setupSession(conf)
	if err != nil {
		return err
	}
	defer session.Conn.Close()

	clients := &sync.Map{}

	// Handle incoming registrations
	for {
		buffer := make([]byte, 4096)
		n, addr, err := session.Conn.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("Read error: %v", err)
			continue
		}

		var msg Message
		fmt.Sscanf(string(buffer[:n]), "%+v", &msg)

		if msg.Type == "REGISTER" {
			// Store new client
			client := Client{
				ID:   msg.ID,
				Addr: addr,
			}
			clients.Store(msg.ID, client)

			// Create peer list
			var peers []Client
			clients.Range(func(key, value interface{}) bool {
				peer := value.(Client)
				peers = append(peers, peer)
				return true
			})

			// Send peer list to everyone
			response := Message{
				Type:  "PEERS",
				Peers: peers,
			}
			data := fmt.Sprintf("%+v", response)

			clients.Range(func(key, value interface{}) bool {
				client := value.(Client)
				session.Conn.WriteToUDP([]byte(data), client.Addr)
				return true
			})

			log.Printf("New client registered: %v (total: %d)", addr, len(peers))
		}
	}
}

func runClient(conf *Config) error {
	session, err := setupSession(conf)
	if err != nil {
		return err
	}
	defer session.Conn.Close()

	clients := &sync.Map{}

	// Register with server and get peer list
	if err := session.handleRegistration(clients); err != nil {
		return fmt.Errorf("registration failed: %v", err)
	}

	// Perform hole punching with all peers
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := session.performHolePunch(ctx, clients); err != nil {
		return err
	}

	// Handle data transfer
	for {
		buffer := make([]byte, 4096)
		n, addr, err := session.Conn.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("Read error: %v", err)
			continue
		}

		// Echo data back
		session.Conn.WriteToUDP(buffer[:n], addr)
	}
}
