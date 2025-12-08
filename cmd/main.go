package main

import (
	"fmt"
	"os"

	"github.com/nimling/nimo-api/internal"
	"github.com/spf13/cobra"
)

func main() {
	internal.PrintBanner()

	rootCmd := &cobra.Command{
		Use:     "nimo",
		Short:   "OpenAPI toolkit",
		Version: internal.GetVersion(),
	}

	rootCmd.AddCommand(internal.NewGenerateCommand())
	rootCmd.AddCommand(internal.NewConvertCommand())
	rootCmd.AddCommand(internal.NewMergeCommand())
	rootCmd.AddCommand(internal.NewSyncCommand())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
