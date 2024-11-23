package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/spf13/cobra"
)

const VERSION = "2.0.0"

var (
	debug    bool
	timeout  int
	localIP  string
	remoteIP string
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	rand.Seed(time.Now().UnixNano())

	var rootCmd = &cobra.Command{
		Use:   "udpholepunch",
		Short: "UDP hole punching tool",
		Long: `A UDP hole punching tool for establishing peer-to-peer connections.
Example usage:
  First run the server:
  $ udpholepunch server --local :4500

  Then run clients with server's public IP:
  $ udpholepunch client --local :4500 --remote server-ip:4500

Each client will:
1. Register with the server
2. Get a list of other peers
3. Attempt hole punching with each peer
4. Establish direct P2P connections`,
	}

	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "enable debug logging")
	rootCmd.PersistentFlags().IntVarP(&timeout, "timeout", "t", 30, "connection timeout in seconds")

	// Server command
	var serverCmd = &cobra.Command{
		Use:   "server",
		Short: "Run in server mode",
		Run: func(cmd *cobra.Command, args []string) {
			conf := &Config{
				Mode:       "server",
				LocalAddr:  localIP,
				RemoteAddr: remoteIP,
				Timeout:    timeout,
				Debug:      debug,
				ID:         rand.Intn(10000), // Random ID for this instance
			}

			log.Printf("Starting UDP hole punch server on %s", localIP)
			if err := runServer(conf); err != nil {
				log.Fatalf("Server error: %v", err)
			}
		},
	}

	// Client command
	var clientCmd = &cobra.Command{
		Use:   "client",
		Short: "Run in client mode",
		Run: func(cmd *cobra.Command, args []string) {
			conf := &Config{
				Mode:       "client",
				LocalAddr:  localIP,
				RemoteAddr: remoteIP,
				Timeout:    timeout,
				Debug:      debug,
				ID:         rand.Intn(10000), // Random ID for this instance
			}

			log.Printf("Starting UDP hole punch client")
			log.Printf("Local address: %s", localIP)
			log.Printf("Remote server: %s", remoteIP)

			if err := runClient(conf); err != nil {
				log.Fatalf("Client error: %v", err)
			}
		},
	}

	// Add address flags to both commands
	for _, cmd := range []*cobra.Command{serverCmd, clientCmd} {
		cmd.Flags().StringVarP(&localIP, "local", "l", ":4500", "local address (e.g. :4500)")
		cmd.Flags().StringVarP(&remoteIP, "remote", "r", "", "remote address (e.g. 1.2.3.4:4500)")

		// Server doesn't require remote address
		if cmd != serverCmd {
			cmd.MarkFlagRequired("remote")
		}
	}

	rootCmd.AddCommand(serverCmd, clientCmd)

	fmt.Printf("UDP Hole Punch Tool v%s\n", VERSION)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
