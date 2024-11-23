package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"log"
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

	var rootCmd = &cobra.Command{
		Use:   "jammer",
		Short: "UDP hole punching tool",
		Long: `A simplified UDP hole punching tool for establishing peer-to-peer connections.
Example: Find your public IP first:
  $ curl ipinfo.io

Then run as server:
  $ udpholepunch server --local :4500 --remote 1.2.3.4:4500

Or as client:
  $ udpholepunch client --local :4500 --remote 5.6.7.8:4500`,
	}

	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "enable debug logging")
	rootCmd.PersistentFlags().IntVarP(&timeout, "timeout", "t", 30, "connection timeout in seconds")

	// Server command
	var serverCmd = &cobra.Command{
		Use:   "server",
		Short: "Run in server mode",
		Run: func(cmd *cobra.Command, args []string) {
			log.Printf("Starting server on %s", localIP)
			if debug {
				log.Printf("Debug mode enabled")
			}
			conf := &Config{
				Mode:       "server",
				LocalAddr:  localIP,
				RemoteAddr: remoteIP,
				Timeout:    timeout,
				Debug:      debug,
			}
			runServer(conf)
		},
	}

	// Client command
	var clientCmd = &cobra.Command{
		Use:   "client",
		Short: "Run in client mode",
		Run: func(cmd *cobra.Command, args []string) {
			log.Printf("Starting client connecting to %s", remoteIP)
			if debug {
				log.Printf("Debug mode enabled")
			}
			conf := &Config{
				Mode:       "client",
				LocalAddr:  localIP,
				RemoteAddr: remoteIP,
				Timeout:    timeout,
				Debug:      debug,
			}
			runClient(conf)
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
	fmt.Println("Tip: Use 'curl ipinfo.io' to find your public IP")

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
