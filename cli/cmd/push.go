package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var judgelsTarget string
var judgelsJid string

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Migrate problems to a Judgels instance",
	RunE:  runPush,
}

func init() {
	pushCmd.Flags().StringVar(&judgelsTarget, "judgels", "", "SSH target for Judgels server (e.g. user@judgels-server)")
	pushCmd.Flags().StringVar(&judgelsJid, "jid", "", "Judgels problemJid (optional, resolved from DB if omitted)")
}

func runPush(cmd *cobra.Command, args []string) error {
	fmt.Println("tcforge push — not yet implemented")
	return nil
}
