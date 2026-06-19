package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var tunnel bool

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Boot the full contest stack (web + API + judge)",
	RunE:  runServe,
}

func init() {
	serveCmd.Flags().BoolVar(&tunnel, "tunnel", false, "Print cloudflared tunnel instructions after starting")
}

func runServe(cmd *cobra.Command, args []string) error {
	fmt.Println("tcforge serve — not yet implemented")
	return nil
}
