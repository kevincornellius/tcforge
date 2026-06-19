package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Compile tcframe specs and generate test cases into tc/",
	RunE:  runBuild,
}

func runBuild(cmd *cobra.Command, args []string) error {
	fmt.Println("tcforge build — not yet implemented")
	return nil
}
