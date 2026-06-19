package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the contest stack",
	RunE:  runStop,
}

func runStop(cmd *cobra.Command, args []string) error {
	fmt.Println("tcforge stop — not yet implemented")
	return nil
}
