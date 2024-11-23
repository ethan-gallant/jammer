package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"log"
	"time"
)

const VERSION = "2.0.0"

var (
	debug    bool
	timeout  int
	localIP  string
	remoteIP string

	// Punch configuration flags
	maxRetries    int
	maxPorts      int
	punchTimeout  int
	punchInterval int
	messageSize   int
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	var rootCmd = &cobra.Command{
		Use:   "udpholepunch",
		Short: "UDP hole punching tool",
		Long: `A UDP hole punching tool for establishing peer-to-peer connections.
Example usage:
  First find your public IP:
  $ curl ipinfo.io

  Run as server:
  $ udpholepunch server --local :4500 --remote 1.2.3.4:4500

  Run as client:
  $ udpholepunch client --local :4500 --remote 5.6.7.8:4500

Advanced usage with custom punch parameters:
  $ udpholepunch server --local :4500 --remote 1.2.3.4:4500 \
    --max-retries 200 --max-ports 20 --punch-timeout 3000 \
    --punch-interval 100 --message-size 2048`,
	}

	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "enable debug logging")
	rootCmd.PersistentFlags().IntVarP(&timeout, "timeout", "t", 30, "connection timeout in seconds")

	// Punch configuration flags
	rootCmd.PersistentFlags().IntVar(&maxRetries, "max-retries", 100, "maximum number of punch attempts")
	rootCmd.PersistentFlags().IntVar(&maxPorts, "max-ports", 10, "number of ports to try around target")
	rootCmd.PersistentFlags().IntVar(&punchTimeout, "punch-timeout", 2000, "timeout for each punch attempt in milliseconds")
	rootCmd.PersistentFlags().IntVar(&punchInterval, "punch-interval", 200, "interval between punch attempts in milliseconds")
	rootCmd.PersistentFlags().IntVar(&messageSize, "message-size", 1024, "size of message buffer in bytes")

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
				Punch: PunchConfig{
					MaxRetries:    maxRetries,
					MaxPorts:      maxPorts,
					PunchTimeout:  time.Duration(punchTimeout) * time.Millisecond,
					PunchInterval: time.Duration(punchInterval) * time.Millisecond,
					MessageSize:   messageSize,
				},
			}

			log.Printf("Starting server with configuration:")
			logPunchConfig(conf.Punch)

			if err := runImprovedServer(conf); err != nil {
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
				Punch: PunchConfig{
					MaxRetries:    maxRetries,
					MaxPorts:      maxPorts,
					PunchTimeout:  time.Duration(punchTimeout) * time.Millisecond,
					PunchInterval: time.Duration(punchInterval) * time.Millisecond,
					MessageSize:   messageSize,
				},
			}

			log.Printf("Starting client with configuration:")
			logPunchConfig(conf.Punch)

			if err := runImprovedClient(conf); err != nil {
				log.Fatalf("Client error: %v", err)
			}
		},
	}

	// Add address flags to both commands
	for _, cmd := range []*cobra.Command{serverCmd, clientCmd} {
		cmd.Flags().StringVarP(&localIP, "local", "l", ":4500", "local address (e.g. :4500)")
		cmd.Flags().StringVarP(&remoteIP, "remote", "r", "", "remote address (e.g. 1.2.3.4:4500)")
		cmd.MarkFlagRequired("remote")
	}

	rootCmd.AddCommand(serverCmd, clientCmd)

	fmt.Printf("UDP Hole Punch Tool v%s\n", VERSION)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func logPunchConfig(config PunchConfig) {
	log.Printf("🔧 Max Retries: %d", config.MaxRetries)
	log.Printf("🔧 Max Ports: %d (±%d around target)", config.MaxPorts*2, config.MaxPorts)
	log.Printf("🔧 Punch Timeout: %v", config.PunchTimeout)
	log.Printf("🔧 Punch Interval: %v", config.PunchInterval)
	log.Printf("🔧 Message Size: %d bytes", config.MessageSize)
}
